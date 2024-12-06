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
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/epoch"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/generator"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validatorcommand"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validatorinfo"
	"github.com/sat20-labs/satsnet_btcd/wire"
)

// LocalPeerInterface defines callback function pointers to invoke by local
// peers. Include notify from peer to manager, and get info from manager.

// All notify functions should be start with "On". And all get functions should
// be start with "Get".
type LocalPeerInterface interface {
	// OnPeerConnected is invoked when a remote peer connects to the local peer .
	OnPeerConnected(net.Addr, *validatorinfo.ValidatorInfo)

	// GetAllValidators invoke when get all validators.
	GetAllValidators(uint64) []*validatorinfo.ValidatorInfo

	// GetLocalValidatorInfo invoke when local validator info.
	GetLocalValidatorInfo(uint64) *validatorinfo.ValidatorInfo

	// received broadcast for validators declare
	OnAllValidatorsDeclare([]validatorinfo.ValidatorInfo, net.Addr)

	// GetEpoch from local peer
	GetLocalEpoch(uint64) (*epoch.Epoch, *epoch.Epoch, error)

	// Req new epoch from remote peer
	ReqNewEpoch(uint64, uint32) (*epoch.Epoch, error)

	// OnNextEpoch from local peer to manager to change epoch to next
	OnNextEpoch(*epoch.HandOverEpoch)

	// OnUpdatedEpoch from current epoch is updated from remote peer
	OnUpdateEpoch(*epoch.Epoch)

	// GetGenerator from local peer
	GetGenerator(uint64) *generator.Generator

	// received broadcast for validators declare
	OnHandOverGenerator(generator.GeneratorHandOver, net.Addr)

	// Receives a generator response from remote with GetGenerator cmmand
	OnGeneratorResponse(*generator.Generator)

	// Received a confirm epoch command
	OnConfirmEpoch(*epoch.Epoch, net.Addr)

	// Received a Del epoch member command
	ConfirmDelEpochMember(*validatorcommand.MsgReqDelEpochMember, net.Addr) *epoch.DelEpochMember

	// Received a notify handover command
	OnNotifyHandover(uint64, net.Addr)

	// Received get vc state command
	GetVCState(uint64) (*validatorcommand.MsgGetVCState, error)

	// Received get vc list command
	GetVCList(uint64, int64, int64) (*validatorcommand.MsgGetVCList, error)

	// Received get vc block command
	GetVCBlock(uint64, uint32, chainhash.Hash) (*validatorcommand.MsgGetVCBlock, error)

	// Received a vc block command
	OnVCBlock(*validatorcommand.MsgVCBlock, net.Addr)
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

	ValidatorId uint64 // local validator id
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

// OnConnDisconnected to be called when a connection is disconnected.
//
// This function is safe for concurrent access.
func (p *LocalPeer) OnConnDisconnected(connReq *ConnReq) {
	log.Debugf("----------[LocalPeer]OnConnDisconnected conn[%d]: %s", connReq.id, connReq.RemoteAddr)

	p.connMapMtx.Lock()
	delete(p.connMap, connReq.id)
	p.connMapMtx.Unlock()

	p.logCurrentConn()

	log.Debugf("----------[LocalPeer]OnConnDisconnected End")
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

	log.Debugf("NewLocalPeervalidator ID: %d", p.cfg.ValidatorId)

	p.addrsList = make([]net.Addr, 0, len(addrs))
	p.addrsList = append(p.addrsList, addrs...)

	log.Debugf("NewLocalpeer IP:")
	for _, addr := range p.addrsList {
		log.Debugf("  %s", addr.String())
	}

	//p.addr = addrs[0].String() // Default to the first address

	// host, portStr, err := net.SplitHostPort(p.addr)
	// if err != nil {
	// 	return nil, err
	// }

	// port, err := strconv.ParseUint(portStr, 10, 16)
	// if err != nil {
	// 	return nil, err
	// }

	// if cfg.HostToNetAddress != nil {
	// 	na, err := cfg.HostToNetAddress(host, uint16(port), 0)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	p.na = na
	// } else {
	// 	// If host is an onion hidden service or a hostname, it is
	// 	// likely that a nil-pointer-dereference will occur. The caller
	// 	// should set HostToNetAddress if connecting to these.
	// 	p.na = wire.NetAddressV2FromBytes(
	// 		time.Now(), 0, net.ParseIP(host), uint16(port),
	// 	)
	// }

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
		if p.addr == "" {
			// Set the default addr is first address can be used
			//p.addr = listenAddr
			p.setDefaultAddr(addr)
		}
	}

	return listeners, nil
}

