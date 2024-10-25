// Copyright (c) 2013-2018 The btcsuite developers
// Copyright (c) 2016-2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package localpeer

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/btcsuite/go-socks/socks"
	"github.com/davecgh/go-spew/spew"
	"github.com/decred/dcrd/lru"
	"github.com/sat20-labs/satsnet_btcd/chaincfg"
	"github.com/sat20-labs/satsnet_btcd/chaincfg/chainhash"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validatorcommand"
	"github.com/sat20-labs/satsnet_btcd/wire"
)

const (
	// MaxValidatorVersion is the max validator version the peer supports.
	MaxValidatorVersion = validatorcommand.VALIDATOR_VERION

	// DefaultTrickleInterval is the min time between attempts to send an
	// inv message to a peer.
	DefaultTrickleInterval = 10 * time.Second

	// outputBufferSize is the number of elements the output channels use.
	outputBufferSize = 50

	// pingInterval is the interval of time to wait in between sending ping
	// messages.
	pingInterval = 10 * time.Second //2 * time.Minute

	// negotiateTimeout is the duration of inactivity before we timeout a
	// peer that hasn't completed the initial version negotiation.
	negotiateTimeout = 30 * time.Second

	// idleTimeout is the duration of inactivity before we time out a peer.
	idleTimeout = 5 * time.Minute

	// stallTickInterval is the interval of time between each check for
	// stalled peers.
	stallTickInterval = 15 * time.Second

	// stallResponseTimeout is the base maximum amount of time messages that
	// expect a response will wait before disconnecting the peer for
	// stalling.  The deadlines are adjusted for callback running times and
	// only checked on each stall tick interval.
	stallResponseTimeout = 30 * time.Second
)

var (
	// nodeCount is the total number of peer connections made since startup
	// and is used to assign an id to a peer.
	nodeCount int32

	// zeroHash is the zero value hash (all zeros).  It is defined as a
	// convenience.
	zeroHash chainhash.Hash

	// sentNonces houses the unique nonces that are generated when pushing
	// version messages that are used to detect self connections.
	sentNonces = lru.NewCache(50)
)

// LocalPeerInterface defines callback function pointers to invoke by local
// peers. Include notify from peer to manager, and get info from manager.

// All notify functions should be start with "On". And all get functions should
// be start with "Get".
type LocalPeerInterface interface {
	// OnPeerConnected is invoked when a remote peer connects to the local peer .
	OnPeerConnected(net.Addr)

	// GetAllValidators invoke when get all validators.
	GetAllValidators() []byte
}

// Config is the struct to hold configuration options useful to localpeer.
type LocalPeerConfig struct {
	// HostToNetAddress returns the netaddress for the given host. This can be
	// nil in  which case the host will be parsed as an IP address.
	HostToNetAddress HostToNetAddrFunc

	// Proxy indicates a proxy is being used for connections.  The only
	// effect this has is to prevent leaking the tor proxy address, so it
	// only needs to specified if using a tor proxy.
	Proxy string

	// UserAgentName specifies the user agent name to advertise.  It is
	// highly recommended to specify this value.
	UserAgentName string

	// ChainParams identifies which chain parameters the peer is associated
	// with.  It is highly recommended to specify this field, however it can
	// be omitted in which case the test network will be used.
	ChainParams *chaincfg.Params

	// ValidatorVersion specifies the maximum validator version to use and
	// advertise.  This field can be omitted in which case
	// peer.MaxProtocolVersion will be used.
	ValidatorVersion uint32

	// LocalPeerInterface to be used by peer manager.
	LocalValidator LocalPeerInterface

	// TrickleInterval is the duration of the ticker which trickles down the
	// inventory to a peer.
	TrickleInterval time.Duration

	// AllowSelfConns is only used to allow the tests to bypass the self
	// connection detecting and disconnect logic since they intentionally
	// do so for testing purposes.
	AllowSelfConns bool

	// Dial connects to the address on the named network. It cannot be nil.
	Dial   func(net.Addr) (net.Conn, error)
	Lookup func(string) ([]net.IP, error)
}

// minUint32 is a helper function to return the minimum of two uint32s.
// This avoids a math import and the need to cast to floats.
func minUint32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

// newNetAddress attempts to extract the IP address and port from the passed
// net.Addr interface and create a bitcoin NetAddress structure using that
// information.
func newNetAddress(addr net.Addr, services wire.ServiceFlag) (*wire.NetAddress, error) {
	// addr will be a net.TCPAddr when not using a proxy.
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		ip := tcpAddr.IP
		port := uint16(tcpAddr.Port)
		na := wire.NewNetAddressIPPort(ip, port, services)
		return na, nil
	}

	// addr will be a socks.ProxiedAddr when using a proxy.
	if proxiedAddr, ok := addr.(*socks.ProxiedAddr); ok {
		ip := net.ParseIP(proxiedAddr.Host)
		if ip == nil {
			ip = net.ParseIP("0.0.0.0")
		}
		port := uint16(proxiedAddr.Port)
		na := wire.NewNetAddressIPPort(ip, port, services)
		return na, nil
	}

	// For the most part, addr should be one of the two above cases, but
	// to be safe, fall back to trying to parse the information from the
	// address string as a last resort.
	host, portStr, err := net.SplitHostPort(addr.String())
	if err != nil {
		return nil, err
	}
	ip := net.ParseIP(host)
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, err
	}
	na := wire.NewNetAddressIPPort(ip, uint16(port), services)
	return na, nil
}

// outMsg is used to house a message to be sent along with a channel to signal
// when the message has been sent (or won't be sent due to things such as
// shutdown)
type outMsg struct {
	msg      validatorcommand.Message
	doneChan chan<- struct{}
}

// stallControlCmd represents the command of a stall control message.
type stallControlCmd uint8

// Constants for the command of a stall control message.
const (
	// sccSendMessage indicates a message is being sent to the remote peer.
	sccSendMessage stallControlCmd = iota

	// sccReceiveMessage indicates a message has been received from the
	// remote peer.
	sccReceiveMessage

	// sccHandlerStart indicates a callback handler is about to be invoked.
	sccHandlerStart

	// sccHandlerStart indicates a callback handler has completed.
	sccHandlerDone
)

// stallControlMsg is used to signal the stall handler about specific events
// so it can properly detect and handle stalled remote peers.
type stallControlMsg struct {
	command stallControlCmd
	message wire.Message
}

// StatsSnap is a snapshot of peer stats at a point in time.
type StatsSnap struct {
	ID             int32
	Addr           string
	Services       wire.ServiceFlag
	LastSend       time.Time
	LastRecv       time.Time
	BytesSent      uint64
	BytesRecv      uint64
	ConnTime       time.Time
	TimeOffset     int64
	Version        uint32
	UserAgent      string
	Inbound        bool
	StartingHeight int32
	LastBlock      int32
	LastPingNonce  uint64
	LastPingTime   time.Time
	LastPingMicros int64
}

// HashFunc is a function which returns a block hash, height and error
// It is used as a callback to get newest block details.
type HashFunc func() (hash *chainhash.Hash, height int32, err error)

// AddrFunc is a func which takes an address and returns a related address.
type AddrFunc func(remoteAddr *wire.NetAddress) *wire.NetAddress

// HostToNetAddrFunc is a func which takes a host, port, services and returns
// the netaddress.
type HostToNetAddrFunc func(host string, port uint16,
	services wire.ServiceFlag) (*wire.NetAddressV2, error)

// ConnState represents the state of the requested connection.
type ConnState uint8

// ConnState can be either pending, established, disconnected or failed.  When
// a new connection is requested, it is attempted and categorized as
// established or failed depending on the connection result.  An established
// connection which was disconnected is categorized as disconnected.
const (
	ConnPending ConnState = iota
	ConnFailing
	ConnCanceled
	ConnEstablished
	ConnDisconnected
)

