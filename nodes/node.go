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
	PING_INTERVAL      = 10
	PING_MAX_RETRYS    = 3
	MQ_MAX_LEN         = 500
	MQ_LEN_AFTER_TRIM  = 200
)

type Node struct {
	sync.Mutex
	addr             string // ip:port
	isAlive          bool // 是否存活
	NumClient        int64
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

	raw, err := proto.Marshal(req)
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
			if count, err := redis.GetNodeClientCount(s.addr); err != nil {
				log.Errorf(err, "node heartbeat %s", s.addr)
				s.Lock()
				s.isAlive = false
				s.pingRetrys ++
				s.Unlock()
				// 清空队列
				if length, err := redis.GetLenMQ(s.addr); err != nil {
					log.Error("GetLenMQ", err)
				} else if length > 0 {
					redis.ClearMQ(s.addr)
				}
			} else {
				s.Lock()
				s.isAlive = true
				s.pingRetrys = 0
				s.NumClient = count
				s.Unlock()
			}
			time.Sleep(PING_INTERVAL * time.Second)
		}
	}()
}

func (s *Node) IsAlive() bool {
	return s.isAlive
}
