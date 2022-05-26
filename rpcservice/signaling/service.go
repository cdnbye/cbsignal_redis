package signaling

import (
	"cbsignal/hub"
	message "cbsignal/protobuf"
	"errors"
	"fmt"
	"github.com/lexkong/log"
	"github.com/mars9/codec"
	"net"
	"net/rpc"
)

type SignalServiceInterface = interface {
	SignalBatch(request *message.SignalBatchReq, reply *message.RpcResp) error
	Login(request *message.Auth, reply *message.RpcResp ) error
	Pong(request *message.Ping, reply *message.Pong) error
}

func RegisterSignalService(svc SignalServiceInterface) error {
	return rpc.RegisterName(SIGNAL_SERVICE, svc)
}

func DialSignalService(network, address string) (*SignalServiceClient, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	c := rpc.NewClientWithCodec(codec.NewClientCodec(conn))
	return &SignalServiceClient{Client: c}, nil
}

type SignalService struct {
	Conn    net.Conn
	isLogin bool
	from    string
}

func (b *SignalService) Login(request *message.Auth, reply *message.RpcResp) error {
	if request.Token != Token {
		msg := fmt.Sprintf("rpc from %s token %s not matched %s", request.From, request.Token, Token)
		log.Warn(msg)
		return errors.New(msg)
	}
	b.isLogin = true
	b.from = request.From
	log.Infof("receive login from %s", request.From)
	reply.Success = true
	return nil
}

func (b *SignalService) SignalBatch(request *message.SignalBatchReq, reply *message.RpcResp) error {
	log.Infof("received %d signals", len(request.Items))
	if !b.isLogin {
		msg := fmt.Sprintf("rpc from %s not auth", b.from)
		log.Warn(msg)
		return errors.New(msg)
	}
	go func() {
		for _, item := range request.Items {
			toPeerId := item.ToPeerId
			cli, ok := hub.GetClient(toPeerId)
			if ok {
				log.Infof("batch local peer %s found", toPeerId)
				if err, _ := cli.SendMessage(item.Data); err != nil {
					log.Warnf("from remote send signal to peer %s error %s", toPeerId, err)
					if ok := hub.DoUnregister(cli.PeerId); ok {
						cli.Close()
					}
				}
			}
		}
	}()

	reply.Success = true
	return nil
}

func (b *SignalService) Pong(request *message.Ping, reply *message.Pong) error {
	//time.Sleep(1 * time.Second)
	if !b.isLogin {
		msg := fmt.Sprintf("rpc from %s not auth", request.From)
		log.Warn(msg)
		return errors.New(msg)
	}
	reply.NumClient = hub.GetClientNum()
	return nil
}
