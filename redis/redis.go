package redis

import (
	"errors"
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
	BREAK_DURATION = 1*time.Second
)

var (
	RedisCli RedisClient
	_rpcAddr string
	isAlive = true
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
	if !isAlive {
		return errors.New("redis is not alive")
	}
	//fmt.Printf("SetLocalPeer peerId %s _rpcAddr %s\n", peerId, _rpcAddr)
	err := RedisCli.Set(peerId, _rpcAddr, PEER_EXPIRE_DUTATION).Err()
	if err != nil {
		takeABreak()
	}
	return err
}

func DelLocalPeer(peerId string) error {
	if !isAlive {
		return errors.New("redis is not alive")
	}
	err := RedisCli.Del(peerId).Err()
	if err != nil {
		takeABreak()
	}
	return err
}

func UpdateLocalPeerExpiration(peerId string) error {
	//fmt.Printf("UpdateLocalPeerExpiration peerId %s\n", peerId)
	if !isAlive {
		return errors.New("redis is not alive")
	}
	err := RedisCli.Expire(peerId, PEER_EXPIRE_DUTATION).Err()
	if err != nil {
		takeABreak()
	}
	return err
}

func takeABreak()  {
	isAlive = false
	go func() {
		time.Sleep(BREAK_DURATION)

		isAlive = true
	}()
}