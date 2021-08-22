package client

import (
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	jsoniter "github.com/json-iterator/go"
	"github.com/lexkong/log"
	"net"
	"time"
)

const (
	MAX_NOT_FOUND_PEERS_LIMIT = 3
	MAX_REMOTE_PEERS_LIMIT = 3
)

var (
	json = jsoniter.ConfigCompatibleWithStandardLibrary
)

type Client struct {

	Conn            net.Conn

	PeerId          string              //唯一标识

	Timestamp int64

	NotFoundPeers     []string   // 记录没有找到的peer的队列

	RemotePeers []RemotePeer
}

type SignalCloseResp struct {
	Action string              `json:"action"`
	FromPeerId string          `json:"from_peer_id,omitempty"`
	Data interface{}           `json:"data,omitempty"`
	Reason string              `json:"reason,omitempty"`
}

type SignalVerResp struct {
	Action string              `json:"action"`
	Ver int                    `json:"ver"`
}

type RemotePeer struct {
	Id string
	Addr string
}

func NewPeerClient(peerId string, conn net.Conn) *Client {
	return &Client{
		Conn:        conn,
		PeerId:      peerId,
		Timestamp:   time.Now().Unix(),
		NotFoundPeers: make([]string, 0, MAX_NOT_FOUND_PEERS_LIMIT+1),
		RemotePeers: make([]RemotePeer, 0, MAX_REMOTE_PEERS_LIMIT+1),
	}
}

func (c *Client)UpdateTs() {
	//log.Warnf("%s UpdateTs", c.PeerId)
	c.Timestamp = time.Now().Unix()
}

func (c *Client)IsExpired(now, limit int64) bool {
	return now - c.Timestamp > limit
}

func (c *Client)SendMsgClose(reason string) error {
	resp := SignalCloseResp{
		Action: "close",
		Reason: reason,
	}
	b, err := json.Marshal(resp)
	if err != nil {
		log.Error("json.Marshal", err)
		return err
	}
	err, _ = c.SendMessage(b)
	return err
}

func (c *Client)SendMsgVersion(version int) error {
	resp := SignalVerResp{
		Action: "ver",
		Ver: version,
	}
	b, err := json.Marshal(resp)
	if err != nil {
		log.Error("json.Marshal", err)
		return err
	}
	err, _ = c.SendMessage(b)
	return err
}

func (c *Client)SendMessage(msg []byte) (error, bool) {
	return c.sendData(msg, false)
}

func (c *Client)SendBinaryData(data []byte) (error, bool) {
	return c.sendData(data, true)
}

func (c *Client)sendData(data []byte, binary bool) (error, bool) {
	var opCode ws.OpCode
	if binary {
		opCode = ws.OpBinary
	} else {
		opCode = ws.OpText
	}
	// 本地节点
	err := wsutil.WriteServerMessage(c.Conn, opCode, data)
	if err != nil {
		// handle error
		log.Warnf("WriteServerMessage " + err.Error())
		return err, true
	}
	return nil, false
}

func (c *Client)Close() error {
	return c.Conn.Close()
}

func (c *Client)EnqueueNotFoundOrRejectPeer(id string) {
	c.NotFoundPeers = append(c.NotFoundPeers, id)
	if len(c.NotFoundPeers) > MAX_NOT_FOUND_PEERS_LIMIT {
		c.NotFoundPeers = c.NotFoundPeers[1:(MAX_NOT_FOUND_PEERS_LIMIT+1)]
	}
}

func (c *Client)HasNotFoundOrRejectPeer(id string) bool {
	//for _, v := range c.NotFoundPeers {
	//	if id == v {
	//		return true
	//	}
	//}
	//return false
	for i := len(c.NotFoundPeers)-1; i >= 0; i-- {
		if id == c.NotFoundPeers[i] {
			return true
		}
	}
	return false
}

func (c *Client)EnqueueRemotePeer(id string, addr string) {
	c.RemotePeers = append(c.RemotePeers, RemotePeer{Id: id, Addr: addr})
	if len(c.RemotePeers) > MAX_REMOTE_PEERS_LIMIT {
		c.RemotePeers = c.RemotePeers[1:(MAX_REMOTE_PEERS_LIMIT+1)]
	}
}

func (c *Client)GetRemotePeer(id string) (string, bool) {
	//for _, v := range c.RemotePeers {
	//	if id == v.Id {
	//		return v.Addr, true
	//	}
	//}
	//return "", false
	for i := len(c.RemotePeers)-1; i >= 0; i-- {
		peer := c.RemotePeers[i]
		if id == peer.Id {
			return peer.Addr, true
		}
	}
	return "", false
}