// ConnReq is the connection request to a network address. The
// connection will be retried on disconnection.
type ConnReq struct {
	// The following variables must only be used atomically.
	id uint64

	LocalAddr  net.Addr
	RemoteAddr net.Addr

	lastActive int64 // last time it was used
	connClose  int32 // set to 1, the connection has been closed

	conn net.Conn

	CmdsLock    sync.RWMutex
	pendingCmds *list.List // Output commands
}

// NOTE: The overall data flow of a peer is split into 3 goroutines.  Inbound
// messages are read via the inHandler goroutine and generally dispatched to
// their own handler.  For inbound data-related messages such as blocks,
// transactions, and inventory, the data is handled by the corresponding
// message handlers.  The data flow for outbound messages is split into 2
// goroutines, queueHandler and outHandler.  The first, queueHandler, is used
// as a way for external entities to queue messages, by way of the QueueMessage
// function, quickly regardless of whether the peer is currently sending or not.
// It acts as the traffic cop between the external world and the actual
// goroutine which writes to the network socket.

// localpeer provides a basic concurrent safe bitcoin peer for handling bitcoin
// communications via the peer-to-peer protocol.  It provides full duplex
// reading and writing, automatic handling of the initial handshake process,
// querying of usage statistics and other information about the remote peer such
// as its address, user agent, and protocol version, output message queuing,
// inventory trickling, and the ability to dynamically register and unregister
// callbacks for handling bitcoin protocol messages.
//
// Outbound messages are typically queued via QueueMessage or QueueInventory.
// QueueMessage is intended for all messages, including responses to data such
// as blocks and transactions.  QueueInventory, on the other hand, is only
// intended for relaying inventory as it employs a trickling mechanism to batch
// the inventory together.  However, some helper functions for pushing messages
// of specific types that typically require common special handling are
// provided as a convenience.
type LocalPeer struct {
	// The following variables must only be used atomically.
	bytesReceived uint64
	bytesSent     uint64
	lastRecv      int64
	lastSend      int64
	connected     int32
	disconnect    int32

	conn net.Conn
	stop int32
	wg   sync.WaitGroup

	// These fields are set at creation time and never modified, so they are
	// safe to read from concurrently without a mutex.
	addrsList []net.Addr // All the addresses reported by the peer
	addr      string     // default addr for connected to peer
	cfg       LocalPeerConfig
	inbound   bool

	flagsMtx             sync.Mutex // protects the peer flags below
	na                   *wire.NetAddressV2
	id                   int32
	userAgent            string
	services             wire.ServiceFlag
	versionKnown         bool
	advertisedProtoVer   uint32 // protocol version advertised by remote
	validatorVersion     uint32 // negotiated validator version
	sendHeadersPreferred bool   // peer sent a sendheaders message
	verAckReceived       bool
	witnessEnabled       bool
	sendAddrV2           bool

	wireEncoding wire.MessageEncoding

	prevGetBlocksMtx   sync.Mutex
	prevGetBlocksBegin *chainhash.Hash
	prevGetBlocksStop  *chainhash.Hash
	prevGetHdrsMtx     sync.Mutex
	prevGetHdrsBegin   *chainhash.Hash
	prevGetHdrsStop    *chainhash.Hash

	// These fields keep track of statistics for the peer and are protected
	// by the statsMtx mutex.
	statsMtx           sync.RWMutex
	timeOffset         int64
	timeConnected      time.Time
	startingHeight     int32
	lastBlock          int32
	lastAnnouncedBlock *chainhash.Hash
	lastPingNonce      uint64    // Set to nonce if we have a pending ping.
	lastPingTime       time.Time // Time we sent last ping.
	lastPingMicros     int64     // Time for last ping to return.

	stallControl  chan stallControlMsg
	outputQueue   chan outMsg
	sendQueue     chan outMsg
	sendDoneQueue chan struct{}
	outputInvChan chan *wire.InvVect
	inQuit        chan struct{}
	queueQuit     chan struct{}
	outQuit       chan struct{}
	quit          chan struct{}

	connReqIndex uint64              // Index of the current connection request, From 1
	connMap      map[uint64]*ConnReq // map of all connections, key is conn id, value is conn req
}

// String returns the peer's address and directionality as a human-readable
// string.
//
// This function is safe for concurrent access.
func (p *LocalPeer) String() string {
	return fmt.Sprintf("%s (%s)", p.addr, directionString(p.inbound))
}

// UpdateLastBlockHeight updates the last known block for the peer.
//
// This function is safe for concurrent access.
func (p *LocalPeer) UpdateLastBlockHeight(newHeight int32) {
	p.statsMtx.Lock()
	if newHeight <= p.lastBlock {
		p.statsMtx.Unlock()
		return
	}
	log.Tracef("Updating last block height of peer %v from %v to %v",
		p.addr, p.lastBlock, newHeight)
	p.lastBlock = newHeight
	p.statsMtx.Unlock()
}

// UpdateLastAnnouncedBlock updates meta-data about the last block hash this
// peer is known to have announced.
//
// This function is safe for concurrent access.
func (p *LocalPeer) UpdateLastAnnouncedBlock(blkHash *chainhash.Hash) {
	log.Tracef("Updating last blk for peer %v, %v", p.addr, blkHash)

	p.statsMtx.Lock()
	p.lastAnnouncedBlock = blkHash
	p.statsMtx.Unlock()
}

// StatsSnapshot returns a snapshot of the current peer flags and statistics.
//
// This function is safe for concurrent access.
func (p *LocalPeer) StatsSnapshot() *StatsSnap {
	p.statsMtx.RLock()

	p.flagsMtx.Lock()
	id := p.id
	addr := p.addr
	userAgent := p.userAgent
	services := p.services
	validatorVersion := p.advertisedProtoVer
	p.flagsMtx.Unlock()

	// Get a copy of all relevant flags and stats.
	statsSnap := &StatsSnap{
		ID:             id,
		Addr:           addr,
		UserAgent:      userAgent,
		Services:       services,
		LastSend:       p.LastSend(),
		LastRecv:       p.LastRecv(),
		BytesSent:      p.BytesSent(),
		BytesRecv:      p.BytesReceived(),
		ConnTime:       p.timeConnected,
		TimeOffset:     p.timeOffset,
		Version:        validatorVersion,
		Inbound:        p.inbound,
		StartingHeight: p.startingHeight,
		LastBlock:      p.lastBlock,
		LastPingNonce:  p.lastPingNonce,
		LastPingMicros: p.lastPingMicros,
		LastPingTime:   p.lastPingTime,
	}

	p.statsMtx.RUnlock()
	return statsSnap
}

// ID returns the peer id.
//
// This function is safe for concurrent access.
func (p *LocalPeer) ID() int32 {
	p.flagsMtx.Lock()
	id := p.id
	p.flagsMtx.Unlock()

	return id
}

// NA returns the peer network address.
//
// This function is safe for concurrent access.
func (p *LocalPeer) NA() *wire.NetAddressV2 {
	p.flagsMtx.Lock()
	na := p.na
	p.flagsMtx.Unlock()

	return na
}

// Addr returns the peer address.
//
// This function is safe for concurrent access.
func (p *LocalPeer) Addr() string {
	// The address doesn't change after initialization, therefore it is not
	// protected by a mutex.
	return p.addr
}

// Addr returns the peer address.
//
// This function is safe for concurrent access.
func (p *LocalPeer) GetPeerAddrsList() []net.Addr {
	// The address doesn't change after initialization, therefore it is not
	// protected by a mutex.
	return p.addrsList
}

// Inbound returns whether the peer is inbound.
//
// This function is safe for concurrent access.
func (p *LocalPeer) Inbound() bool {
	return p.inbound
}

// Services returns the services flag of the remote peer.
//
// This function is safe for concurrent access.
func (p *LocalPeer) Services() wire.ServiceFlag {
	p.flagsMtx.Lock()
	services := p.services
	p.flagsMtx.Unlock()

	return services
}

