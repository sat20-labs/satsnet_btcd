// Copyright (c) 2013-2018 The btcsuite developers
// Copyright (c) 2016-2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package validatorpeer

import (
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sat20-labs/satsnet_btcd/chaincfg"
	"github.com/sat20-labs/satsnet_btcd/chaincfg/chainhash"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validatorcommand"
	"github.com/sat20-labs/satsnet_btcd/wire"
)

// RemotePeerInterface defines callback function pointers to invoke by local
// peers. Include notify from peer to manager, and get info from manager.

// All notify functions should be start with "On". And all get functions should
// be start with "Get".
type RemotePeerInterface interface {
	// OnPeerDisconnected is invoked when a remote peer connects to the local peer .
	OnPeerDisconnected(net.Addr)
}

// Config is the struct to hold configuration options useful to localpeer.
type RemotePeerConfig struct {
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

	// RemotePeerInterface to be used by peer manager.
	RemoteValidator RemotePeerInterface

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

	LocalValidatorId uint64 // local validator id

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
type RemotePeer struct {
	// The following variables must only be used atomically.
	bytesReceived uint64
	bytesSent     uint64
	lastRecv      int64
	lastSend      int64
	connected     int32
	disconnect    int32

	connReq        *ConnReq // map of all connections, key is conn id, value is conn req
	reconnectTimes int64    // if the peer is disconnected, will reconnect again, reconnectTimes is recoed the times of reconnect
	//conn net.Conn
	//stop int32
	//wg   sync.WaitGroup

	// These fields are set at creation time and never modified, so they are
	// safe to read from concurrently without a mutex.
	//addrsList []net.Addr // All the addresses reported by the peer
	addr    net.Addr // default addr for connected to peer
	cfg     RemotePeerConfig
	inbound bool

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

	LocalValidatorId uint64 // local validator id

}

// String returns the peer's address and directionality as a human-readable
// string.
//
// This function is safe for concurrent access.
func (p *RemotePeer) String() string {
	return fmt.Sprintf("RemotePeer : %s", p.addr)
}

// StatsSnapshot returns a snapshot of the current peer flags and statistics.
//
// This function is safe for concurrent access.
func (p *RemotePeer) StatsSnapshot() *StatsSnap {
	// Get a copy of all relevant flags and stats.
	statsSnap := &StatsSnap{}
	return statsSnap
}

// ID returns the peer id.
//
// This function is safe for concurrent access.
func (p *RemotePeer) ID() int32 {
	p.flagsMtx.Lock()
	id := p.id
	p.flagsMtx.Unlock()

	return id
}

// NA returns the peer network address.
//
// This function is safe for concurrent access.
func (p *RemotePeer) NA() *wire.NetAddressV2 {
	p.flagsMtx.Lock()
	na := p.na
	p.flagsMtx.Unlock()

	return na
}

// Addr returns the peer address.
//
// This function is safe for concurrent access.
func (p *RemotePeer) Addr() string {
	// The address doesn't change after initialization, therefore it is not
	// protected by a mutex.
	return p.addr.String()
}

// Addr returns the peer address.
//
// This function is safe for concurrent access.
func (p *RemotePeer) GetPeerAddr() net.Addr {
	// The address doesn't change after initialization, therefore it is not
	// protected by a mutex.
	return p.addr
}

// UserAgent returns the user agent of the remote peer.
//
// This function is safe for concurrent access.
func (p *RemotePeer) UserAgent() string {
	p.flagsMtx.Lock()
	userAgent := p.userAgent
	p.flagsMtx.Unlock()

	return userAgent
}

// LastPingNonce returns the last ping nonce of the remote peer.
//
// This function is safe for concurrent access.
func (p *RemotePeer) LastPingNonce() uint64 {
	p.statsMtx.RLock()
	lastPingNonce := p.lastPingNonce
	p.statsMtx.RUnlock()

	return lastPingNonce
}

// LastPingTime returns the last ping time of the remote peer.
//
// This function is safe for concurrent access.
func (p *RemotePeer) LastPingTime() time.Time {
	p.statsMtx.RLock()
	lastPingTime := p.lastPingTime
	p.statsMtx.RUnlock()

	return lastPingTime
}

// LastPingMicros returns the last ping micros of the remote peer.
//
// This function is safe for concurrent access.
func (p *RemotePeer) LastPingMicros() int64 {
	p.statsMtx.RLock()
	lastPingMicros := p.lastPingMicros
	p.statsMtx.RUnlock()

	return lastPingMicros
}

// ProtocolVersion returns the negotiated peer protocol version.
//
// This function is safe for concurrent access.
func (p *RemotePeer) ValidatorVersion() uint32 {
	p.flagsMtx.Lock()
	validatorVersion := p.validatorVersion
	p.flagsMtx.Unlock()

	return validatorVersion
}

// LastSend returns the last send time of the peer.
//
// This function is safe for concurrent access.
func (p *RemotePeer) LastSend() time.Time {
	return time.Unix(atomic.LoadInt64(&p.lastSend), 0)
}

// LastRecv returns the last recv time of the peer.
//
// This function is safe for concurrent access.
func (p *RemotePeer) LastRecv() time.Time {
	return time.Unix(atomic.LoadInt64(&p.lastRecv), 0)
}

// LocalAddr returns the local address of the connection.
//
// This function is safe for concurrent access.
func (p *RemotePeer) LocalAddr() net.Addr {
	var localAddr net.Addr
	// if atomic.LoadInt32(&p.connected) != 0 {
	// 	localAddr = p.conn.LocalAddr()
	// }
	return localAddr
}

// BytesSent returns the total number of bytes sent by the peer.
//
// This function is safe for concurrent access.
func (p *RemotePeer) BytesSent() uint64 {
	return atomic.LoadUint64(&p.bytesSent)
}

// BytesReceived returns the total number of bytes received by the peer.
//
// This function is safe for concurrent access.
func (p *RemotePeer) BytesReceived() uint64 {
	return atomic.LoadUint64(&p.bytesReceived)
}

// TimeConnected returns the time at which the peer connected.
//
// This function is safe for concurrent access.
func (p *RemotePeer) TimeConnected() time.Time {
	p.statsMtx.RLock()
	timeConnected := p.timeConnected
	p.statsMtx.RUnlock()

	return timeConnected
}

// newPeerBase returns a new base bitcoin peer based on the inbound flag.  This
// is used by the NewInboundPeer and NewOutboundPeer functions to perform base
// setup needed by both types of peers.
func newRemotePeerBase(origCfg *RemotePeerConfig, inbound bool) *RemotePeer {
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

	p := RemotePeer{
		inbound:          inbound,
		cfg:              cfg, // Copy so caller can't mutate.
		validatorVersion: cfg.ValidatorVersion,
		LocalValidatorId: cfg.LocalValidatorId,
		// connReqIndex: 1,
		// connMap:      make(map[uint64]*ConnReq),
	}
	return &p
}

// NewLocalpeer returns a new local validator peer. If the Config argument
// does not set HostToNetAddress, connecting to anything other than an ipv4 or
// ipv6 address will fail and may cause a nil-pointer-dereference. This
// includes hostnames and onion services.
func NewRemotePeer(cfg *RemotePeerConfig, addr net.Addr) (*RemotePeer, error) {
	p := newRemotePeerBase(cfg, false)

	// p.addrsList = make([]net.Addr, 0, len(addrs))
	// p.addrsList = append(p.addrsList, addrs...)

	log.Debugf("NewRemotepeer (%s) with local validator ID: %d", addr.String(), p.LocalValidatorId)

	p.addr = addr

	host, portStr, err := net.SplitHostPort(p.addr.String())
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

func (p *RemotePeer) Connected() bool {
	if atomic.LoadInt32(&p.connected) == 0 {
		return false
	}

	if p.connReq == nil || p.connReq.isInactive() {
		return false
	}

	return true

}

func (p *RemotePeer) Connect() error {
	conn, err := p.cfg.Dial(p.addr)
	if err != nil {
		log.Errorf("***********Unable to connect to %s: %v", p.addr, err)
		return err
	}

	newConnReq := &ConnReq{
		id:           1024, // remote peer id
		LocalAddr:    conn.LocalAddr(),
		RemoteAddr:   conn.RemoteAddr(),
		conn:         conn,
		CmdsLock:     sync.RWMutex{},
		btcnet:       p.cfg.ChainParams.Net,
		version:      p.ValidatorVersion(),
		lastReceived: time.Now().Unix(), // default value, connected time is last received time
	}
	//newConnReq.setLast()
	newConnReq.logConnInfo("newConnReq")

	newConnReq.Start()

	atomic.StoreInt32(&p.connected, 1)

	p.connReq = newConnReq
	p.reconnectTimes = 0
	go p.listenCommand(newConnReq)

	// new pingHandler to send ping timer
	go p.pingHandler()

	return nil
}

// pingHandler periodically pings the peer.  It must be run as a goroutine.
func (p *RemotePeer) pingHandler() {
	pingTicker := time.NewTicker(pingInterval)
	defer pingTicker.Stop()

out:
	for {
		select {
		case <-pingTicker.C:
			if p.connReq.isInactive() {
				// Current conn is inactive, will try to reconnect
				p.reconnectTimes++

				if p.reconnectTimes > peerReconnectMaxTimes {
					log.Errorf("***********Reconnect times too much, will disconnect the validator peer. %d", p.reconnectTimes)
					p.cfg.RemoteValidator.OnPeerDisconnected(p.addr)
					// The peer is disconnected, will exit ping handler
					break out
				}

				log.Debugf("***********Reconnect to validator peer [%s]", p)
				p.Connect()
				continue
			}
			nonce, err := wire.RandomUint64()
			if err != nil {
				log.Errorf("Not sending ping to %s: %v", p, err)
				continue
			}
			log.Debugf("**********Sending \"ping\" to validator peer [%s] with nonce=%d", p, nonce)
			//p.QueueMessage(validatorcommand.NewMsgPing(nonce), nil)
			p.connReq.SendCommand(validatorcommand.NewMsgPing(nonce))
		}
	}
}

func (p *RemotePeer) listenCommand(connReq *ConnReq) {
	for {
		if connReq.isInactive() {
			// The connection is inactive, will exit listen handler
			break
		}
		log.Debugf("----------[RemotePeer]Will read command from conn[%d]: %s to %s", connReq.id, connReq.RemoteAddr, connReq.LocalAddr)
		_, command, _, err := validatorcommand.ReadMessage(connReq.conn, p.ValidatorVersion(), p.cfg.ChainParams.Net)
		if err != nil {
			log.Errorf("----------[RemotePeer]conn[%d]: Read message err: %v", connReq.id, err)
			return
		}
		log.Debugf("----------[RemotePeer]Received validator command [%v] from %d", command.Command(), connReq.id)

		connReq.setLastReceived()
		// TODO: handle the command
		go p.handleCommand(connReq, command)
	}
}

func (p *RemotePeer) handleCommand(connReq *ConnReq, command validatorcommand.Message) {
	log.Debugf("----------[RemotePeer]handleCommand command [%v]:", command.Command())
	switch cmd := command.(type) {
	case *validatorcommand.MsgGetInfo:
		log.Debugf("----------[RemotePeer]Receive MsgGetInfo command, will response MsgPeerInfo command")
		cmd.LogCommandInfo(log)
		// Handle command ping, it will response "PeerInfo" message
		cmdPeerInfo := validatorcommand.NewMsgPeerInfo(p.LocalValidatorId, cmd.Nonce)
		connReq.SendCommand(cmdPeerInfo)

	case *validatorcommand.MsgPeerInfo:
		log.Debugf("----------[RemotePeer]Receive MsgPeerInfo command")
		cmd.LogCommandInfo(log)

	case *validatorcommand.MsgPing:

		log.Debugf("----------[RemotePeer]Receive ping command, will response pong command")
		cmd.LogCommandInfo(log)
		// Handle command ping, it will response "pong" message
		cmdPong := validatorcommand.NewMsgPong(cmd.Nonce)
		connReq.SendCommand(cmdPong)

	default:
		log.Errorf("----------[RemotePeer]Not to handle command [%v] from %d", command.Command(), connReq.id)
	}
}