func (p *LocalPeer) setDefaultAddr(addr net.Addr) error {
	p.addr = addr.String() // Default to the first address

	host, portStr, err := net.SplitHostPort(p.addr)
	if err != nil {
		return err
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return err
	}

	if p.cfg.HostToNetAddress != nil {
		na, err := p.cfg.HostToNetAddress(host, uint16(port), 0)
		if err != nil {
			return err
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
	return nil
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
			Listener:   p,
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
			//p.closeConn(connReq)
			p.connMapMtx.Lock()
			delete(p.connMap, connReq.id)
			p.connMapMtx.Unlock()
		} else {
			// The remote peer is new peer connected
			p.SendGetInfoCommand(newConnReq)
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
		// Handle the command
		go p.handleCommand(connReq, command)
	}
}

// handleCommand processes different types of validator commands received from a connection request.
// It handles various commands such as MsgGetInfo, MsgPeerInfo, MsgPing, MsgGetValidators, and others.
// For each command, it performs specific actions like sending appropriate responses or notifying
// the validator manager. Logs are generated for each command received and processed. If a command
// is not recognized, an error log is generated.
func (p *LocalPeer) handleCommand(connReq *ConnReq, command validatorcommand.Message) {
	log.Debugf("----------[LocalPeer]handleCommand command [%v]:", command.Command())
	switch cmd := command.(type) {
	case *validatorcommand.MsgGetInfo:
		log.Debugf("----------[LocalPeer]Receive MsgGetInfo command, will response MsgPeerInfo command")
		//cmd.LogCommandInfo(log)

		validatorInfo := p.cfg.LocalValidator.GetLocalValidatorInfo(0)
		// Handle command ping, it will response "PeerInfo" message
		cmdPeerInfo := validatorcommand.NewMsgPeerInfo(validatorInfo)
		connReq.SendCommand(cmdPeerInfo)

	case *validatorcommand.MsgPeerInfo:
		log.Debugf("----------[LocalPeer]Receive MsgPeerInfo command")
		cmd.LogCommandInfo(log)
		// Notify the validator that we have a new connection， And received peer info from the remote peer
		p.HandleRemotePeerInfoConfirmed(cmd, connReq)

	case *validatorcommand.MsgPing:

		log.Debugf("----------[LocalPeer]Receive ping command, will response pong command")
		// cmd.LogCommandInfo(log)
		// Handle command ping, it will response "pong" message
		cmdPong := validatorcommand.NewMsgPong(cmd.Nonce)
		connReq.SendCommand(cmdPong)

	case *validatorcommand.MsgGetValidators:
		log.Debugf("----------[LocalPeer]Receive GetValidators command, will response Validators command")
		// cmd.LogCommandInfo(log)
		p.HandleGetValidators(cmd, connReq)

	case *validatorcommand.MsgValidators:
		// The command should be boardcast command to all peers when the validators are changed
		log.Debugf("----------[LocalPeer]Receive Validators command, will notify validatorManager for sync validators")
		// cmd.LogCommandInfo(log)

	case *validatorcommand.MsgGetEpoch:
		log.Debugf("----------[LocalPeer]Receive MsgGetEpoch command, will response local epoch list to remote peer with MsgEpoch command")
		//cmd.LogCommandInfo(log)
		p.HandleGetEpoch(cmd, connReq)

	case *validatorcommand.MsgReqEpoch:
		log.Debugf("----------[LocalPeer]Receive MsgReqEpoch command, will response new epoch info to remote peer with MsgNewEpoch command")
		//cmd.LogCommandInfo(log)
		p.HandleReqEpoch(cmd, connReq)

	case *validatorcommand.MsgNextEpoch:
		log.Debugf("----------[LocalPeer]Receive MsgNextEpoch command, will notify validatorManager for change to next epoch. ")
		//cmd.LogCommandInfo(log)
		p.HandleNextEpoch(cmd, connReq)

	case *validatorcommand.MsgUpdateEpoch:
		log.Debugf("----------[LocalPeer]Receive MsgNextEpoch command, will notify validatorManager for change to next epoch. ")
		//cmd.LogCommandInfo(log)
		p.HandleUpdateEpoch(cmd, connReq)

	case *validatorcommand.MsgGetGenerator:
		log.Debugf("----------[LocalPeer]Receive MsgGetGetGenerator command, will response local generator list to remote peer with MsgGenerator command")
		//cmd.LogCommandInfo(log)
		p.HandleGetGenerator(cmd, connReq)

	case *validatorcommand.MsgHandOver:
		log.Debugf("----------[LocalPeer]Receive MsgHandOver command, will notify validatorManager for generator handovered. ")
		//cmd.LogCommandInfo(log)
		p.HandleHandOverGenerator(cmd, connReq)

	case *validatorcommand.MsgGenerator:
		log.Debugf("----------[LocalPeer]Receive MsgGenerator command, will notify validatorManager for generator updated. ")
		//cmd.LogCommandInfo(log)
		p.HandleGeneratorResponse(cmd, connReq)

	case *validatorcommand.MsgConfirmEpoch:
		log.Debugf("----------[LocalPeer]Receive MsgConfirmEpoch command, will notify validatorManager for the confirm epoch. ")
		cmd.LogCommandInfo(log)
		p.HandleConfirmEpoch(cmd, connReq)

	case *validatorcommand.MsgReqDelEpochMember:
		log.Debugf("----------[LocalPeer]Receive MsgReqDelEpochMember command, will notify validatorManager for delete epoch member. ")
		cmd.LogCommandInfo(log)
		p.handleDelEpochMember(cmd, connReq)

	case *validatorcommand.MsgNotifyHandover:
		log.Debugf("----------[LocalPeer]Receive MsgReqDelEpochMember command, will notify validatorManager for delete epoch member. ")
		cmd.LogCommandInfo(log)
		p.handleNotifyHandover(cmd, connReq)

	case *validatorcommand.MsgGetVCState:
		log.Debugf("----------[LocalPeer]Receive MsgGetVCState command, will response local vc state to remote peer with MsgVCState command")
		//cmd.LogCommandInfo(log)
		p.HandleGetVCState(cmd, connReq)

	case *validatorcommand.MsgGetVCList:
		log.Debugf("----------[LocalPeer]Receive MsgGetVCList command, will response local vc list to remote peer with MsgVCList command")
		//cmd.LogCommandInfo(log)
		p.HandleGetVCList(cmd, connReq)

	case *validatorcommand.MsgGetVCBlock:
		log.Debugf("----------[LocalPeer]Receive MsgGetVCBlock command, will response local vs block to remote peer with MsgVCBlock command")
		//cmd.LogCommandInfo(log)
		p.HandleGetVCBlock(cmd, connReq)

	case *validatorcommand.MsgVCBlock:
		log.Debugf("----------[LocalPeer]Receive MsgVCBlock broadcast command, will notify validatormanager to process vc block. ")
		//cmd.LogCommandInfo(log)
		p.HandleVCBlock(cmd, connReq)

	default:
		cmd.LogCommandInfo(log)
		log.Errorf("----------[LocalPeer]Not to handle command [%v] from %d", command.Command(), connReq.id)
	}
}

// trickleHandler handle inactive conn with timer.  It must be run as a go routine.
func (p *LocalPeer) trickleHandler() {
	trickleTicker := time.NewTicker(p.cfg.TrickleInterval)
	defer trickleTicker.Stop()

	// for {
	// 	select {
	// 	case <-trickleTicker.C:
	// 		p.checkInactiveConn()
	// 	}
	// }
	go func() {
		for range trickleTicker.C { // 遍历 channel，直到 channel 关闭
			p.checkInactiveConn()
		}
	}()
}

func (p *LocalPeer) checkInactiveConn() {
	log.Debugf("----------[LocalPeer]checkInactiveConn")
	for _, connReq := range p.connMap {
		if connReq.isInactive() {
			// The connection is inactive, will close it, and remove it from connMap
			log.Debugf("----------[LocalPeer]The connection [%s] is inactive, will close it, and remove it from connMap", connReq.RemoteAddr)
			connReq.Close()
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

	connReq.Close()
	p.connMapMtx.Lock()
	delete(p.connMap, connReq.id)
	p.connMapMtx.Unlock()

	p.logCurrentConn()

	log.Debugf("----------[LocalPeer]checkInactiveConn End")
}

func (p *LocalPeer) lookupConn(addr net.Addr) *ConnReq {
	log.Debugf("----------[LocalPeer]lookupConn")
	addrHost := validatorinfo.GetAddrHost(addr)
	p.connMapMtx.RLock()
	defer p.connMapMtx.RUnlock()

	for _, connReq := range p.connMap {
		remoteHost := validatorinfo.GetAddrHost(connReq.RemoteAddr)
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

func (p *LocalPeer) SendGetInfoCommand(newConnReq *ConnReq) {

	validatorInfo := p.cfg.LocalValidator.GetLocalValidatorInfo(0)
	getInfoCmd := validatorcommand.NewMsgGetInfo(validatorInfo)
	//log.Debugf("----------[LocalPeer]Will Send GetInfoCommand")
	//getInfoCmd.LogCommandInfo(log)
	newConnReq.SendCommand(getInfoCmd)
}

func (p *LocalPeer) HandleRemotePeerInfoConfirmed(peerInfo *validatorcommand.MsgPeerInfo, connReq *ConnReq) {
	// 	First check the remote validator is valid, then notify the validator

	if CheckValidatorID(peerInfo.ValidatorId) == false {
		log.Errorf("----------[LocalPeer]The remote peer is not valid")
		return
	}

	log.Debugf("----------[LocalPeer]The remote peer is comfirmed, will notify validator conn[%d]: %s", connReq.id, connReq.RemoteAddr)
	if p.cfg.LocalValidator != nil {
		validatorInfo := &validatorinfo.ValidatorInfo{
			ValidatorId: peerInfo.ValidatorId,
			PublicKey:   peerInfo.PublicKey,
			Host:        peerInfo.Host,
			CreateTime:  peerInfo.CreateTime,
		}
		p.cfg.LocalValidator.OnPeerConnected(connReq.RemoteAddr, validatorInfo)
	}
}

func (p *LocalPeer) HandleGetValidators(getValidatorsCmd *validatorcommand.MsgGetValidators, connReq *ConnReq) {
	validators := p.cfg.LocalValidator.GetAllValidators(getValidatorsCmd.ValidatorId)
	if validators == nil {
		validators = []*validatorinfo.ValidatorInfo{}
	}
	respValidators := validatorcommand.NewMsgValidators(validators)

	//log.Debugf("----------[LocalPeer]Send MsgValidators command")
	//respValidators.LogCommandInfo(log)
	connReq.SendCommand(respValidators)
}

func (p *LocalPeer) HandleAllValidatorsDeclare(validatorsCmd *validatorcommand.MsgValidators, connReq *ConnReq) {
	// 	First check the remote validator is valid, then notify the validator

	log.Debugf("----------[LocalPeer]The remote peer is comfirmed, will notify validator conn[%d]: %s", connReq.id, connReq.RemoteAddr)
	if p.cfg.LocalValidator != nil {
		p.cfg.LocalValidator.OnAllValidatorsDeclare(validatorsCmd.Validators, connReq.RemoteAddr)
	}
}

func (p *LocalPeer) HandleGetEpoch(getEpochCmd *validatorcommand.MsgGetEpoch, connReq *ConnReq) {
	log.Debugf("----------[LocalPeer]HandleGetEpoch...")
	currentEpoch, nextEpoch, err := p.cfg.LocalValidator.GetLocalEpoch(getEpochCmd.ValidatorId)
	if err != nil {
		//validators = []*epoch.EpochItem{}
		log.Errorf("----------[LocalPeer]GetLocalEpoch error: %s", err.Error())
		return
	}
	epochCommand := validatorcommand.NewMsgEpoch(currentEpoch, nextEpoch)

	//log.Debugf("----------[LocalPeer]Send MsgValidators command")
	epochCommand.LogCommandInfo(log)
	connReq.SendCommand(epochCommand)
}

func (p *LocalPeer) HandleReqEpoch(getEpochCmd *validatorcommand.MsgReqEpoch, connReq *ConnReq) {
	newEpoch, err := p.cfg.LocalValidator.ReqNewEpoch(getEpochCmd.ValidatorId, getEpochCmd.EpochIndex)
	if err != nil {
		log.Errorf("----------[LocalPeer]ReqNewEpoch error: %s", err.Error())
		return
	} else {
		log.Debugf("----------[LocalPeer]ReqNewEpoch success: %v", newEpoch)
	}
	respNewEpoch := validatorcommand.NewMsgNewEpoch(p.cfg.ValidatorId, newEpoch)

	log.Debugf("----------[LocalPeer]Send MsgNewEpoch command")
	respNewEpoch.LogCommandInfo(log)
	connReq.SendCommand(respNewEpoch)
}

func (p *LocalPeer) HandleNextEpoch(nextEpochCmd *validatorcommand.MsgNextEpoch, connReq *ConnReq) {
	p.cfg.LocalValidator.OnNextEpoch(&nextEpochCmd.HandoverEpoch)
}

func (p *LocalPeer) HandleUpdateEpoch(updateEpochCmd *validatorcommand.MsgUpdateEpoch, connReq *ConnReq) {
	p.cfg.LocalValidator.OnUpdateEpoch(updateEpochCmd.CurrentEpoch)
}

func (p *LocalPeer) HandleGetGenerator(getGeneratorCmd *validatorcommand.MsgGetGenerator, connReq *ConnReq) {
	generator := p.cfg.LocalValidator.GetGenerator(getGeneratorCmd.ValidatorId)
	respGeneratorCmd := validatorcommand.NewMsgGenerator(generator)

	log.Debugf("----------[LocalPeer]Send MsgGenerator command")
	respGeneratorCmd.LogCommandInfo(log)
	connReq.SendCommand(respGeneratorCmd)
}

func (p *LocalPeer) HandleHandOverGenerator(handOverGeneratorCmd *validatorcommand.MsgHandOver, connReq *ConnReq) {
	p.cfg.LocalValidator.OnHandOverGenerator(handOverGeneratorCmd.HandOverInfo, connReq.RemoteAddr)
}

func (p *LocalPeer) HandleGeneratorResponse(generatorCmd *validatorcommand.MsgGenerator, connReq *ConnReq) {
	// 	First check the remote validator is valid, then notify the validator
	log.Debugf("----------[LocalPeer]The generator info is response from  validatorvalidator ID: %s", connReq.RemoteAddr.String())

	// if generator.IsValid(generatorCmd.GeneratorInfo) == false {
	// 	return
	// }

	p.cfg.LocalValidator.OnGeneratorResponse(&generatorCmd.GeneratorInfo)
}

func (p *LocalPeer) HandleConfirmEpoch(confirmEpochCmd *validatorcommand.MsgConfirmEpoch, connReq *ConnReq) {
	confirmEpoch := &epoch.Epoch{
		EpochIndex:      confirmEpochCmd.EpochIndex,
		ItemList:        make([]*epoch.EpochItem, 0),
		CreateHeight:    confirmEpochCmd.CreateHeight,
		CreateTime:      confirmEpochCmd.CreateTime,
		CurGeneratorPos: epoch.Pos_Epoch_NotStarted,
	}

	for _, epochItem := range confirmEpochCmd.ItemList {
		validatorItem := &epoch.EpochItem{
			ValidatorId: epochItem.ValidatorId,
			Host:        epochItem.Host,
			PublicKey:   epochItem.PublicKey,
			Index:       epochItem.Index,
		}
		confirmEpoch.ItemList = append(confirmEpoch.ItemList, validatorItem)
	}

	p.cfg.LocalValidator.OnConfirmEpoch(confirmEpoch, connReq.RemoteAddr)
}

// handleDelEpochMember is called when a peer sends a request to delete an epoch member from the epoch list.
// It will notify the validator to confirm the epoch member deletion.
func (p *LocalPeer) handleDelEpochMember(delEpochMember *validatorcommand.MsgReqDelEpochMember, connReq *ConnReq) {
	cdmDelEpochMember := p.cfg.LocalValidator.ConfirmDelEpochMember(delEpochMember, connReq.RemoteAddr)

	if cdmDelEpochMember != nil {
		// Response the result for Del epoch memeber
		cfmDelEpochMemCmd := validatorcommand.NewMsgConfirmDelEpoch(cdmDelEpochMember)
		connReq.SendCommand(cfmDelEpochMemCmd)
	}
}

func (p *LocalPeer) handleNotifyHandover(notifyHandover *validatorcommand.MsgNotifyHandover, connReq *ConnReq) {
	p.cfg.LocalValidator.OnNotifyHandover(notifyHandover.ValidatorId, connReq.RemoteAddr)

}

func (p *LocalPeer) HandleGetVCState(getVCState *validatorcommand.MsgGetVCState, connReq *ConnReq) {
	vcStateCmd, err := p.cfg.LocalValidator.GetVCState(getVCState.ValidatorId)
	if err != nil {
		log.Errorf("----------[LocalPeer]GetVCState error: %s", err.Error())
		return
	}
	log.Debugf("----------[LocalPeer]Send MsgVCState command")
	vcStateCmd.LogCommandInfo(log)
	connReq.SendCommand(vcStateCmd)
}

func (p *LocalPeer) HandleGetVCList(getVCList *validatorcommand.MsgGetVCList, connReq *ConnReq) {
	vcListCmd, err := p.cfg.LocalValidator.GetVCList(getVCList.ValidatorId, getVCList.Start, getVCList.End)
	if err != nil {
		log.Errorf("----------[LocalPeer]GetVCList error: %s", err.Error())
		return
	}
	log.Debugf("----------[LocalPeer]Send MsgVCList command")
	vcListCmd.LogCommandInfo(log)
	connReq.SendCommand(vcListCmd)
}

func (p *LocalPeer) HandleGetVCBlock(getVCBlock *validatorcommand.MsgGetVCBlock, connReq *ConnReq) {
	vcBlockCmd, err := p.cfg.LocalValidator.GetVCBlock(getVCBlock.ValidatorId, getVCBlock.BlockType, getVCBlock.BlockHash)
	if err != nil {
		log.Errorf("----------[LocalPeer]GetVCBlock error: %s", err.Error())
		return
	}
	log.Debugf("----------[LocalPeer]Send MsgVCBlock command")
	vcBlockCmd.LogCommandInfo(log)
	connReq.SendCommand(vcBlockCmd)
}

func (p *LocalPeer) HandleVCBlock(vcBlock *validatorcommand.MsgVCBlock, connReq *ConnReq) {
	p.cfg.LocalValidator.OnVCBlock(vcBlock, connReq.RemoteAddr)
}
