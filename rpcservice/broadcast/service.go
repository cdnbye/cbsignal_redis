package broadcast

import (
	"cbsignal/rpcservice"
	"github.com/lexkong/log"
	"net/rpc"
)

type Service struct {

}

func RegisterBroadcastService() error {
	log.Infof("register rpc service %s", rpcservice.BROADCAST_SERVICE)
	s := new(Service)
	return rpc.RegisterName(rpcservice.BROADCAST_SERVICE, s)
}

func (h *Service) Pong(request rpcservice.Ping, reply *rpcservice.Pong) error {
	//time.Sleep(1 * time.Second)
	return nil
}