// UserAgent returns the user agent of the remote peer.
//
// This function is safe for concurrent access.
func (p *LocalPeer) UserAgent() string {
	p.flagsMtx.Lock()
	userAgent := p.userAgent
	p.flagsMtx.Unlock()

	return userAgent
}

// LastAnnouncedBlock returns the last announced block of the remote peer.
//
// This function is safe for concurrent access.
func (p *LocalPeer) LastAnnouncedBlock() *chainhash.Hash {
	p.statsMtx.RLock()
	lastAnnouncedBlock := p.lastAnnouncedBlock
	p.statsMtx.RUnlock()

	return lastAnnouncedBlock
}

// LastPingNonce returns the last ping nonce of the remote peer.
//
// This function is safe for concurrent access.
func (p *LocalPeer) LastPingNonce() uint64 {
	p.statsMtx.RLock()
	lastPingNonce := p.lastPingNonce
	p.statsMtx.RUnlock()

	return lastPingNonce
}

// LastPingTime returns the last ping time of the remote peer.
//
// This function is safe for concurrent access.
func (p *LocalPeer) LastPingTime() time.Time {
	p.statsMtx.RLock()
	lastPingTime := p.lastPingTime
	p.statsMtx.RUnlock()

	return lastPingTime
}

// LastPingMicros returns the last ping micros of the remote peer.
//
// This function is safe for concurrent access.
func (p *LocalPeer) LastPingMicros() int64 {
	p.statsMtx.RLock()
	lastPingMicros := p.lastPingMicros
	p.statsMtx.RUnlock()

	return lastPingMicros
}

// VersionKnown returns the whether or not the version of a peer is known
// locally.
//
// This function is safe for concurrent access.
func (p *LocalPeer) VersionKnown() bool {
	p.flagsMtx.Lock()
	versionKnown := p.versionKnown
	p.flagsMtx.Unlock()

	return versionKnown
}

// VerAckReceived returns whether or not a verack message was received by the
// peer.
//
// This function is safe for concurrent access.
func (p *LocalPeer) VerAckReceived() bool {
	p.flagsMtx.Lock()
	verAckReceived := p.verAckReceived
	p.flagsMtx.Unlock()

	return verAckReceived
}

// ProtocolVersion returns the negotiated peer protocol version.
//
// This function is safe for concurrent access.
func (p *LocalPeer) ValidatorVersion() uint32 {
	p.flagsMtx.Lock()
	validatorVersion := p.validatorVersion
	p.flagsMtx.Unlock()

	return validatorVersion
}

// LastBlock returns the last block of the peer.
//
// This function is safe for concurrent access.
func (p *LocalPeer) LastBlock() int32 {
	p.statsMtx.RLock()
	lastBlock := p.lastBlock
	p.statsMtx.RUnlock()

	return lastBlock
}

// LastSend returns the last send time of the peer.
//
// This function is safe for concurrent access.
func (p *LocalPeer) LastSend() time.Time {
	return time.Unix(atomic.LoadInt64(&p.lastSend), 0)
}

// LastRecv returns the last recv time of the peer.
//
// This function is safe for concurrent access.
func (p *LocalPeer) LastRecv() time.Time {
	return time.Unix(atomic.LoadInt64(&p.lastRecv), 0)
}

// LocalAddr returns the local address of the connection.
//
// This function is safe for concurrent access.
func (p *LocalPeer) LocalAddr() net.Addr {
	var localAddr net.Addr
	if atomic.LoadInt32(&p.connected) != 0 {
		localAddr = p.conn.LocalAddr()
	}
	return localAddr
}

// BytesSent returns the total number of bytes sent by the peer.
//
// This function is safe for concurrent access.
func (p *LocalPeer) BytesSent() uint64 {
	return atomic.LoadUint64(&p.bytesSent)
}

// BytesReceived returns the total number of bytes received by the peer.
//
// This function is safe for concurrent access.
func (p *LocalPeer) BytesReceived() uint64 {
	return atomic.LoadUint64(&p.bytesReceived)
}

// TimeConnected returns the time at which the peer connected.
//
// This function is safe for concurrent access.
func (p *LocalPeer) TimeConnected() time.Time {
	p.statsMtx.RLock()
	timeConnected := p.timeConnected
	p.statsMtx.RUnlock()

	return timeConnected
}

// TimeOffset returns the number of seconds the local time was offset from the
// time the peer reported during the initial negotiation phase.  Negative values
// indicate the remote peer's time is before the local time.
//
// This function is safe for concurrent access.
func (p *LocalPeer) TimeOffset() int64 {
	p.statsMtx.RLock()
	timeOffset := p.timeOffset
	p.statsMtx.RUnlock()

	return timeOffset
}

// StartingHeight returns the last known height the peer reported during the
// initial negotiation phase.
//
// This function is safe for concurrent access.
func (p *LocalPeer) StartingHeight() int32 {
	p.statsMtx.RLock()
	startingHeight := p.startingHeight
	p.statsMtx.RUnlock()

	return startingHeight
}

// WantsHeaders returns if the peer wants header messages instead of
// inventory vectors for blocks.
//
// This function is safe for concurrent access.
func (p *LocalPeer) WantsHeaders() bool {
	p.flagsMtx.Lock()
	sendHeadersPreferred := p.sendHeadersPreferred
	p.flagsMtx.Unlock()

	return sendHeadersPreferred
}

// IsWitnessEnabled returns true if the peer has signalled that it supports
// segregated witness.
//
// This function is safe for concurrent access.
func (p *LocalPeer) IsWitnessEnabled() bool {
	p.flagsMtx.Lock()
	witnessEnabled := p.witnessEnabled
	p.flagsMtx.Unlock()

	return witnessEnabled
}

// WantsAddrV2 returns if the peer supports addrv2 messages instead of the
// legacy addr messages.
func (p *LocalPeer) WantsAddrV2() bool {
	p.flagsMtx.Lock()
	wantsAddrV2 := p.sendAddrV2
	p.flagsMtx.Unlock()

	return wantsAddrV2
}

// PushRejectMsg sends a reject message for the provided command, reject code,
// reject reason, and hash.  The hash will only be used when the command is a tx
// or block and should be nil in other cases.  The wait parameter will cause the
// function to block until the reject message has actually been sent.
//
// This function is safe for concurrent access.
func (p *LocalPeer) PushRejectMsg(command string, code validatorcommand.RejectCode, reason string, wait bool) {
	// Don't bother sending the reject message if the protocol version
	// is too low.
	if p.VersionKnown() && p.ValidatorVersion() < wire.RejectVersion {
		return
	}

	msg := validatorcommand.NewMsgReject(command, code, reason)

	// Send the message without waiting if the caller has not requested it.
	if !wait {
		p.QueueMessage(msg, nil)
		return
	}

	// Send the message and block until it has been sent before returning.
	doneChan := make(chan struct{}, 1)
	p.QueueMessage(msg, doneChan)
	<-doneChan
}

// handlePingMsg is invoked when a peer receives a ping bitcoin message.  For
// recent clients (protocol version > BIP0031Version), it replies with a pong
// message.  For older clients, it does nothing and anything other than failure
// is considered a successful ping.
func (p *LocalPeer) handlePingMsg(msg *validatorcommand.MsgPing) {
	// Only reply with pong if the message is from a new enough client.
	p.QueueMessage(validatorcommand.NewMsgPong(msg.Nonce), nil)
}

// handlePongMsg is invoked when a peer receives a pong bitcoin message.  It
// updates the ping statistics as required for recent clients (protocol
// version > BIP0031Version).  There is no effect for older clients or when a
// ping was not previously sent.
func (p *LocalPeer) handlePongMsg(msg *validatorcommand.MsgPong) {
	// Arguably we could use a buffered channel here sending data
	// in a fifo manner whenever we send a ping, or a list keeping track of
	// the times of each ping. For now we just make a best effort and
	// only record stats if it was for the last ping sent. Any preceding
	// and overlapping pings will be ignored. It is unlikely to occur
	// without large usage of the ping rpc call since we ping infrequently
	// enough that if they overlap we would have timed out the peer.
	p.statsMtx.Lock()
	if p.lastPingNonce != 0 && msg.Nonce == p.lastPingNonce {
		p.lastPingMicros = time.Since(p.lastPingTime).Nanoseconds()
		p.lastPingMicros /= 1000 // convert to usec.
		p.lastPingNonce = 0
	}
	p.statsMtx.Unlock()
}

