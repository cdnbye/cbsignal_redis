package signaling

import (
	"cbsignal/hub"
	"cbsignal/rpcservice"
	"errors"
	"fmt"
	"github.com/lexkong/log"
	"net/rpc"
)

type Service struct {

}

func RegisterSignalService() error {
	log.Infof("register rpc service %s", rpcservice.SIGNAL_SERVICE)
	s := new(Service)
	return rpc.RegisterName(rpcservice.SIGNAL_SERVICE, s)
}

func (b *Service) SignalBatch(request rpcservice.SignalBatchReq, reply *rpcservice.RpcResp) error {
	log.Infof("received %d signals token %s", len(request.Items), request.Token)
	if request.Token != rpcservice.Token {
		msg := fmt.Sprintf("rpc from %s token not matched", request.From)
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
