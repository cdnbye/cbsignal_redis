package rpcservice

import (
	message "cbsignal/protobuf"
	"cbsignal/rpcservice/pool"
	"cbsignal/rpcservice/signaling"
	"errors"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/lexkong/log"
	"runtime"
	"sync"
	"time"
)

var (
	json = jsoniter.ConfigCompatibleWithStandardLibrary
	poolMaxConns = runtime.NumCPU() * 8
)

const (
	PING_INTERVAL     = 5
	PING_MAX_RETRYS   = 3
	POOL_MIN_CONNS = 3
	CONSUME_INTERVAL = 25 * time.Millisecond
	ALERT_THRESHOLD = 100
)

type Node struct {
	sync.Mutex
	addr             string // ip:port
	ts               int64
	isAlive          bool // 是否存活
	connPool         pool.Pool
	Released         bool
	NumClient        int32
	pipe             chan *message.SignalReq
	pingRetrys       int
	IsDead           bool
}

type SignalResp struct {
	Action string              `json:"action"`
	FromPeerId string          `json:"from_peer_id,omitempty"`
	Data interface{}           `json:"data,omitempty"`
	Reason string              `json:"reason,omitempty"`
}

func NewNode(addr string) (*Node, error) {
	node := Node{
		addr: addr,
		pipe: make(chan *message.SignalReq, 1000),
		ts:   time.Now().Unix(),
	}

	//factory 创建连接的方法
	factory := func() (*signaling.SignalServiceClient, error) {
		c, err := signaling.DialSignalService("tcp", addr)
		if err != nil {
			return nil, err
		}
		// auth
		authReq := &message.Auth{
			Token: signaling.Token,
			From: GetSelfAddr(),
		}
		var authResp message.RpcResp
		if err := c.Login(authReq, &authResp); err != nil {
			return nil, err
		}
		if !authResp.Success {
			return nil, errors.New(authResp.Reason)
		}
		return c, nil
	}

	//close 关闭连接的方法
	closer := func(v *signaling.SignalServiceClient) error { return v.Close() }

	poolConfig := &pool.Config{
		InitialCap: POOL_MIN_CONNS,         //资源池初始连接数
		MaxIdle:   poolMaxConns,                 //最大空闲连接数
		MaxCap:     poolMaxConns,//最大并发连接数
		Factory:    factory,
		Close:      closer,
		//连接最大空闲时间，超过该时间的连接 将会关闭，可避免空闲时连接EOF，自动失效的问题
		IdleTimeout: 60 * time.Second,
	}
	p, err := pool.NewChannelPool(poolConfig)

	if err != nil {
		return nil, err
	}
	node.connPool = p
	node.isAlive = true

	// 定时消费batch request
	go node.Consume()

	return &node, nil
}

func (s *Node) UpdateTs() {
	s.ts = time.Now().Unix()
}

func (s *Node) Addr() string {
	return s.addr
}

func (s *Node) Ts() int64 {
	return s.ts
}

func (s *Node) SendMsgSignal(signalResp *SignalResp, toPeerId string) error {
	//log.Infof("SendMsgSignal to %s", s.addr)

	if !s.IsAlive() {
		return errors.New(fmt.Sprintf("node %s is not alive when send signal", s.Addr()))
	}

	b, err := json.Marshal(signalResp)
	if err != nil {
		return err
	}
	req := &message.SignalReq{
		ToPeerId: toPeerId,
		Data:     b,
	}

	s.pipe <- req

	return nil
}

func (s *Node)Consume()  {
	ticker := time.NewTicker(CONSUME_INTERVAL)
	defer func() {
		ticker.Stop()
		if err := recover(); err != nil {
			go s.Consume()
		}
	}()
	var items []*message.SignalReq
	for range ticker.C {
		l := len(s.pipe)
		if l > ALERT_THRESHOLD {
			log.Warnf("rpc pipe len is %d", l)
		}
		items = make([]*message.SignalReq, 0, l)
		for i:=0;i<l;i++ {
			m := <- s.pipe
			//log.Infof("append signal from %s", m.ToPeerId)
			items = append(items, m)
		}
		//fmt.Printf("after size is %d\n", len(pipe))
		if len(items) == 0 {
			continue
		}
		go func() {
			if err := s.sendMsgSignalBatch(items); err != nil {
				//lock.Unlock()
				log.Errorf(err, "sendMsgSignalBatch to %s", s.addr)
			}
		}()
	}
}

func (s *Node)sendMsgSignalBatch(items []*message.SignalReq) error {
	var resp message.RpcResp
	batchReq := &message.SignalBatchReq{
		Items: items,
	}
	log.Infof("send batch request len %d to %s", len(batchReq.Items), s.addr)
	err := s.sendSignalBatch(batchReq, &resp)
	if err != nil {
		return err
	}
	if !resp.Success {
		return errors.New(fmt.Sprintf("request is not success"))
	}
	return nil
}

func (s *Node) SendMsgPing(request *message.Ping, reply *message.Pong) error {
	//log.Infof("SendMsgPing to %s", s.addr)
	client, err := s.connPool.Acquire()
	if err != nil {
		return err
	}
	err = client.Pong(request, reply)
	if err != nil {
		s.connPool.Close(client)
	} else {
		s.connPool.Release(client)
	}
	return err
}

func (s *Node) sendSignalBatch(request *message.SignalBatchReq, reply *message.RpcResp) error {
	client, err := s.connPool.Acquire()
	if err != nil {
		return err
	}
	err = client.SignalBatch(request, reply)
	if err != nil {
		s.connPool.Close(client)
	} else {
		s.connPool.Release(client)
	}
	return err
}

func (s *Node) StartHeartbeat() {
	go func() {
		for {
			if s.Released {
				//log.Warnf("%s s.Released", s.addr)
				break
			}
			if s.pingRetrys > PING_MAX_RETRYS {
				s.IsDead = true
				break
			}
			//log.Warnf("ConnPool %s conn %d idle %d", s.addr, s.connPool.NumTotalConn(), s.connPool.NumIdleConn())
			ping := &message.Ping{From: GetSelfAddr()}
			var pong message.Pong
			if err := s.SendMsgPing(ping, &pong); err != nil {
				log.Errorf(err, "node heartbeat %s", s.addr)
				s.Lock()
				s.isAlive = false
				s.pingRetrys ++
				s.Unlock()
			} else {
				s.Lock()
				s.isAlive = true
				s.pingRetrys = 0
				s.NumClient = pong.NumClient
				s.Unlock()
			}
			time.Sleep(PING_INTERVAL * time.Second)
		}
	}()
}

func (s *Node) IsAlive() bool {
	return s.isAlive
}
