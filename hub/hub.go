package hub

import (
	"cbsignal/client"
	"cbsignal/redis"
	"cbsignal/util/fastmap/cmap"
	jsoniter "github.com/json-iterator/go"
	"github.com/lexkong/log"
	"sync"
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

func GetClientNum() int32 {
	return int32(h.Clients.CountNoLock())
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