// readMessage reads the next bitcoin message from the peer with logging.
func (p *LocalPeer) readMessage() (validatorcommand.Message, []byte, error) {
	n, msg, buf, err := validatorcommand.ReadMessage(p.conn,
		p.ValidatorVersion(), p.cfg.ChainParams.Net)
	atomic.AddUint64(&p.bytesReceived, uint64(n))
	// if p.cfg.Listeners.OnRead != nil {
	// 	p.cfg.Listeners.OnRead(p, n, msg, err)
	// }
	if err != nil {
		return nil, nil, err
	}

	// Use closures to log expensive operations so they are only run when
	// the logging level requires it.
	log.Debugf("%v", newLogClosure(func() string {
		// Debug summary of message.
		summary := messageSummary(msg)
		if len(summary) > 0 {
			summary = " (" + summary + ")"
		}
		return fmt.Sprintf("Received %v%s from %s",
			msg.Command(), summary, p)
	}))
	log.Tracef("%v", newLogClosure(func() string {
		return spew.Sdump(msg)
	}))
	log.Tracef("%v", newLogClosure(func() string {
		return spew.Sdump(buf)
	}))

	return msg, buf, nil
}

// writeMessage sends a bitcoin message to the peer with logging.
func (p *LocalPeer) writeMessage(msg validatorcommand.Message) error {
	// Don't do anything if we're disconnecting.
	if atomic.LoadInt32(&p.disconnect) != 0 {
		return nil
	}

	// Use closures to log expensive operations so they are only run when
	// the logging level requires it.
	log.Debugf("%v", newLogClosure(func() string {
		// Debug summary of message.
		summary := messageSummary(msg)
		if len(summary) > 0 {
			summary = " (" + summary + ")"
		}
		return fmt.Sprintf("Sending %v%s to %s", msg.Command(),
			summary, p)
	}))
	log.Tracef("%v", newLogClosure(func() string {
		return spew.Sdump(msg)
	}))
	log.Tracef("%v", newLogClosure(func() string {
		var buf bytes.Buffer
		n, err := validatorcommand.WriteMessageWithEncodingN(&buf, msg, p.ValidatorVersion(),
			p.cfg.ChainParams.Net)
		if err != nil {
			return err.Error()
		}
		log.Debugf("**********newLogClosure  WriteMessage (%s): %v bytes written", msg.Command(), n)
		return spew.Sdump(buf.Bytes())
	}))

	// Write the message to the peer.
	n, err := validatorcommand.WriteMessageWithEncodingN(p.conn, msg,
		p.ValidatorVersion(), p.cfg.ChainParams.Net)
	atomic.AddUint64(&p.bytesSent, uint64(n))
	// if p.cfg.Listeners.OnWrite != nil {
	// 	p.cfg.Listeners.OnWrite(p, n, msg, err)
	// }

	log.Debugf("**********WriteMessage (%s): %d bytes written", msg.Command(), n)

	return err
}

// isAllowedReadError returns whether or not the passed error is allowed without
// disconnecting the peer.  In particular, regression tests need to be allowed
// to send malformed messages without the peer being disconnected.
func (p *LocalPeer) isAllowedReadError(err error) bool {
	// Only allow read errors in regression test mode.
	if p.cfg.ChainParams.Net != wire.TestNet {
		return false
	}

	// Don't allow the error if it's not specifically a malformed message error.
	if _, ok := err.(*wire.MessageError); !ok {
		return false
	}

	// Don't allow the error if it's not coming from localhost or the
	// hostname can't be determined for some reason.
	host, _, err := net.SplitHostPort(p.addr)
	if err != nil {
		return false
	}

	if host != "127.0.0.1" && host != "localhost" {
		return false
	}

	// Allowed if all checks passed.
	return true
}

// shouldHandleReadError returns whether or not the passed error, which is
// expected to have come from reading from the remote peer in the inHandler,
// should be logged and responded to with a reject message.
func (p *LocalPeer) shouldHandleReadError(err error) bool {
	// No logging or reject message when the peer is being forcibly
	// disconnected.
	if atomic.LoadInt32(&p.disconnect) != 0 {
		return false
	}

	// No logging or reject message when the remote peer has been
	// disconnected.
	if err == io.EOF {
		return false
	}
	if opErr, ok := err.(*net.OpError); ok && !opErr.Temporary() {
		return false
	}

	return true
}

// maybeAddDeadline potentially adds a deadline for the appropriate expected
// response for the passed wire protocol command to the pending responses map.
func (p *LocalPeer) maybeAddDeadline(pendingResponses map[string]time.Time, msgCmd string) {
	// Setup a deadline for each message being sent that expects a response.
	//
	// NOTE: Pings are intentionally ignored here since they are typically
	// sent asynchronously and as a result of a long backlock of messages,
	// such as is typical in the case of initial block download, the
	// response won't be received in time.
	deadline := time.Now().Add(stallResponseTimeout)
	switch msgCmd {
	case wire.CmdVersion:
		// Expects a verack message.
		pendingResponses[wire.CmdVerAck] = deadline

	case wire.CmdMemPool:
		// Expects an inv message.
		pendingResponses[wire.CmdInv] = deadline

	case wire.CmdGetBlocks:
		// Expects an inv message.
		pendingResponses[wire.CmdInv] = deadline

	case wire.CmdGetData:
		// Expects a block, merkleblock, tx, or notfound message.
		pendingResponses[wire.CmdBlock] = deadline
		pendingResponses[wire.CmdMerkleBlock] = deadline
		pendingResponses[wire.CmdTx] = deadline
		pendingResponses[wire.CmdNotFound] = deadline

	case wire.CmdGetHeaders:
		// Expects a headers message.  Use a longer deadline since it
		// can take a while for the remote peer to load all of the
		// headers.
		deadline = time.Now().Add(stallResponseTimeout * 3)
		pendingResponses[wire.CmdHeaders] = deadline
	}
}

