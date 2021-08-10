package handler

import (
	"cbsignal/client"
	"cbsignal/hub"
	"cbsignal/redis"
	"cbsignal/rpcservice"
	"encoding/json"
	"github.com/lexkong/log"
)

type RejectHandler struct {
	Msg   *SignalMsg
	Cli   *client.Client
}

func (s *RejectHandler)Handle() {
	//h := hub.GetInstance()
	//判断节点是否还在线
	toPeerId := s.Msg.ToPeerId
	if s.Cli.HasNotFoundOrRejectPeer(toPeerId) {
		return
	}
	resp := SignalResp{
		Action: "reject",
		FromPeerId: s.Cli.PeerId,
		Reason: s.Msg.Reason,
	}
	if target, ok := hub.GetClient(toPeerId); ok {
		hub.SendJsonToClient(target, resp)
		s.Cli.EnqueueNotFoundOrRejectPeer(toPeerId)
		return
	}
	if addr, err := redis.GetRemotePeerRpcAddr(toPeerId); err == nil {
		node, ok := rpcservice.GetNode(addr)
		if ok {
			if !node.IsAlive() {
				log.Warnf("node %s is not alive when send signal", node.Addr())
				return
			}
			b, err := json.Marshal(resp)
			if err != nil {
				log.Error("json.Marshal", err)
				return
			}
			req := rpcservice.SignalReq{
				ToPeerId: toPeerId,
				Data:     b,
			}
			var resp rpcservice.RpcResp
			err = node.SendMsgSignal(req, &resp)
			if err != nil {
				log.Warnf("SendMsgSignal to remote failed " + err.Error())
				return
			}
			if !resp.Success {
				//log.Warnf("SendMsgSignal failed reason " + resp.Reason)
				log.Warnf(resp.Reason)
			}
			s.Cli.EnqueueNotFoundOrRejectPeer(toPeerId)
		} else {
			log.Warnf("node %s not found", addr)
		}
	}
}
