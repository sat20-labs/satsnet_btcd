// Copyright (c) 2013-2018 The btcsuite developers
// Copyright (c) 2016-2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package validatorpeer

import (
	"container/list"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validatorcommand"
	"github.com/sat20-labs/satsnet_btcd/wire"
)

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

const (

	// pingInterval is the interval of time to wait in between sending ping
	// messages.
	pingInterval = 10 * time.Second //2 * time.Minute

	peerInActiveInterval = 5 * pingInterval // 5 times pingInterval

	peerReconnectMaxTimes = 10
)

// ConnReq is the connection request to a network address. The
// connection will be retried on disconnection.
type ConnReq struct {
	// The following variables must only be used atomically.
	id uint64

	LocalAddr  net.Addr
	RemoteAddr net.Addr

	lastSend     int64 // last sent time
	lastReceived int64 // last time it was used
	connClose    int32 // set to 1, the connection has been closed

	CheckNonce uint64
	IsChecked  bool // if a peer connected , it should be checked the remote peer is valid validator peer

	conn    net.Conn
	version uint32
	btcnet  wire.BitcoinNet

	CmdsLock    sync.RWMutex
	pendingCmds *list.List // Output commands

	sendQueue     chan struct{}
	sendDoneQueue chan *list.Element
	quitQueue     chan struct{}
}

func (connReq *ConnReq) Start() {
	// Start connReq
	connReq.pendingCmds = list.New()

	connReq.sendQueue = make(chan struct{}, 1)          // nonblocking sync
	connReq.sendDoneQueue = make(chan *list.Element, 1) // nonblocking sync
	connReq.quitQueue = make(chan struct{}, 1)          // nonblocking sync

	go connReq.sendQueueHandler()
	return
}

func (connReq *ConnReq) close() {
	atomic.StoreInt32(&connReq.connClose, 1)
	connReq.conn.Close()
	// close the send queue
	connReq.quitQueue <- struct{}{}
}

func (connReq *ConnReq) setLastReceived() {
	atomic.StoreInt64(&connReq.lastReceived, time.Now().Unix())
}

func (connReq *ConnReq) isInactive() bool {
	isClosed := atomic.LoadInt32(&connReq.connClose)

	if isClosed != 0 {
		return true
	}

	lastReceivedTime := atomic.LoadInt64(&connReq.lastReceived)
	now := time.Now().Unix()

	intervalLastReceived := now - lastReceivedTime
	if intervalLastReceived > int64(peerInActiveInterval.Seconds()) {
		return true
	}
	return false
}

func (connReq *ConnReq) logConnInfo(desc string) {
	log.Debugf("——————————————————Conn info: %s——————————————————", desc)
	log.Debugf("Conn id: %d", connReq.id)
	log.Debugf("Conn LocalAddr: %s", connReq.LocalAddr)
	log.Debugf("Conn RemoteAddr: %s", connReq.RemoteAddr)

	lastReceivedTime := time.Unix(atomic.LoadInt64(&connReq.lastReceived), 0)
	log.Debugf("Conn Last received time: %s", lastReceivedTime.String())

	lastSendTime := time.Unix(atomic.LoadInt64(&connReq.lastSend), 0)
	log.Debugf("Conn Last sent time: %s", lastSendTime.String())

	if connReq.pendingCmds != nil {
		log.Debugf("Pending Commands Count: %d", connReq.pendingCmds.Len())
	}

	// for e := connReq.pendingCmds.Front(); e != nil; e = e.Next() {
	// 	command := e.Value.(*validatorcommand.Message)
	// 	log.Debugf("Pending Command: %s", command.Command())
	// }

	log.Debugf("——————————————————Conn info: %s End——————————————————", desc)
}

func (connReq *ConnReq) SendCommand(command validatorcommand.Message) error {
	if atomic.LoadInt32(&connReq.connClose) != 0 {
		// The connection was closed
		err := fmt.Errorf("connection %s closed", connReq.RemoteAddr)
		log.Debugf("**********SendCommand (%s) to (%s) failed: %v", command.Command(), connReq.RemoteAddr, err)
		return err
	}
	log.Debugf("----------[ConnReq]Add command [%s] to send queue", command.Command())
	connReq.CmdsLock.Lock()
	connReq.pendingCmds.PushBack(command)
	connReq.CmdsLock.Unlock()

	log.Debugf("----------[ConnReq]Send signal to send command")
	// Signal the send queue
	connReq.sendQueue <- struct{}{}

	return nil
}

func (connReq *ConnReq) GetNextItem() *list.Element {
	connReq.CmdsLock.Lock()
	item := connReq.pendingCmds.Front()
	connReq.CmdsLock.Unlock()

	return item
}

func (connReq *ConnReq) RemoveItem(item *list.Element) {
	connReq.CmdsLock.Lock()
	connReq.pendingCmds.Remove(item)
	connReq.CmdsLock.Unlock()
}

// outHandler handles all outgoing messages for the peer.  It must be run as a
// goroutine.  It uses a buffered channel to serialize output messages while
// allowing the sender to continue running asynchronously.
func (connReq *ConnReq) sendQueueHandler() {
out:
	for {
		log.Debugf("----------[ConnReq]Wait...")
		select {
		case <-connReq.sendQueue:
			log.Debugf("----------[ConnReq]Received signal to send command.")
			item := connReq.GetNextItem()
			if item == nil {
				// No command in queue, wait for the next signal
				log.Debugf("----------[ConnReq]No command to send.")
				continue
			}

			command := item.Value.(validatorcommand.Message)

			err := connReq.writeMessage(command)
			if err != nil {
				// TODO: writeMessage error, maybe the connect is disconnect
				connReq.close()
				break out
			}

			// At this point, the message was successfully sent, so
			// update the last send time, signal the sender of the
			// message that it has been sent (if requested), and
			// signal the send queue to the deliver the next queued
			// message.
			atomic.StoreInt64(&connReq.lastSend, time.Now().Unix())

			connReq.sendDoneQueue <- item

		case item := <-connReq.sendDoneQueue:
			// The command was sent successfully, will removed it from the
			// pending command list, and will signal the send queue
			connReq.RemoveItem(item)

			if connReq.pendingCmds.Len() > 0 {
				// command queue not empty, Signal the send queue
				connReq.sendQueue <- struct{}{}
			}

		case <-connReq.quitQueue:
			break out
		}
	}

	log.Tracef("----------[ConnReq] sendQueueHandler done for %d", connReq.id)
}

// writeMessage sends a bitcoin message to the peer with logging.
func (connReq *ConnReq) writeMessage(msg validatorcommand.Message) error {
	// Don't do anything if we're disconnecting.
	if atomic.LoadInt32(&connReq.connClose) != 0 {
		// The connection was closed
		err := fmt.Errorf("connection %s closed", connReq.RemoteAddr)
		log.Debugf("**********WriteMessage (%s) to (%s) failed: %v", msg.Command(), connReq.RemoteAddr, err)
		return err
	}

	// Write the message to the peer.
	n, err := validatorcommand.WriteMessageWithEncodingN(connReq.conn, msg,
		connReq.version, connReq.btcnet)
	if err != nil {
		log.Debugf("**********WriteMessage (%s) to (%s) failed: %v", msg.Command(), connReq.RemoteAddr, err)
		return err
	}

	log.Debugf("**********WriteMessage (%s) to (%s): %d bytes written", msg.Command(), connReq.RemoteAddr, n)

	return nil
}
