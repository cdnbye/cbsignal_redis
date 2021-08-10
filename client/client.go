package client

import (
	"encoding/json"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/lexkong/log"
	"net"
	"time"
)

const (

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	MAX_NOT_FOUND_PEERS_LIMIT = 5
)

type Client struct {

	Conn            net.Conn

	PeerId          string              //唯一标识

	Timestamp int64

	NotFoundPeers     []string   // 记录没有找到的peer的队列
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

func NewPeerClient(peerId string, conn net.Conn) *Client {
	return &Client{
		Conn:        conn,
		PeerId:      peerId,
		Timestamp:   time.Now().Unix(),
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
		c.NotFoundPeers = c.NotFoundPeers[1:len(c.NotFoundPeers)]
	}
}

func (c *Client)HasNotFoundOrRejectPeer(id string) bool {
	for _, v := range c.NotFoundPeers {
		if id == v {
			return true
		}
	}
	return false
}