package hub

import (
	"cbsignal/client"
	message "cbsignal/protobuf"
	"cbsignal/redis"
	"cbsignal/util/fastmap/cmap"
	"github.com/golang/protobuf/proto"
	jsoniter "github.com/json-iterator/go"
	"github.com/lexkong/log"
	"sync"
	"time"
)

const (
	MQ_BLOCK_DURATION = 5 * time.Second
	MQ_SLEEP_DURATION = 10 * time.Millisecond
	MQ_GET_RANGE_LEN = 50
)

var (
	json = jsoniter.ConfigCompatibleWithStandardLibrary
	h *Hub
	once sync.Once
)

type Hub struct {
	Clients cmap.ConcurrentMap
}

func Init() {
	once.Do(func() {
		h = &Hub{
			Clients: cmap.NewCMap(),
			//Clients: smap.NewSMap(),
		}
	})
}

func GetInstance() *Hub {
	return h
}

func GetClientCount() int64 {
	return int64(h.Clients.CountNoLock())
}

func GetClientNumPerMap() []int {
	return h.Clients.CountPerMapNoLock()
}

func DoRegister(client *client.Client) {
	log.Infof("hub DoRegister %s", client.PeerId)
	/*
		1. 本地保存节点id
		2. redis保存节点id和addr
	*/
	h.Clients.Set(client.PeerId, client)
	if err := redis.SetLocalPeer(client.PeerId); err != nil {
		log.Error("SetLocalPeer", err)
	}
}

func GetClient(id string) (*client.Client, bool) {
	return h.Clients.Get(id)
}

func HasClient(id string) bool {
	return h.Clients.Has(id)
}

func RemoveClient(id string) {
	h.Clients.Remove(id)
}

func DoUnregister(peerId string) bool {
	log.Infof("hub DoUnregister %s", peerId)
	if peerId == "" {
		return false
	}
	/*
		1. 本地删除节点id
		2. redis删除节点id
	*/
	if h.Clients.Has(peerId) {
		h.Clients.Remove(peerId)
		//go func() {
		//	if err := redis.DelLocalPeer(peerId); err != nil {
		//		log.Error("DelLocalPeer", err)
		//	}
		//}()
		if err := redis.DelLocalPeer(peerId); err != nil {
			log.Error("DelLocalPeer", err)
		}
		return true
	}
	return false
}

// send json object to a client with peerId
func SendJsonToClient(target *client.Client, value interface{}) (error, bool) {

	b, err := json.Marshal(value)
	if err != nil {
		log.Error("json.Marshal", err)
		return err, false
	}
	defer func() {                            // 必须要先声明defer，否则不能捕获到panic异常
		if err := recover(); err != nil {
			log.Warnf(err.(string))                  // 这里的err其实就是panic传入的内容
		}
	}()
	return target.SendMessage(b)
}

func ClearAll()  {
	h.Clients.Clear()
}

func Consume(addr string)  {
	for {
		b, err := redis.BlockPopMQ(MQ_BLOCK_DURATION, addr)
		if err != nil {
			if err != redis.ERR_REDIS_NIL {
				log.Errorf(err, "BlockPopMQ %s", addr)
			}
			continue
		}
		sendMessageToLocalPeer(b)
		tryConsumeRange(addr)
	}
}

func tryConsumeRange(addr string) {
	arr, err := redis.PopRangeMQ(addr, MQ_GET_RANGE_LEN)
	if err != nil {
		log.Errorf(err, "PopRangeMQ %s", addr)
		return
	}
	if len(arr) == 0 {
		return
	}
	for _, item := range arr {
		sendMessageToLocalPeer([]byte(item))
	}
	tryConsumeRange(addr)
}

func sendMessageToLocalPeer(raw []byte) {
	var data message.SignalReq
	if err := proto.Unmarshal(raw, &data); err != nil {
		log.Errorf(err, "json.Unmarshal")
		return
	}
	cli, ok := GetClient(data.ToPeerId)
	if ok {
		log.Infof("local peer %s found", data.ToPeerId)
		if err, _ := cli.SendMessage(data.Data); err != nil {
			log.Warnf("from remote send signal to peer %s error %s", data.ToPeerId, err)
			if ok := DoUnregister(cli.PeerId); ok {
				cli.Close()
			}
		}
	}
}




