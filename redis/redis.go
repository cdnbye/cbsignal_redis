package redis

import (
	"errors"
	"github.com/allegro/bigcache/v3"
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
	SelfAddr string
	IsAlive  = true
	once     sync.Once
	cache    *bigcache.BigCache
)

func InitRedisClient(isCluster bool, selfAddr string, redisAddr string, password string, db int) RedisClient {
	once.Do(func() {
		SelfAddr = selfAddr
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
				PoolSize:    100,                          // Maximum number of socket connections.
				PoolTimeout: time.Millisecond * 500,       // mount of time client waits for connection if all connections are busy before returning an error.
				ReadTimeout: time.Millisecond * 500,       //
			})
		}

		cacheConfig := bigcache.Config {
			Shards: 128,
			LifeWindow: 10 * time.Minute,
			CleanWindow: 5 * time.Minute,

			// rps * lifeWindow, used only in initial memory allocation
			MaxEntriesInWindow: 1000 * 10 * 40,

			// max entry size in bytes, used only in initial memory allocation
			MaxEntrySize: 20,

			Verbose: false,

			// cache will not allocate more memory than this limit, value in MB
			// if value is reached then the oldest entries can be overridden for the new ones
			// 0 value means no size limit
			HardMaxCacheSize: 15,
		}
		var err error
		cache, err = bigcache.NewBigCache(cacheConfig)
		if err != nil {
			panic(err)
		}
	})
	return RedisCli
}

func GetRemotePeerAddr(peerId string) (string, error) {
	//fmt.Println("redis GetRemotePeerAddr peerId " + peerId)
	v, err := cache.Get(peerId)
	if err != nil {
		addr, err := RedisCli.Get(genKeyForPeerId(peerId)).Result()
		if err == nil {
			_ = cache.Set(peerId, []byte(addr))
		}
		return addr, err
	}
	return string(v), nil
}

func SetLocalPeer(peerId string) error {
	if !IsAlive {
		return errors.New("redis is not alive")
	}
	//fmt.Printf("SetLocalPeer peerId %s SelfAddr %s\n", peerId, SelfAddr)
	err := RedisCli.Set(genKeyForPeerId(peerId), SelfAddr, PEER_EXPIRE_DUTATION).Err()
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
	return RedisCli.Set(genKeyForStats(SelfAddr), count, CLIENT_ALIVE_EXPIRE_DUTATION).Err()
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