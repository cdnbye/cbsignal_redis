package redis

import (
	"errors"
	"github.com/go-redis/redis"
	"sync"
	"time"
)

type RedisClient interface {
	redis.Cmdable
	Close() error
}

const (
	PEER_EXPIRE_DUTATION = 10*time.Minute
	CLIENT_ALIVE_EXPIRE_DUTATION = 20*time.Second
	BREAK_DURATION = 2*time.Second
    ERR_REDIS_NIL = redis.Nil
)

var (
	RedisCli RedisClient
	_rpcAddr string
	IsAlive  = true
	once     sync.Once
)

func InitRedisClient(isCluster bool, rpcAddr string, redisAddr string, password string, db int) RedisClient {
	once.Do(func() {
		_rpcAddr = rpcAddr
		if isCluster {
			RedisCli = redis.NewClusterClient(&redis.ClusterOptions{
				Addrs:    []string{redisAddr},
				Password: password,
			})
		} else {
			RedisCli = redis.NewClient(&redis.Options{
				Addr:        redisAddr,
				Password:    password,
				DB:          db, // use default DB
				PoolSize:    80,                          // Maximum number of socket connections.
				PoolTimeout: time.Millisecond * 500,       // mount of time client waits for connection if all connections are busy before returning an error.
				ReadTimeout: time.Millisecond * 500,       //
			})
		}
	})
	return RedisCli
}

func GetRemotePeerAddr(peerId string) (string, error) {
	//fmt.Println("redis GetRemotePeerAddr peerId " + peerId)
	return RedisCli.Get(genKeyForPeerId(peerId)).Result()
}

func SetLocalPeer(peerId string) error {
	if !IsAlive {
		return errors.New("redis is not alive")
	}
	//fmt.Printf("SetLocalPeer peerId %s _rpcAddr %s\n", peerId, _rpcAddr)
	err := RedisCli.Set(genKeyForPeerId(peerId), _rpcAddr, PEER_EXPIRE_DUTATION).Err()
	if err != nil {
		takeABreak()
	}
	return err
}

func DelLocalPeer(peerId string) error {
	if !IsAlive {
		return errors.New("redis is not alive")
	}
	err := RedisCli.Del(genKeyForPeerId(peerId)).Err()
	if err != nil {
		takeABreak()
	}
	return err
}

func UpdateLocalPeerExpiration(peerId string) error {
	//fmt.Printf("UpdateLocalPeerExpiration peerId %s\n", peerId)
	if !IsAlive {
		return errors.New("redis is not alive")
	}
	return RedisCli.Expire(genKeyForPeerId(peerId), PEER_EXPIRE_DUTATION).Err()
}

func PushMsgToMQ(addr string, msg interface{}) (int64, error) {
	return RedisCli.RPush(genKeyForMQ(addr), msg).Result()
}

func GetLenMQ(addr string) (int64, error) {
	return RedisCli.LLen(genKeyForMQ(addr)).Result()
}

func ClearMQ(addr string) error {
	return RedisCli.LTrim(genKeyForMQ(addr), 1, 0).Err()
}

func BlockPopMQ(timeout time.Duration, addr string) ([]byte, error) {
	result, err := RedisCli.BLPop(timeout, genKeyForMQ(addr)).Result()
	if err != nil {
		return nil, err
	}
	return []byte(result[1]), nil
}

func TrimMQ(addr string, len int64) error {
	return RedisCli.LTrim(genKeyForMQ(addr), -len, -1).Err()
}

func PopRangeMQ(addr string, len int64) ([]string, error) {
	key := genKeyForMQ(addr)
	pClient := RedisCli.Pipeline()
	pClient.LRange(key, 0, len-1)
	pClient.LTrim(key, len, -1)
	c, err := pClient.Exec()
	if err != nil {
		return nil, err
	}
	return c[0].(*redis.StringSliceCmd).Result()
}

func UpdateClientCount(count int64) error {
	return RedisCli.Set(genKeyForStats(_rpcAddr), count, CLIENT_ALIVE_EXPIRE_DUTATION).Err()
}

func GetNodeClientCount(addr string) (int64, error) {
	return RedisCli.Get(genKeyForStats(addr)).Int64()
}

func genKeyForPeerId(peerId string) string {
	return "signal:peerId:" + peerId
}

func genKeyForMQ(addr string) string {
	return "signal:mq:" + addr
}

func genKeyForStats(addr string) string {
	return "signal:stats:count:" + addr
}

func takeABreak()  {
	IsAlive = false
	go func() {
		time.Sleep(BREAK_DURATION)

		IsAlive = true
	}()
}