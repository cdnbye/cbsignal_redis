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
	MQ_MAX_LEN         = 1000
	MQ_LEN_AFTER_TRIM  = 500
	MAX_PIPE_LEN       = 35
	CONSUME_INTERVAL   = 40 * time.Millisecond
)

type Node struct {
	sync.Mutex
	addr             string // ip:port
	isAlive          bool // 是否存活
	NumClient        int64
	pingRetrys       int
	IsDead           bool
	aggregator       *Aggregator
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
	}

	node.isAlive = true

	batchProcess := func(items []*message.SignalReq) error {
		if len(items) == 0 {
			return nil
		}
		//log.Infof("send %d items", len(items))
		return node.sendBatchReq(items)
	}

	errorHandler := func(err error, items []*message.SignalReq, batchProcessFunc BatchProcessFunc, aggregator *Aggregator) {
		log.Errorf(err,"Receive error, item size is %d", len(items))
		node.sendBatchReq(items)
	}

	node.aggregator = NewAggregator(batchProcess, func(option AggregatorOption) AggregatorOption {
		option.BatchSize = MAX_PIPE_LEN
		option.Workers = 1
		option.ChannelBufferSize = MAX_PIPE_LEN + 10
		option.LingerTime = CONSUME_INTERVAL
		option.ErrorHandler = errorHandler
		return option
	})

	node.aggregator.Start()

	return &node, nil
}

func (s *Node) Addr() string {
	return s.addr
}

func(s *Node) Destroy() {
	s.aggregator.SafeStop()
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

	s.aggregator.TryEnqueue(req)

	return nil
}

func (s *Node)sendBatchReq(items []*message.SignalReq) error {
	batchReq := &message.SignalBatchReq{
		Items: items,
	}
	raw, err := proto.Marshal(batchReq)
	if err != nil {
		return err
	}
	length, err := redis.PushMsgToMQ(s.addr, raw)
	if err != nil {
		return err
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
	return nil
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
