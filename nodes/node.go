package nodes

import (
	message "cbsignal/protobuf"
	"cbsignal/redis"
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	jsoniter "github.com/json-iterator/go"
	"github.com/lexkong/log"
	"sync"
	"time"
)

var (
	json = jsoniter.ConfigCompatibleWithStandardLibrary
)

const (
	PING_INTERVAL      = 7
	PING_MAX_RETRYS    = 2
	MQ_MAX_LEN         = 800
	MQ_LEN_AFTER_TRIM  = 500
	MAX_PIPE_LEN       = 50
)

type Node struct {
	sync.Mutex
	addr             string // ip:port
	isAlive          bool // 是否存活
	NumClient        int64
	pingRetrys       int
	IsDead           bool
	pipe             chan *message.SignalReq
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
	}

	node.isAlive = true

	return &node, nil
}

func (s *Node) Addr() string {
	return s.addr
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

	if len(s.pipe) > MAX_PIPE_LEN {
		log.Warnf("pipe len %d reset ticker", len(s.pipe))
		s.sendBatchReq()
	}

	return nil
}

func (s *Node)sendBatchReq()  {
	var items []*message.SignalReq
	l := len(s.pipe)
	items = make([]*message.SignalReq, 0, l)
	for i:=0;i<l;i++ {
		m := <- s.pipe
		//log.Infof("append signal from %s", m.ToPeerId)
		items = append(items, m)
	}
	//fmt.Printf("after size is %d\n", len(pipe))
	if len(items) == 0 {
		return
	}
	batchReq := &message.SignalBatchReq{
		Items: items,
	}
	raw, err := proto.Marshal(batchReq)
	if err != nil {
		log.Error("proto.Marshal", err)
		return
	}
	length, err := redis.PushMsgToMQ(s.addr, raw)
	if err != nil {
		log.Error("PushMsgToMQ", err)
		return
	}
	if length > MQ_MAX_LEN {
		log.Warnf("before trim %s, len %d", s.addr, length)
		err := redis.TrimMQ(s.addr, MQ_LEN_AFTER_TRIM)
		if err != nil {
			log.Error("TrimMQ", err)
		} else {
			curLength, _ := redis.GetLenMQ(s.addr)
			log.Warnf("trim %s done, current len %d", s.addr, curLength)
		}
	}
}

func (s *Node) StartHeartbeat() {
	go func() {
		for {
			if s.pingRetrys > PING_MAX_RETRYS {
				s.IsDead = true
				break
			}
			time.Sleep(PING_INTERVAL * time.Second)
			if count, err := redis.GetNodeClientCount(s.addr); err != nil {
				log.Errorf(err, "node heartbeat %s", s.addr)
				s.Lock()
				if s.isAlive {
					s.isAlive = false
					// 清空队列
					if length, err := redis.GetLenMQ(s.addr); err != nil {
						log.Error("GetLenMQ", err)
					} else if length > 0 {
						redis.ClearMQ(s.addr)
					}
				}
				s.pingRetrys ++
				s.Unlock()
			} else {
				s.Lock()
				if !s.isAlive {
					s.isAlive = true
				}
				s.pingRetrys = 0
				s.NumClient = count
				s.Unlock()
			}
		}
	}()
}

func (s *Node) IsAlive() bool {
	return s.isAlive
}