// inHandler handles all incoming messages for the peer.  It must be run as a
// goroutine.
func (p *LocalPeer) inHandler() {
	// The timer is stopped when a new message is received and reset after it
	// is processed.
	idleTimer := time.AfterFunc(idleTimeout, func() {
		log.Warnf("localpeer %s no answer for %s -- disconnecting", p, idleTimeout)
		p.Disconnect()
	})

out:
	for atomic.LoadInt32(&p.disconnect) == 0 {
		// Read a message and stop the idle timer as soon as the read
		// is done.  The timer is reset below for the next iteration if
		// needed.
		rmsg, _, err := p.readMessage()
		idleTimer.Stop()
		if err != nil {
			// In order to allow regression tests with malformed messages, don't
			// disconnect the peer when we're in regression test mode and the
			// error is one of the allowed errors.
			if p.isAllowedReadError(err) {
				log.Errorf("Allowed test error from %s: %v", p, err)
				idleTimer.Reset(idleTimeout)
				continue
			}

			// Since the protocol version is 70016 but we don't
			// implement compact blocks, we have to ignore unknown
			// messages after the version-verack handshake. This
			// matches bitcoind's behavior and is necessary since
			// compact blocks negotiation occurs after the
			// handshake.
			if err == wire.ErrUnknownMessage {
				log.Debugf("Received unknown message from %s:"+
					" %v", p, err)
				idleTimer.Reset(idleTimeout)
				continue
			}

			// Only log the error and send reject message if the
			// local peer is not forcibly disconnecting and the
			// remote peer has not disconnected.
			if p.shouldHandleReadError(err) {
				errMsg := fmt.Sprintf("Can't read message from %s: %v", p, err)
				if err != io.ErrUnexpectedEOF {
					log.Errorf(errMsg)
				}

				// Push a reject message for the malformed message and wait for
				// the message to be sent before disconnecting.
				//
				// NOTE: Ideally this would include the command in the header if
				// at least that much of the message was valid, but that is not
				// currently exposed by wire, so just used malformed for the
				// command.
				p.PushRejectMsg("malformed", validatorcommand.RejectMalformed, errMsg, true)
			}
			break out
		}
		atomic.StoreInt64(&p.lastRecv, time.Now().Unix())
		// p.stallControl <- stallControlMsg{sccReceiveMessage, rmsg}

		// // Handle each supported message type.
		// p.stallControl <- stallControlMsg{sccHandlerStart, rmsg}
		switch msg := rmsg.(type) {
		case *validatorcommand.MsgVersion:
			// Limit to one version message per peer.
			p.PushRejectMsg(msg.Command(), validatorcommand.RejectDuplicate,
				"duplicate version message", true)
			break out

		case *validatorcommand.MsgVerAck:
			// Limit to one verack message per peer.
			p.PushRejectMsg(
				msg.Command(), validatorcommand.RejectDuplicate,
				"duplicate verack message", true,
			)
			break out

		case *validatorcommand.MsgPing:
			p.handlePingMsg(msg)
			// if p.cfg.Listeners.OnPing != nil {
			// 	p.cfg.Listeners.OnPing(p, msg)
			// }

		case *validatorcommand.MsgPong:
			p.handlePongMsg(msg)
			// if p.cfg.Listeners.OnPong != nil {
			// 	p.cfg.Listeners.OnPong(p, msg)
			// }

		// case *validatorcommand.MsgNotFound:
		// 	if p.cfg.Listeners.OnNotFound != nil {
		// 		p.cfg.Listeners.OnNotFound(p, msg)
		// 	}

		default:
			log.Debugf("Received unhandled message of type %v "+
				"from %v", rmsg.Command(), p)
		}
		//p.stallControl <- stallControlMsg{sccHandlerDone, rmsg}

		// A message was received so reset the idle timer.
		idleTimer.Reset(idleTimeout)
	}

	// Ensure the idle timer is stopped to avoid leaking the resource.
	idleTimer.Stop()

	// Ensure connection is closed.
	p.Disconnect()

	close(p.inQuit)
	log.Tracef("localpeer input handler done for %s", p)
}

// queueHandler handles the queuing of outgoing data for the peer. This runs as
// a muxer for various sources of input so we can ensure that server and peer
// handlers will not block on us sending a message.  That data is then passed on
// to outHandler to be actually written.
func (p *LocalPeer) queueHandler() {
	//	pendingMsgs := list.New()
	invSendQueue := list.New()
	trickleTicker := time.NewTicker(p.cfg.TrickleInterval)
	defer trickleTicker.Stop()

	// We keep the waiting flag so that we know if we have a message queued
	// to the outHandler or not.  We could use the presence of a head of
	// the list for this but then we have rather racy concerns about whether
	// it has gotten it at cleanup time - and thus who sends on the
	// message's done channel.  To avoid such confusion we keep a different
	// flag and pendingMsgs only contains messages that we have not yet
	// passed to outHandler.
	//waiting := false

	// To avoid duplication below.
	// queuePacket := func(msg outMsg, list *list.List, waiting bool) bool {

	// 	// 检查连接是否仍然有效
	// 	if p.conn.RemoteAddr() == nil {
	// 		// 连接已关闭，不能重用
	// 		// The connection has been closed by remote.  Will reconnect, and waiting for current senddidng
	// 		p.reconnect()
	// 		waiting = true
	// 	}
	// 	if !waiting {
	// 		p.sendQueue <- msg
	// 	} else {
	// 		list.PushBack(msg)
	// 	}
	// 	// we are always waiting now.
	// 	return true
	// }
out:
	for {
		select {
		//case msg := <-p.outputQueue:
		//waiting = queuePacket(msg, p.pendingMsgs, waiting)

		// This channel is notified when a message has been sent across
		// the network socket.
		case <-p.sendDoneQueue:
			// No longer waiting if there are no more messages
			// in the pending messages queue.

			p.reconnect()

		case <-trickleTicker.C:
			// Don't send anything if we're disconnecting or there
			// is no queued inventory.
			// version is known if send queue has any entries.
			if atomic.LoadInt32(&p.disconnect) != 0 ||
				invSendQueue.Len() == 0 {
				continue
			}

		case <-p.quit:
			break out
		}
	}

	// Drain any wait channels before we go away so we don't leave something
	// waiting for us.
	// for e := p.pendingMsgs.Front(); e != nil; e = p.pendingMsgs.Front() {
	// 	val := p.pendingMsgs.Remove(e)
	// 	msg := val.(outMsg)
	// 	if msg.doneChan != nil {
	// 		msg.doneChan <- struct{}{}
	// 	}
	// }

cleanup:
	for {
		select {
		case msg := <-p.outputQueue:
			if msg.doneChan != nil {
				msg.doneChan <- struct{}{}
			}
		case <-p.outputInvChan:
			// Just drain channel
		// sendDoneQueue is buffered so doesn't need draining.
		default:
			break cleanup
		}
	}
	close(p.queueQuit)
	p.queueQuit = nil
	log.Tracef("LocalPeer queue handler done for %s", p)
}

// shouldLogWriteError returns whether or not the passed error, which is
// expected to have come from writing to the remote peer in the outHandler,
// should be logged.
func (p *LocalPeer) shouldLogWriteError(err error) bool {
	// No logging when the peer is being forcibly disconnected.
	if atomic.LoadInt32(&p.disconnect) != 0 {
		return false
	}

	// No logging when the remote peer has been disconnected.
	if err == io.EOF {
		return false
	}
	if opErr, ok := err.(*net.OpError); ok && !opErr.Temporary() {
		return false
	}

	return true
}

// outHandler handles all outgoing messages for the peer.  It must be run as a
// goroutine.  It uses a buffered channel to serialize output messages while
// allowing the sender to continue running asynchronously.
func (p *LocalPeer) outHandler() {
out:
	for {
		select {
		case msg := <-p.sendQueue:
			switch m := msg.msg.(type) {
			case *validatorcommand.MsgPing:
				// Only expects a pong message in later protocol
				// versions.  Also set up statistics.
				p.statsMtx.Lock()
				p.lastPingNonce = m.Nonce
				p.lastPingTime = time.Now()
				p.statsMtx.Unlock()
			}

			//p.stallControl <- stallControlMsg{sccSendMessage, msg.msg}

			err := p.writeMessage(msg.msg)
			if err != nil {
				p.Disconnect()
				if p.shouldLogWriteError(err) {
					log.Errorf("Failed to send message to "+
						"%s: %v", p, err)
				}
				if msg.doneChan != nil {
					msg.doneChan <- struct{}{}
				}
				continue
			}

			// At this point, the message was successfully sent, so
			// update the last send time, signal the sender of the
			// message that it has been sent (if requested), and
			// signal the send queue to the deliver the next queued
			// message.
			atomic.StoreInt64(&p.lastSend, time.Now().Unix())
			if msg.doneChan != nil {
				msg.doneChan <- struct{}{}
			}
			p.sendDoneQueue <- struct{}{}

		case <-p.quit:
			break out
		}
	}

	<-p.queueQuit

	// Drain any wait channels before we go away so we don't leave something
	// waiting for us. We have waited on queueQuit and thus we can be sure
	// that we will not miss anything sent on sendQueue.
cleanup:
	for {
		select {
		case msg := <-p.sendQueue:
			if msg.doneChan != nil {
				msg.doneChan <- struct{}{}
			}
			// no need to send on sendDoneQueue since queueHandler
			// has been waited on and already exited.
		default:
			break cleanup
		}
	}
	close(p.outQuit)
	log.Tracef("LocalPeer output handler done for %s", p)
}

