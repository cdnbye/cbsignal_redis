package redis

import (
	"fmt"
	"github.com/go-redis/redis"
	"time"
)

type RedisClient interface {
	redis.Cmdable
	Close() error
}

const (
	PEER_EXPIRE_DUTATION = 10*time.Minute
)

var (
	RedisCli RedisClient
	_rpcAddr string
)

func InitRedisClient(isCluster bool, rpcAddr string, redisAddr string, password string, db int) RedisClient {
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
			PoolSize:    150,                          // Maximum number of socket connections.
			PoolTimeout: time.Millisecond * 500,       // mount of time client waits for connection if all connections are busy before returning an error.
			ReadTimeout: time.Millisecond * 500,       //
		})
	}
	return RedisCli
}

func GetRemotePeerRpcAddr(peerId string) (string, error) {
	fmt.Println("redis GetRemotePeerRpcAddr peerId " + peerId)
	return RedisCli.Get(peerId).Result()
}

func SetLocalPeer(peerId string) error {
	//fmt.Printf("SetLocalPeer peerId %s _rpcAddr %s\n", peerId, _rpcAddr)
	return RedisCli.Set(peerId, _rpcAddr, PEER_EXPIRE_DUTATION).Err()
}

func DelLocalPeer(peerId string) error {
	return RedisCli.Del(peerId).Err()
}

func UpdateLocalPeerExpiration(peerId string) error {
	//fmt.Printf("UpdateLocalPeerExpiration peerId %s\n", peerId)
	return RedisCli.Expire(peerId, PEER_EXPIRE_DUTATION).Err()
}