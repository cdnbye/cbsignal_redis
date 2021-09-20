package rpcservice

import (
	"cbsignal/rpcservice/pool"
	"errors"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/lexkong/log"
	"net/rpc"
	"sync"
	"time"
)

var (
	json = jsoniter.ConfigCompatibleWithStandardLibrary
)

const (
	BROADCAST_SERVICE = "BroadcastService"
	PONG              = ".Pong"
	SIGNAL_SERVICE    = "SignalService"
	SIGNAL            = ".Signal"
	SIGNAL_BATCH      = ".SignalBatch"
	PING_INTERVAL     = 5
	READ_TIMEOUT      = 1500 * time.Millisecond
	POOL_MIN_CONNS = 5
	POOL_MAX_CONNS = 32
	CONSUME_INTERVAL = 15 * time.Millisecond
	ALERT_THRESHOLD = 100
)



type JoinLeaveReq struct {
	PeerId string // 节点id
	Addr   string
}

type RpcResp struct {
	Success bool
	Reason  string
}

type SignalReq struct {
	ToPeerId string
	Data     []byte
}

type SignalBatchReq struct {
	Items []*SignalReq
}

type Ping struct {
}

type Pong struct {
	NumClient int
}

type Node struct {
	sync.Mutex
	addr             string // ip:port
	ts               int64
	isAlive          bool // 是否存活
	connPool         pool.Pool
	Released         bool
	NumClient        int
	pipe             chan SignalReq
}

func NewNode(addr string) (*Node, error) {
	node := Node{
		addr: addr,
		pipe: make(chan SignalReq, 1000),
		ts:   time.Now().Unix(),
	}

	//factory 创建连接的方法
	factory := func() (*rpc.Client, error) {
		c, err := rpc.Dial("tcp", addr)
		if err != nil {

			return nil, err
		}
		return c, nil
	}

	//close 关闭连接的方法
	closer := func(v *rpc.Client) error { return v.Close() }

	poolConfig := &pool.Config{
		InitialCap: POOL_MIN_CONNS,         //资源池初始连接数
		MaxIdle:   POOL_MAX_CONNS,                 //最大空闲连接数
		MaxCap:     POOL_MAX_CONNS,//最大并发连接数
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

func (s *Node) IsMaster() bool {
	return false
}

func (s *Node) Addr() string {
	return s.addr
}

func (s *Node) Ts() int64 {
	return s.ts
}

//func (s *Node) SendMsgSignal(signalResp interface{}, toPeerId string) error {
//	//log.Infof("SendMsgSignal to %s", s.addr)
//
//	if !s.IsAlive() {
//		return errors.New(fmt.Sprintf("node %s is not alive when send signal", s.Addr()))
//	}
//
//	b, err := json.Marshal(signalResp)
//	if err != nil {
//		return err
//	}
//	req := SignalReq{
//		ToPeerId: toPeerId,
//		Data:     b,
//	}
//	var resp RpcResp
//	err = s.sendMsg(SIGNAL_SERVICE+SIGNAL, req, &resp)
//	if err != nil {
//		return err
//	}
//	if !resp.Success {
//		return errors.New(fmt.Sprintf("request is not success"))
//	}
//	return nil
//}

func (s *Node) SendMsgSignal(signalResp interface{}, toPeerId string) error {
	//log.Infof("SendMsgSignal to %s", s.addr)

	if !s.IsAlive() {
		return errors.New(fmt.Sprintf("node %s is not alive when send signal", s.Addr()))
	}

	b, err := json.Marshal(signalResp)
	if err != nil {
		return err
	}
	req := SignalReq{
		ToPeerId: toPeerId,
		Data:     b,
	}

	s.pipe <- req

	return nil
}

func (s *Node)Consume()  {
	defer func() {
		if err := recover(); err != nil {
			log.Error("Consume recover", errors.New(err.(string)))
		}
	}()
	var items []*SignalReq
	//var lock sync.Mutex
	for {
		time.Sleep(CONSUME_INTERVAL)
		//fmt.Printf("befor size is %d\n", len(pipe))
		l := len(s.pipe)
		if l > ALERT_THRESHOLD {
			log.Warnf("pipe len is %d", l)
		}
		items = make([]*SignalReq, 0, l)
		for i:=0;i<l;i++ {
			m := <- s.pipe
			//log.Infof("append signal from %s", m.ToPeerId)
			items = append(items, &m)
		}
		//fmt.Printf("after size is %d\n", len(pipe))
		if len(items) == 0 {
			continue
		}
		//batchReq := &SignalBatchReq{Items: items}
		//lock.Lock()
		go func() {
			if err := s.sendMsgSignalBatch(items); err != nil {
				//lock.Unlock()
				log.Error("sendMsgSignalBatch", err)
			}
		}()
		//lock.Unlock()
	}
}

func (s *Node)sendMsgSignalBatch(items []*SignalReq) error {
	var resp RpcResp
	batchReq := &SignalBatchReq{Items:items}
	log.Infof("send batch request len %d to %s", len(batchReq.Items), s.addr)
	err := s.sendMsg(SIGNAL_SERVICE+SIGNAL_BATCH, batchReq, &resp)
	if err != nil {
		return err
	}
	if !resp.Success {
		return errors.New(fmt.Sprintf("request is not success"))
	}
	return nil
}

func (s *Node) SendMsgPing(request Ping, reply *Pong) error {
	//log.Infof("SendMsgPing to %s", s.addr)
	return s.sendMsg(BROADCAST_SERVICE+PONG, request, reply)
}

func (s *Node) sendMsg(method string, request interface{}, reply interface{}) error {
	client, err := s.connPool.Acquire()

	if err != nil {
		return err
	}
	err = s.sendInternal(method, request, reply, client)
	if err != nil {
		s.connPool.Close(client)
	} else {
		s.connPool.Release(client)
	}
	return err
}

func (s *Node) sendInternal(method string, request interface{}, reply interface{}, client *rpc.Client) error {
	//start := time.Now()
	//done := make(chan error, 1)

	done := make(chan *rpc.Call, 1)

	//log.Warnf("GenericPool now conn %d idle %d", s.connPool.NumTotalConn(), s.connPool.NumIdleConn())

	client.Go(method, request, reply, done)

	select {
		case <-time.After(READ_TIMEOUT):
			//log.Warnf("rpc call timeout %s", method)
			//s.Client.Close()
			return fmt.Errorf("rpc call timeout %s", method)
		case call := <-done:
			//.Add(timeout).Before(time.Now())
			//elapsed := time.Since(start)
			//log.Warnf("6666 %d %d", elapsed.Nanoseconds(), PRINT_WARN_LIMIT_NANO)
			//if start.Add(PRINT_WARN_LIMIT_NANO).Before(time.Now()) {
			//	log.Warnf("rpc send %s cost %v", method, elapsed)
			//}

			//if err.Error != nil {
			//	//rpcClient.Close()
			//	return err.Error
			//}
			if err := call.Error; err != nil {
				//rpcClient.Close()
				return err
			}
	}

	return nil
}

func (s *Node) StartHeartbeat() {
	go func() {
		for {
			if s.Released {
				//log.Warnf("%s s.Released", s.addr)
				break
			}
			//log.Warnf("ConnPool %s conn %d idle %d", s.addr, s.connPool.NumTotalConn(), s.connPool.NumIdleConn())
			ping := Ping{}
			var pong Pong
			if err := s.SendMsgPing(ping, &pong); err != nil {
				log.Errorf(err, "node heartbeat %s", s.addr)
				s.Lock()
				s.isAlive = false
				s.Unlock()
			} else {
				s.Lock()
				s.isAlive = true
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
