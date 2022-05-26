package pool

import (
	. "cbsignal/rpcservice/signaling"
	"errors"
)

var (
	// ErrClosed 连接池已经关闭Error
	ErrClosed = errors.New("pool is closed")
)

type Pool interface {
	Acquire() (*SignalServiceClient, error) // 获取资源
	Release(*SignalServiceClient) error     // 释放资源
	Close(*SignalServiceClient) error       // 关闭资源
	Shutdown() error             // 关闭池
	NumTotalConn() int
	NumIdleConn() int
}
