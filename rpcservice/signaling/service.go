package signaling

import (
	"cbsignal/hub"
	"cbsignal/rpcservice"
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

func (b *Service) Signal(request rpcservice.SignalReq, reply *rpcservice.RpcResp) error  {
	//req := handler.SignalResp{}
	//if err := json.Unmarshal(request.Data, &req);err != nil {
	//	log.Warnf("json.Unmarshal error %s", err.Error())
	//	return err
	//}

	// test
	//time.Sleep(3*time.Second)

	//log.Infof("rpc receive signal from %s to %s action %s", req.FromPeerId, request.ToPeerId, req.Action)
	go func() {
		toPeerId := request.ToPeerId
		cli, ok := hub.GetClient(request.ToPeerId)
		if !ok {
			//log.Infof("local peer %s not found", toPeerId)
			// 节点不存在
			//reply.Success = false
			//reply.Reason = fmt.Sprintf("peer %s not found", req.FromPeerId)
		} else {
			//log.Infof("local peer %s found", toPeerId)

			//signalMsg := handler.SignalResp{
			//	Action: req.Action,
			//	FromPeerId: req.FromPeerId,
			//	Data: req.Data,
			//	Reason: req.Reason,
			//}

			//if err, _ := hub.SendJsonToClient(cli, req); err != nil {
			//	log.Warnf("%s send signal to peer %s error %s", req.FromPeerId, toPeerId, err)
			//}

			if err, _ := cli.SendMessage(request.Data); err != nil {
				log.Warnf("from remote send signal to peer %s error %s", toPeerId, err)
			}
		}
	}()

	reply.Success = true
	return nil
}

func (b *Service) SignalBatch(request rpcservice.SignalBatchReq, reply *rpcservice.RpcResp) error {
	log.Infof("received %d signals", len(request.Items))
	go func() {
		for _, item := range request.Items {
			toPeerId := item.ToPeerId
			cli, ok := hub.GetClient(item.ToPeerId)
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
