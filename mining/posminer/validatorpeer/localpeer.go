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

	connReqIndex uint64              // Index of the current connection request, From 1
	connMap      map[uint64]*ConnReq // map of all connections, key is conn id, value is conn req
	connMapMtx   sync.RWMutex
}

// String returns the peer's address and directionality as a human-readable
// string.
//
// This function is safe for concurrent access.
func (p *LocalPeer) String() string {
	return fmt.Sprintf("LocalPeer")
}

// StatsSnapshot returns a snapshot of the current peer flags and statistics.
//
// This function is safe for concurrent access.
func (p *LocalPeer) StatsSnapshot() *StatsSnap {
	// Get a copy of all relevant flags and stats.
	statsSnap := &StatsSnap{}
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

// UserAgent returns the user agent of the remote peer.
//
// This function is safe for concurrent access.
func (p *LocalPeer) UserAgent() string {
	p.flagsMtx.Lock()
	userAgent := p.userAgent
	p.flagsMtx.Unlock()

	return userAgent
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

// ProtocolVersion returns the negotiated peer protocol version.
//
// This function is safe for concurrent access.
func (p *LocalPeer) ValidatorVersion() uint32 {
	p.flagsMtx.Lock()
	validatorVersion := p.validatorVersion
	p.flagsMtx.Unlock()

	return validatorVersion
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

// newPeerBase returns a new base bitcoin peer based on the inbound flag.  This
// is used by the NewInboundPeer and NewOutboundPeer functions to perform base
// setup needed by both types of peers.
func newLocalPeerBase(origCfg *LocalPeerConfig, inbound bool) *LocalPeer {
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
	p := newLocalPeerBase(cfg, false)

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

	go p.trickleHandler()
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
			id:         atomic.LoadUint64(&p.connReqIndex),
			LocalAddr:  conn.LocalAddr(),
			RemoteAddr: conn.RemoteAddr(),
			conn:       conn,
			CmdsLock:   sync.RWMutex{},
			btcnet:     p.cfg.ChainParams.Net,
			version:    p.ValidatorVersion(),
		}
		newConnReq.setLastReceived()
		newConnReq.logConnInfo("newConnReq")

		newConnReq.Start()

		atomic.AddUint64(&p.connReqIndex, 1)

		// Check the remote peer is in the connMap, if yes, it's reconnected, will remove the old connReq
		connReq := p.lookupConn(newConnReq.RemoteAddr)
		if connReq != nil {
			log.Debugf("----------[LocalPeer]The remote peer is reconnected, will remove the old connReq [%d: %s]", connReq.id, connReq.RemoteAddr)
			// The remote peer has been connected to the local peer, the new connReq is reconnected, it will remove the old connReq
			p.closeConn(connReq)
		} else {
			// The remote peer is new peer connected
			// Notify the validator that we have a new connection
			log.Debugf("----------[LocalPeer]The remote peer is new connected, will notify validator conn[%d]: %s", newConnReq.id, newConnReq.RemoteAddr)
			if p.cfg.LocalValidator != nil {
				p.cfg.LocalValidator.OnPeerConnected(newConnReq.RemoteAddr)
			}
		}

		p.addConn(newConnReq)

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

		connReq.setLastReceived()
		// TODO: handle the command
		go p.handleCommand(connReq, command)
	}
}

func (p *LocalPeer) handleCommand(connReq *ConnReq, command validatorcommand.Message) {
	log.Debugf("----------[LocalPeer]handleCommand command [%v]:", command.Command())
	switch cmd := command.(type) {
	case *validatorcommand.MsgVersion:

	case *validatorcommand.MsgVerAck:

	case *validatorcommand.MsgPing:

		log.Debugf("----------[LocalPeer]Receive ping command, will response pong command")
		// Handle command ping, it will response "pong" message
		cmdPong := validatorcommand.NewMsgPong(cmd.Nonce)
		connReq.SendCommand(cmdPong)

	default:
		log.Errorf("----------[LocalPeer]Not to handle command [%v] from %d", command.Command(), connReq.id)
	}
}

// trickleHandler handle inactive conn with timer.  It must be run as a go routine.
func (p *LocalPeer) trickleHandler() {
	trickleTicker := time.NewTicker(p.cfg.TrickleInterval)
	defer trickleTicker.Stop()

	for {
		select {
		case <-trickleTicker.C:
			p.checkInactiveConn()
		}
	}
}

func (p *LocalPeer) checkInactiveConn() {
	log.Debugf("----------[LocalPeer]checkInactiveConn")
	for _, connReq := range p.connMap {
		if connReq.isInactive() {
			// The connection is inactive, will close it, and remove it from connMap
			log.Debugf("----------[LocalPeer]The connection [%s] is inactive, will close it, and remove it from connMap", connReq.RemoteAddr)
			connReq.close()
			delete(p.connMap, connReq.id)
		}
	}

	p.logCurrentConn()

	log.Debugf("----------[LocalPeer]checkInactiveConn End")
}

func (p *LocalPeer) addConn(connReq *ConnReq) {
	log.Debugf("----------[LocalPeer]addConn conn[%d]: %s", connReq.id, connReq.RemoteAddr)
	p.connMapMtx.Lock()
	p.connMap[connReq.id] = connReq
	p.connMapMtx.Unlock()

	p.logCurrentConn()

}

func (p *LocalPeer) closeConn(connReq *ConnReq) {
	log.Debugf("----------[LocalPeer]closeConn")

	connReq.close()
	p.connMapMtx.Lock()
	delete(p.connMap, connReq.id)
	p.connMapMtx.Unlock()

	p.logCurrentConn()

	log.Debugf("----------[LocalPeer]checkInactiveConn End")
}

func (p *LocalPeer) lookupConn(addr net.Addr) *ConnReq {
	log.Debugf("----------[LocalPeer]lookupConn")
	addrHost := addr.(*net.TCPAddr).IP
	p.connMapMtx.RLock()
	defer p.connMapMtx.RUnlock()

	for _, connReq := range p.connMap {
		remoteHost := connReq.RemoteAddr.(*net.TCPAddr).IP
		if addrHost.Equal(remoteHost) {
			// is same host,
			return connReq
		}
	}

	return nil
}

func (p *LocalPeer) logCurrentConn() {
	p.connMapMtx.RLock()
	defer p.connMapMtx.RUnlock()

	count := len(p.connMap)
	log.Debugf("----------[LocalPeer]logCurrentConn: %d conn", count)
	for id, connReq := range p.connMap {
		title := fmt.Sprintf("Conn id: %d", id)
		connReq.logConnInfo(title)
	}

}