// pingHandler periodically pings the peer.  It must be run as a goroutine.
func (p *LocalPeer) pingHandler() {
	pingTicker := time.NewTicker(pingInterval)
	defer pingTicker.Stop()

out:
	for {
		select {
		case <-pingTicker.C:
			nonce, err := wire.RandomUint64()
			if err != nil {
				log.Errorf("Not sending ping to %s: %v", p, err)
				continue
			}
			log.Debugf("**********Sending \"ping\" to validator peer [%s] with nonce=%d", p, nonce)
			p.QueueMessage(validatorcommand.NewMsgPing(nonce), nil)

		case <-p.quit:
			break out
		}
	}
}

// QueueMessage adds the passed bitcoin message to the peer send queue.
//
// This function is safe for concurrent access.
func (p *LocalPeer) QueueMessage(msg validatorcommand.Message, doneChan chan<- struct{}) {
	p.QueueMessageWithEncoding(msg, doneChan)
}

// QueueMessageWithEncoding adds the passed bitcoin message to the peer send
// queue. This function is identical to QueueMessage, however it allows the
// caller to specify the wire encoding type that should be used when
// encoding/decoding blocks and transactions.
//
// This function is safe for concurrent access.
func (p *LocalPeer) QueueMessageWithEncoding(msg validatorcommand.Message, doneChan chan<- struct{}) {

	// Avoid risk of deadlock if goroutine already exited.  The goroutine
	// we will be sending to hangs around until it knows for a fact that
	// it is marked as disconnected and *then* it drains the channels.
	if !p.Connected() {
		if doneChan != nil {
			go func() {
				doneChan <- struct{}{}
			}()
		}
		return
	}
	p.outputQueue <- outMsg{msg: msg, doneChan: doneChan}
}

// Connected returns whether or not the peer is currently connected.
//
// This function is safe for concurrent access.
func (p *LocalPeer) Connected() bool {
	return atomic.LoadInt32(&p.connected) != 0 &&
		atomic.LoadInt32(&p.disconnect) == 0
}

// Disconnect disconnects the peer by closing the connection.  Calling this
// function when the peer is already disconnected or in the process of
// disconnecting will have no effect.
func (p *LocalPeer) Disconnect() {
	if atomic.AddInt32(&p.disconnect, 1) != 1 {
		return
	}

	log.Tracef("Disconnecting %s", p)
	if atomic.LoadInt32(&p.connected) != 0 {
		p.conn.Close()
	}
	close(p.quit)
}

func (p *LocalPeer) reconnect() {
	// Disconnect first
	if atomic.AddInt32(&p.disconnect, 1) != 1 {
		return
	}

	log.Tracef("Disconnecting %s", p)
	if atomic.LoadInt32(&p.connected) != 0 {
		p.conn.Close()
	}

	// Connect
	if p.addrsList == nil || len(p.addrsList) == 0 {
		return
	}

	log.Debugf("----------Will reconnect a remote validator peer connected to peer %v.", p.addrsList)

	// Start to handle all peer
	//p.start()

	index := 0 // Default to the first address to be dial
	if len(p.addrsList) > 1 {
		// The peer has more than one address. Select a random address
		index = rand.Intn(len(p.addrsList))
	}
	addr := p.addrsList[index]
	conn, err := p.cfg.Dial(addr)
	if err != nil {
		log.Errorf("----------Unable to connect to %s: %v", addr, err)
		return
	}

	p.conn = conn
	p.connected = 1

	// Send next message if the pending list is not empty
	//p.sendDoneQueue <- struct{}{}
	p.sendNextCommand()
}

func (p *LocalPeer) sendNextCommand() bool {
	// next := p.pendingMsgs.Front()
	// if next == nil {
	// 	return false
	// }

	// // Notify the outHandler about the next item to
	// // asynchronously send.
	// val := p.pendingMsgs.Remove(next)
	// p.sendQueue <- val.(outMsg)
	return true
}

// readRemoteVersionMsg waits for the next message to arrive from the remote
// peer.  If the next message is not a version message or the version is not
// acceptable then return an error.
func (p *LocalPeer) readRemoteVersionMsg() error {
	// Read their version message.
	remoteMsg, _, err := p.readMessage()
	if err != nil {
		return err
	}

	// Notify and disconnect clients if the first message is not a version
	// message.
	msg, ok := remoteMsg.(*validatorcommand.MsgVersion)
	if !ok {
		reason := "a version message must precede all others"
		rejectMsg := validatorcommand.NewMsgReject(msg.Command(), validatorcommand.RejectMalformed,
			reason)
		_ = p.writeMessage(rejectMsg)
		return errors.New(reason)
	}

	// Detect self connections.
	if !p.cfg.AllowSelfConns && sentNonces.Contains(msg.Nonce) {
		return errors.New("disconnecting peer connected to self")
	}

	// Negotiate the protocol version and set the services to what the remote
	// peer advertised.
	p.flagsMtx.Lock()
	p.advertisedProtoVer = uint32(msg.ValidatorVersion)
	p.validatorVersion = minUint32(p.validatorVersion, p.advertisedProtoVer)
	p.versionKnown = true
	//p.services = msg.Services
	p.flagsMtx.Unlock()
	log.Debugf("Negotiated protocol version %d for peer %s",
		p.validatorVersion, p)

	// Updating a bunch of stats including block based stats, and the
	// peer's time offset.
	p.statsMtx.Lock()
	//p.lastBlock = msg.LastBlock
	//p.startingHeight = msg.LastBlock
	p.timeOffset = msg.Timestamp.Unix() - time.Now().Unix()
	p.statsMtx.Unlock()

	// Set the peer's ID, user agent, and potentially the flag which
	// specifies the witness support is enabled.
	p.flagsMtx.Lock()
	p.id = atomic.AddInt32(&nodeCount, 1)
	//p.userAgent = msg.UserAgent

	// Determine if the peer would like to receive witness data with
	// transactions, or not.
	if p.services&wire.SFNodeWitness == wire.SFNodeWitness {
		p.witnessEnabled = true
	}
	p.flagsMtx.Unlock()

	// Once the version message has been exchanged, we're able to determine
	// if this peer knows how to encode witness data over the wire
	// protocol. If so, then we'll switch to a decoding mode which is
	// prepared for the new transaction format introduced as part of
	// BIP0144.
	if p.services&wire.SFNodeWitness == wire.SFNodeWitness {
		p.wireEncoding = wire.WitnessEncoding
	}

	// Invoke the callback if specified.
	// if p.cfg.Listeners.OnVersion != nil {
	// 	rejectMsg := p.cfg.Listeners.OnVersion(p, msg)
	// 	if rejectMsg != nil {
	// 		_ = p.writeMessage(rejectMsg)
	// 		return errors.New(rejectMsg.Reason)
	// 	}
	// }

	return nil
}

// processRemoteVerAckMsg takes the verack from the remote peer and handles it.
func (p *LocalPeer) processRemoteVerAckMsg(msg *validatorcommand.MsgVerAck) {
	p.flagsMtx.Lock()
	p.verAckReceived = true
	p.flagsMtx.Unlock()

	// if p.cfg.Listeners.OnVerAck != nil {
	// 	p.cfg.Listeners.OnVerAck(p, msg)
	// }
}

