package signaling

import (
	"cbsignal/handler"
	"cbsignal/hub"
	"cbsignal/rpcservice"
	"encoding/json"
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
	req := handler.SignalResp{}
	if err := json.Unmarshal(request.Data, &req);err != nil {
		log.Warnf("json.Unmarshal error %s", err.Error())
		return err
	}

	// test
	//time.Sleep(3*time.Second)

	log.Infof("rpc receive signal from %s to %s action %s", req.FromPeerId, request.ToPeerId, req.Action)
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
			//reply.Success = true
			signalMsg := handler.SignalResp{
				Action: req.Action,
				FromPeerId: req.FromPeerId,
				Data: req.Data,
			}

			if err, _ := hub.SendJsonToClient(cli, signalMsg); err != nil {
				log.Warnf("%s send signal to peer %s error %s", req.FromPeerId, toPeerId, err)
			}
		}
	}()

	reply.Success = true
	return nil
}
