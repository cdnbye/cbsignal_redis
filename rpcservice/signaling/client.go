package signaling

import (
	message "cbsignal/protobuf"
	"fmt"
	"net/rpc"
	"time"
)

const (
	READ_TIMEOUT      = 1500 * time.Millisecond
)

type SignalServiceClient struct {
	*rpc.Client
}

var _ SignalServiceInterface = (*SignalServiceClient)(nil)

func (p *SignalServiceClient) SignalBatch(request *message.SignalBatchReq, reply *message.RpcResp) error {
	return p.sendInternal(SIGNAL_SERVICE+SIGNAL_BATCH, request, reply)
}

func (p *SignalServiceClient) Login(request *message.Auth, reply *message.RpcResp) error {
	return p.sendInternal(SIGNAL_SERVICE+LOGIN, request, reply)
}

func (p *SignalServiceClient) Pong(request *message.Ping, reply *message.Pong) error {
	return p.sendInternal(SIGNAL_SERVICE+PONG, request, reply)
}

func (p *SignalServiceClient) sendInternal(method string, request interface{}, reply interface{}) error {
	done := make(chan *rpc.Call, 1)
	//log.Warnf("GenericPool now conn %d idle %d", s.connPool.NumTotalConn(), s.connPool.NumIdleConn())
	p.Go(method, request, reply, done)

	select {
	case <-time.After(READ_TIMEOUT):
		return fmt.Errorf("rpc call timeout %s", method)
	case call := <-done:
		if err := call.Error; err != nil {
			//rpcClient.Close()
			return err
		}
	}
	return nil
}


