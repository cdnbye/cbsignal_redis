package client

import (
	"cbsignal/util/log"
	"errors"
	"fmt"
	"github.com/bytedance/sonic"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"net"
	"sync"
	"time"
)

const (
	POLLING_QUEUE_SIZE   = 30
	WS_EXPIRE_LIMIT      = 11 * 60
	POLLING_EXPIRE_LIMIT = 3 * 60
)

//Initialing pool
var clientPool = sync.Pool{
	New: func() interface{} {
		return &Client{}
	},
}

type Client struct {
	Conn      net.Conn
	PeerId    string //唯一标识
	Timestamp int64
	// polling
	IsPolling bool
	MsgQueue  chan []byte
}

type SignalCloseResp struct {
	Action     string `json:"action"`
	FromPeerId string `json:"from_peer_id,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type SignalVerResp struct {
	Action string `json:"action,omitempty"`
	Ver    int    `json:"ver"`
}

func NewWebsocketPeerClient(peerId string, conn net.Conn) *Client {
	c := clientPool.Get().(*Client)
	c.Conn = conn
	c.PeerId = peerId
	c.Timestamp = time.Now().Unix()
	c.IsPolling = false
	c.MsgQueue = nil
	return c
}

func NewPollingPeerClient(peerId string) *Client {
	c := clientPool.Get().(*Client)
	c.Conn = nil
	c.PeerId = peerId
	c.IsPolling = true
	c.MsgQueue = make(chan []byte, POLLING_QUEUE_SIZE)
	c.Timestamp = time.Now().Unix()
	return c
}

func (c *Client) UpdateTs() {
	c.Timestamp = time.Now().Unix()
}

func (c *Client) IsExpired(now int64) bool {
	if c.IsPolling {
		return now-c.Timestamp > POLLING_EXPIRE_LIMIT
	}
	return now-c.Timestamp > WS_EXPIRE_LIMIT
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
	if c.IsPolling {
		return c.SendDataPolling(msg)
	}
	return c.SendDataWs(msg)
}

func (c *Client) SendDataPolling(data []byte) (error, bool) {
	if len(c.MsgQueue) >= POLLING_QUEUE_SIZE {
		err := errors.New(fmt.Sprintf("msg queue exceed %d", POLLING_QUEUE_SIZE))
		//log.Error("SendDataPolling", err)
		//if c.IsExpired(time.Now().Unix(), 30) {
		//	// 如果30s内还没有重新轮询
		//	return err, true
		//}
		return err, false
	}
	c.MsgQueue <- data
	return nil, false
}

func (c *Client) SendDataWs(data []byte) (error, bool) {
	opCode := ws.OpText
	err := wsutil.WriteServerMessage(c.Conn, opCode, data)
	if err != nil {
		// handle error
		//log.Info(err)
		return err, true
	}
	return nil, false
}

func (c *Client) Close() error {
	clientPool.Put(c)
	if c.IsPolling {
		//close(c.MsgQueue)
		return nil
	}
	return c.Conn.Close()
}
