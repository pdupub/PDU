// Copyright 2019 The PDU Authors
// This file is part of the PDU library.
//
// The PDU library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The PDU library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the PDU library. If not, see <http://www.gnu.org/licenses/>.

package node

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strings"

	"github.com/pdupub/go-pdu/common"
	"github.com/pdupub/go-pdu/common/log"
	"github.com/pdupub/go-pdu/core"
	"github.com/pdupub/go-pdu/db"
	"github.com/pdupub/go-pdu/galaxy"
	"github.com/pdupub/go-pdu/peer"
	"golang.org/x/net/websocket"
)

const (
	localIPAddress = "127.0.0.1"
)

func (n Node) handleMessages(ws *websocket.Conn, w galaxy.Wave) (*core.Message, common.Hash, error) {
	var msg core.Message
	wm := w.(*galaxy.WaveMessages)
	for _, wmsg := range wm.Msgs {
		if err := json.Unmarshal(wmsg, &msg); err != nil {
			return nil, wm.WaveID, err
		}
	}
	// save msg (universe & udb)
	if err := n.saveMsg(&msg); err != nil {
		return nil, wm.WaveID, err
	} else if err := n.broadcastMsg(&msg); err != nil {
		return nil, wm.WaveID, err
	}
	return &msg, wm.WaveID, nil
}

func (n Node) handlePing(ws *websocket.Conn, w galaxy.Wave) (common.Hash, error) {
	wm := w.(*galaxy.WavePing)
	p := peer.Peer{Conn: ws}
	return wm.WaveID, p.SendPong(wm.WaveID)
}

func (n Node) handlePong(ws *websocket.Conn, w galaxy.Wave) (common.Hash, error) {
	wm := w.(*galaxy.WavePong)
	return wm.WaveID, nil
}

func (n *Node) handleRoots(ws *websocket.Conn, w galaxy.Wave) (common.Hash, error) {
	wm := w.(*galaxy.WaveRoots)
	user0 := wm.Users[0]
	user1 := wm.Users[1]
	log.Info("user0", common.Hash2String(user0.ID()))
	log.Info("user1", common.Hash2String(user1.ID()))
	// update init step
	var err error
	n.initStep = db.StepRootsSaved
	n.universe, err = core.NewUniverse(user0, user1)
	if err != nil {
		return wm.WaveID, err
	}
	if err := db.SaveRootUsers(n.udb, wm.Users[:]); err != nil {
		return wm.WaveID, err
	}
	return wm.WaveID, nil
}

func (n *Node) handlePeers(ws *websocket.Conn, w galaxy.Wave) (common.Hash, error) {
	wm := w.(*galaxy.WavePeers)
	for _, peerBytes := range wm.Peers {
		var targetPeer peer.Peer
		err := json.Unmarshal(peerBytes, &targetPeer)
		if err != nil {
			return wm.WaveID, err
		}

		if err := n.AddPeer(&targetPeer); err != nil {
			if err != errPeerAlreadyExist {
				return wm.WaveID, err
			}
		}
		log.Debug("Peer address", targetPeer.Address())
	}
	return wm.WaveID, nil
}

func (n *Node) handleErr(ws *websocket.Conn, w galaxy.Wave) (common.Hash, error) {
	wm := w.(*galaxy.WaveErr)
	log.Error("Peer handler err", wm.Err, "by wave", wm.WaveID)
	return wm.WaveID, nil
}

func (n Node) handleQuestion(ws *websocket.Conn, w galaxy.Wave) (common.Hash, error) {
	wm := w.(*galaxy.WaveQuestion)
	p := peer.Peer{Conn: ws}
	//log.Debug("Received question", wm.Cmd)
	switch wm.Cmd {
	case galaxy.CmdRoots:
		user0, user1, err := db.GetRootUsers(n.udb)
		if err != nil {
			return wm.WaveID, err
		}

		if err = p.SendRoots(wm.WaveID, user0, user1); err != nil {
			return wm.WaveID, err
		}
	case galaxy.CmdPeers:
		if err := p.SendPeers(wm.WaveID, n.peers, n.localPeer()); err != nil {
			return wm.WaveID, err
		}

		// add request peer to node.peers
		var remotePeer peer.Peer
		if err := json.Unmarshal(wm.Args[0], &remotePeer); err != nil {
			return wm.WaveID, err
		}
		// get remote ip address
		remoteAddr := strings.Split(ws.Request().RemoteAddr, ":")
		remotePeer.IP = remoteAddr[0]
		if err := n.AddPeer(&remotePeer); err != nil {
			return wm.WaveID, err
		}

	case galaxy.CmdMessages:
		var order, count *big.Int
		var err error
		var msgs []*core.Message
		msgID := common.Bytes2Hash(wm.Args[0])

		if msgID != common.Bytes2Hash([]byte{}) {
			order, count, err = db.GetOrderCntByMsg(n.udb, msgID)
			if err != nil {
				return wm.WaveID, err
			}
			order = order.Add(order, big.NewInt(1))
		} else {
			order = big.NewInt(0)
			count, err = db.GetMsgCount(n.udb)
			if err != nil {
				return wm.WaveID, err
			}
		}

		if order != nil && count != nil && count.Uint64()-order.Uint64() > peer.MaxMsgCountPerWave {
			//log.Debug("Send msg from order", order, "size", peer.MaxMsgCountPerWave)
			msgs = db.GetMsgByOrder(n.udb, order, peer.MaxMsgCountPerWave)
		}
		if err = p.SendMsgs(wm.WaveID, msgs); err != nil {
			return wm.WaveID, err
		}
	}
	return wm.WaveID, nil
}

func (n Node) handleWave(ws *websocket.Conn, w galaxy.Wave) (waveID common.Hash, err error) {
	switch w.Command() {
	case galaxy.CmdMessages:
		var msg *core.Message
		if msg, waveID, err = n.handleMessages(ws, w); err != nil {
			return waveID, err
		} else {
			log.Info("Received message", common.Hash2String(msg.ID()))
		}
	case galaxy.CmdQuestion:
		if waveID, err = n.handleQuestion(ws, w); err != nil {
			return waveID, err
		}
	case galaxy.CmdPing:
		if waveID, err = n.handlePing(ws, w); err != nil {
			return waveID, err
		}
	case galaxy.CmdPong:
		if waveID, err = n.handlePong(ws, w); err != nil {
			return waveID, err
		}
	case galaxy.CmdRoots:
		if waveID, err = n.handleRoots(ws, w); err != nil {
			return waveID, err
		}
	case galaxy.CmdPeers:
		if waveID, err = n.handlePeers(ws, w); err != nil {
			return waveID, err
		}
	case galaxy.CmdErr:
		if waveID, err = n.handleErr(ws, w); err != nil {
			return waveID, err
		}
	default:
		return common.Hash{}, fmt.Errorf("unhandled command [%s]", w.Command())
	}
	return waveID, nil
}

func (n Node) serveReceiveWave(r io.Reader, chanWave chan<- galaxy.Wave) {
	for {
		w, err := galaxy.ReceiveWave(r)
		if err != nil {
			log.Error("Serve receive wave fail", err)
			if err == io.EOF {
				break
			}
			continue
		}
		chanWave <- w
	}
}

func (n Node) wsHandler(ws *websocket.Conn) {
	chanWave := make(chan galaxy.Wave)
	p := peer.Peer{Conn: ws}
	go n.serveReceiveWave(ws, chanWave)
	for {
		waveID, err := n.handleWave(ws, <-chanWave)
		if err != nil {
			log.Error("Socket Handler", err)
			p.SendErr(waveID, err)
		}
	}
}
