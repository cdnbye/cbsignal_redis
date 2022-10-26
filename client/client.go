package client

import (
	"cbsignal/util/log"
	"github.com/bytedance/sonic"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"net"
	"sync"
	"time"
)

const (
	BLACKLIST_PEERS_LIMIT = 4
)

type Client struct {
	Conn net.Conn

	PeerId string //唯一标识

	Timestamp int64

	blacklist    []string
	blacklistPos int
	mu           sync.Mutex
}

type SignalCloseResp struct {
	Action     string      `json:"action"`
	FromPeerId string      `json:"from_peer_id,omitempty"`
	Data       interface{} `json:"data,omitempty"`
	Reason     string      `json:"reason,omitempty"`
}

type SignalVerResp struct {
	Action string `json:"action"`
	Ver    int    `json:"ver"`
}

func NewPeerClient(peerId string, conn net.Conn) *Client {
	return &Client{
		Conn:      conn,
		PeerId:    peerId,
		Timestamp: time.Now().Unix(),
		blacklist: make([]string, BLACKLIST_PEERS_LIMIT),
	}
}

func (c *Client) UpdateTs() {
	c.Timestamp = time.Now().Unix()
}

func (c *Client) IsExpired(now, limit int64) bool {
	return now-c.Timestamp > limit
}

func (c *Client) SendMsgClose(reason string) error {
	resp := SignalCloseResp{
		Action: "close",
		Reason: reason,
	}
	b, err := sonic.Marshal(resp)
	if err != nil {
		log.Error(err)
		return err
	}
	err, _ = c.SendMessage(b)
	return err
}

func (c *Client) SendMsgVersion(version int) error {
	resp := SignalVerResp{
		Action: "ver",
		Ver:    version,
	}
	b, err := sonic.Marshal(resp)
	if err != nil {
		log.Error(err)
		return err
	}
	err, _ = c.SendMessage(b)
	return err
}

func (c *Client) SendMessage(msg []byte) (error, bool) {
	return c.sendData(msg, false)
}

func (c *Client) SendBinaryData(data []byte) (error, bool) {
	return c.sendData(data, true)
}

func (c *Client) sendData(data []byte, binary bool) (error, bool) {
	var opCode ws.OpCode
	if binary {
		opCode = ws.OpBinary
	} else {
		opCode = ws.OpText
	}
	err := wsutil.WriteServerMessage(c.Conn, opCode, data)
	if err != nil {
		// handle error
		log.Info(err)
		return err, true
	}
	return nil, false
}

func (c *Client) Close() error {
	return c.Conn.Close()
}

func (c *Client) EnqueueBlacklistPeer(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.blacklist[c.blacklistPos] = id
	c.blacklistPos++
	if c.blacklistPos >= BLACKLIST_PEERS_LIMIT {
		c.blacklistPos = 0
	}
}

func (c *Client) HasBlacklistPeer(id string) bool {
	for _, v := range c.blacklist {
		if id == v {
			return true
		}
	}
	return false
}