// localVersionMsg creates a version message that can be used to send to the
// remote peer.
func (p *LocalPeer) localVersionMsg() (*validatorcommand.MsgVersion, error) {
	theirNA := p.na.ToLegacy()

	// If p.na is a torv3 hidden service address, we'll need to send over
	// an empty NetAddress for their address.
	if p.na.IsTorV3() {
		theirNA = wire.NewNetAddressIPPort(
			net.IP([]byte{0, 0, 0, 0}), p.na.Port, p.na.Services,
		)
	}

	// If we are behind a proxy and the connection comes from the proxy then
	// we return an unroutable address as their address. This is to prevent
	// leaking the tor proxy address.
	if p.cfg.Proxy != "" {
		proxyaddress, _, err := net.SplitHostPort(p.cfg.Proxy)
		// invalid proxy means poorly configured, be on the safe side.
		if err != nil || p.na.Addr.String() == proxyaddress {
			theirNA = wire.NewNetAddressIPPort(net.IP([]byte{0, 0, 0, 0}), 0,
				theirNA.Services)
		}
	}

	// Create a wire.NetAddress with only the services set to use as the
	// "addrme" in the version message.
	//
	// Older nodes previously added the IP and port information to the
	// address manager which proved to be unreliable as an inbound
	// connection from a peer didn't necessarily mean the peer itself
	// accepted inbound connections.
	//
	// Also, the timestamp is unused in the version message.
	// ourNA := &wire.NetAddress{
	// 	Services: p.cfg.Services,
	// }

	// Generate a unique nonce for this peer so self connections can be
	// detected.  This is accomplished by adding it to a size-limited map of
	// recently seen nonces.
	nonce := uint64(rand.Int63())
	sentNonces.Add(nonce)

	// Version message.
	msg := validatorcommand.NewMsgVersion(nonce)

	return msg, nil
}

// writeLocalVersionMsg writes our version message to the remote peer.
func (p *LocalPeer) writeLocalVersionMsg() error {
	localVerMsg, err := p.localVersionMsg()
	if err != nil {
		return err
	}

	return p.writeMessage(localVerMsg)
}

// waitToFinishNegotiation waits until desired negotiation messages are
// received, recording the remote peer's preference for sendaddrv2 as an
// example. The list of negotiated features can be expanded in the future. If a
// verack is received, negotiation stops and the connection is live.
func (p *LocalPeer) waitToFinishNegotiation(pver uint32) error {
	// There are several possible messages that can be received here. We
	// could immediately receive verack and be done with the handshake. We
	// could receive sendaddrv2 and still have to wait for verack. Or we
	// can receive unknown messages before and after sendaddrv2 and still
	// have to wait for verack.
	for {
		remoteMsg, _, err := p.readMessage()
		if err == wire.ErrUnknownMessage {
			continue
		} else if err != nil {
			return err
		}

		switch m := remoteMsg.(type) {
		// case *wire.MsgSendAddrV2:
		// 	if pver >= wire.AddrV2Version {
		// 		p.flagsMtx.Lock()
		// 		p.sendAddrV2 = true
		// 		p.flagsMtx.Unlock()

		// 		if p.cfg.Listeners.OnSendAddrV2 != nil {
		// 			p.cfg.Listeners.OnSendAddrV2(p, m)
		// 		}
		// 	}
		case *validatorcommand.MsgVerAck:
			// Receiving a verack means we are done with the
			// handshake.
			p.processRemoteVerAckMsg(m)
			return nil
		default:
			// This is triggered if the peer sends, for example, a
			// GETDATA message during this negotiation.
			return wire.ErrInvalidHandshake
		}
	}
}

// negotiateInboundProtocol performs the negotiation protocol for an inbound
// peer. The events should occur in the following order, otherwise an error is
// returned:
//
//  1. Remote peer sends their version.
//  2. We send our version.
//  3. We send sendaddrv2 if their version is >= 70016.
//  4. We send our verack.
//  5. Wait until sendaddrv2 or verack is received. Unknown messages are
//     skipped as it could be wtxidrelay or a different message in the future
//     that btcd does not implement but bitcoind does.
//  6. If remote peer sent sendaddrv2 above, wait until receipt of verack.
func (p *LocalPeer) negotiateInboundProtocol() error {
	if err := p.readRemoteVersionMsg(); err != nil {
		return err
	}

	if err := p.writeLocalVersionMsg(); err != nil {
		return err
	}

	var validatorVersion uint32
	p.flagsMtx.Lock()
	validatorVersion = p.validatorVersion
	p.flagsMtx.Unlock()

	// if err := p.writeSendAddrV2Msg(protoVersion); err != nil {
	// 	return err
	// }

	// err := p.writeMessage(wire.NewMsgVerAck(), wire.LatestEncoding)
	// if err != nil {
	// 	return err
	// }

	// Finish the negotiation by waiting for negotiable messages or verack.
	return p.waitToFinishNegotiation(validatorVersion)
}

// negotiateOutboundProtocol performs the negotiation protocol for an outbound
// peer. The events should occur in the following order, otherwise an error is
// returned:
//
//  1. We send our version.
//  2. Remote peer sends their version.
//  3. We send sendaddrv2 if their version is >= 70016.
//  4. We send our verack.
//  5. We wait to receive sendaddrv2 or verack, skipping unknown messages as
//     in the inbound case.
//  6. If sendaddrv2 was received, wait for receipt of verack.
func (p *LocalPeer) negotiateOutboundProtocol() error {
	if err := p.writeLocalVersionMsg(); err != nil {
		return err
	}

	if err := p.readRemoteVersionMsg(); err != nil {
		return err
	}

	var validatorVersion uint32
	p.flagsMtx.Lock()
	validatorVersion = p.validatorVersion
	p.flagsMtx.Unlock()

	// if err := p.writeSendAddrV2Msg(protoVersion); err != nil {
	// 	return err
	// }

	// err := p.writeMessage(wire.NewMsgVerAck(), wire.LatestEncoding)
	// if err != nil {
	// 	return err
	// }

	// Finish the negotiation by waiting for negotiable messages or verack.
	return p.waitToFinishNegotiation(validatorVersion)
}

// start begins processing input and output messages.
func (p *LocalPeer) start() error {
	log.Tracef("Starting peer %s", p)

	negotiateErr := make(chan error, 1)
	go func() {
		if p.inbound {
			negotiateErr <- p.negotiateInboundProtocol()
		} else {
			negotiateErr <- p.negotiateOutboundProtocol()
		}
	}()

	// Negotiate the protocol within the specified negotiateTimeout.
	select {
	case err := <-negotiateErr:
		if err != nil {
			p.Disconnect()
			return err
		}
	case <-time.After(negotiateTimeout):
		p.Disconnect()
		return errors.New("protocol negotiation timeout")
	}
	log.Debugf("Connected to %s", p.Addr())

	// The protocol has been negotiated successfully so start processing input
	// and output messages.
	go p.inHandler()
	go p.queueHandler()
	go p.outHandler()
	go p.pingHandler()

	return nil
}

// WaitForDisconnect waits until the peer has completely disconnected and all
// resources are cleaned up.  This will happen if either the local or remote
// side has been disconnected or the peer is forcibly disconnected via
// Disconnect.
func (p *LocalPeer) WaitForDisconnect() {
	<-p.quit
}

// newPeerBase returns a new base bitcoin peer based on the inbound flag.  This
// is used by the NewInboundPeer and NewOutboundPeer functions to perform base
// setup needed by both types of peers.
func newPeerBase(origCfg *LocalPeerConfig, inbound bool) *LocalPeer {
	// Default to the max supported protocol version if not specified by the
	// caller.
	cfg := *origCfg // Copy to avoid mutating caller.
	if cfg.ValidatorVersion == 0 {
		cfg.ValidatorVersion = MaxValidatorVersion
	}

	// Set the chain parameters to testnet if the caller did not specify any.
	if cfg.ChainParams == nil {
		cfg.ChainParams = &chaincfg.TestNet3Params
	}

	// Set the trickle interval if a non-positive value is specified.
	if cfg.TrickleInterval <= 0 {
		cfg.TrickleInterval = DefaultTrickleInterval
	}

	p := LocalPeer{
		inbound:          inbound,
		wireEncoding:     wire.BaseEncoding,
		stallControl:     make(chan stallControlMsg, 1), // nonblocking sync
		outputQueue:      make(chan outMsg, outputBufferSize),
		sendQueue:        make(chan outMsg, 1),   // nonblocking sync
		sendDoneQueue:    make(chan struct{}, 1), // nonblocking sync
		outputInvChan:    make(chan *wire.InvVect, outputBufferSize),
		inQuit:           make(chan struct{}),
		queueQuit:        make(chan struct{}),
		outQuit:          make(chan struct{}),
		quit:             make(chan struct{}),
		cfg:              cfg, // Copy so caller can't mutate.
		validatorVersion: cfg.ValidatorVersion,

		connReqIndex: 1,
		connMap:      make(map[uint64]*ConnReq),
	}
	return &p
}

// NewLocalpeer returns a new local validator peer. If the Config argument
// does not set HostToNetAddress, connecting to anything other than an ipv4 or
// ipv6 address will fail and may cause a nil-pointer-dereference. This
// includes hostnames and onion services.
func NewLocalPeer(cfg *LocalPeerConfig, addrs []net.Addr) (*LocalPeer, error) {
	p := newPeerBase(cfg, false)

	p.addrsList = make([]net.Addr, 0, len(addrs))
	p.addrsList = append(p.addrsList, addrs...)

	log.Debugf("NewLocalpeer IP:")
	for _, addr := range p.addrsList {
		log.Debugf("  %s", addr.String())
	}

	p.addr = addrs[0].String() // Default to the first address

	host, portStr, err := net.SplitHostPort(p.addr)
	if err != nil {
		return nil, err
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, err
	}

	if cfg.HostToNetAddress != nil {
		na, err := cfg.HostToNetAddress(host, uint16(port), 0)
		if err != nil {
			return nil, err
		}
		p.na = na
	} else {
		// If host is an onion hidden service or a hostname, it is
		// likely that a nil-pointer-dereference will occur. The caller
		// should set HostToNetAddress if connecting to these.
		p.na = wire.NetAddressV2FromBytes(
			time.Now(), 0, net.ParseIP(host), uint16(port),
		)
	}

	return p, nil
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Connect assigns an id and dials a connection to the address of the
// connection request.
func (p *LocalPeer) Connect() {
	if p.addrsList == nil || len(p.addrsList) == 0 {
		return
	}
	log.Debugf("----------Will connect a remote validator peer connected to peer %v.", p.addrsList)

	// Start to handle all peer
	//p.start()

	index := 0 // Default to the first address to be dial
	if len(p.addrsList) > 1 {
		// The peer has more than one address. Select a random address
		index = rand.Intn(len(p.addrsList))
	}
	addr := p.addrsList[index]
	conn, err := p.cfg.Dial(addr)
	if err != nil {
		log.Errorf("----------Unable to connect to %s: %v", addr, err)
		return
	}

	p.conn = conn
	p.connected = 1

	// connect the peer with tinmer
	go p.queueHandler()
	go p.outHandler()
	go p.pingHandler()

	log.Debugf("----------Connected to validator peer [%s] succeed.", addr)
}

func (p *LocalPeer) Start() error {
	// initialize listeners for received messages from other peers to connect to this peer
	listeners, err := p.initListeners()

	if err != nil {
		log.Errorf("Unable to initialize listeners: %v", err)
		return err
	}

	// Start all the listeners so long as the caller requested them and
	// provided a callback to be invoked when connections are accepted.
	for _, listener := range listeners {
		p.wg.Add(1)
		go p.listenHandler(listener)
	}

	// go p.queueHandler()
	// go p.outHandler()

	return nil
}

// initListeners initializes the configured net listeners and adds any bound
// addresses to the address manager. Returns the listeners.
func (p *LocalPeer) initListeners() ([]net.Listener, error) {
	// Listen for TCP connections at the configured addresses

	listeners := make([]net.Listener, 0, len(p.addrsList))
	for _, addr := range p.addrsList {
		network := addr.Network()
		listenAddr := addr.String()
		log.Debugf("----------Listen on %s : %s", network, listenAddr)
		listener, err := net.Listen(network, listenAddr)
		if err != nil {
			log.Warnf("----------Can't listen on %s: %v", addr, err)
			continue
		}
		listeners = append(listeners, listener)
	}

	return listeners, nil
}

// listenHandler accepts incoming connections on a given listener.  It must be
// run as a goroutine.
func (p *LocalPeer) listenHandler(listener net.Listener) {
	log.Infof("----------Validator peer listening on %s", listener.Addr())
	for atomic.LoadInt32(&p.stop) == 0 {

		log.Debugf("----------Will accept connection from %v", listener.Addr())
		conn, err := listener.Accept()
		if err != nil {
			// Only log the error if not forcibly shutting down.
			if atomic.LoadInt32(&p.stop) == 0 {
				log.Errorf("----------Can't accept connection: %v", err)
			}
			return
		}

		// The local peer is connected to the remote peer, will create a new conn to manage it
		log.Debugf("----------New connected from %v", conn.RemoteAddr())

		newConnReq := &ConnReq{
			id:          atomic.LoadUint64(&p.connReqIndex),
			LocalAddr:   conn.LocalAddr(),
			RemoteAddr:  conn.RemoteAddr(),
			conn:        conn,
			pendingCmds: list.New(),
			CmdsLock:    sync.RWMutex{},
		}
		newConnReq.setLastActive()
		newConnReq.logConnInfo("newConnReq")

		atomic.AddUint64(&p.connReqIndex, 1)

		p.connMap[newConnReq.id] = newConnReq

		// Start to listen the command from this conn
		go p.listenCommand(newConnReq)
	}

	log.Debugf("----------listener %s has stopped ", listener.Addr())
	p.wg.Done()
	log.Debugf("Listener handler done for %s", listener.Addr())
}

func (p *LocalPeer) listenCommand(connReq *ConnReq) {
	for atomic.LoadInt32(&connReq.connClose) == 0 {
		log.Debugf("----------[LocalPeer]Will read command from conn[%d]: %s to %s", connReq.id, connReq.RemoteAddr, connReq.LocalAddr)
		_, command, _, err := validatorcommand.ReadMessage(connReq.conn, p.ValidatorVersion(), p.cfg.ChainParams.Net)
		if err != nil {
			log.Errorf("----------[LocalPeer]conn[%d]: Read message err: %v", connReq.id, err)
			return
		}
		log.Debugf("----------[LocalPeer]Received validator command [%v] from %d", command.Command(), connReq.id)

		connReq.setLastActive()
		// TODO: handle the command
		go p.handleCommand(connReq, command)
	}
}

func (p *LocalPeer) handleCommand(connReq *ConnReq, command validatorcommand.Message) {

}

func (connReq *ConnReq) close() {
	atomic.StoreInt32(&connReq.connClose, 1)
	connReq.conn.Close()
}

func (connReq *ConnReq) setLastActive() {
	atomic.StoreInt64(&connReq.lastActive, time.Now().Unix())
}

func (connReq *ConnReq) logConnInfo(desc string) {
	log.Debugf("——————————————————Conn info: %s——————————————————", desc)
	log.Debugf("Conn id: %d", connReq.id)
	log.Debugf("Conn LocalAddr: %s", connReq.LocalAddr)
	log.Debugf("Conn RemoteAddr: %s", connReq.RemoteAddr)

	lastActiviteTime := time.Unix(0, atomic.LoadInt64(&connReq.lastActive))
	log.Debugf("Conn Last active time: %s", lastActiviteTime.String())

	log.Debugf("Pending Commands Count: %d", connReq.pendingCmds.Len())

	// for e := connReq.pendingCmds.Front(); e != nil; e = e.Next() {
	// 	command := e.Value.(*validatorcommand.Message)
	// 	log.Debugf("Pending Command: %s", command.Command())
	// }

	log.Debugf("——————————————————Conn info: %s End——————————————————", desc)
}

func (connReq *ConnReq) SendCommand(command *validatorcommand.Message) {
	connReq.CmdsLock.Lock()
	connReq.pendingCmds.PushBack(command)
	connReq.CmdsLock.Unlock()
}
