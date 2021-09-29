package handler

import (
	"cbsignal/client"
	"cbsignal/hub"
	"cbsignal/redis"
	"cbsignal/rpcservice"
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
	resp := rpcservice.SignalResp{
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
		// 如果rpc节点是本节点
		if addr == rpcservice.GetSelfAddr() {
			return
		}
		node, ok := rpcservice.GetNode(addr)
		if ok {
			err = node.SendMsgSignal(&resp, toPeerId)
			if err != nil {
				log.Warnf("SendMsgSignal to remote failed " + err.Error())
				return
			}
			s.Cli.EnqueueNotFoundOrRejectPeer(toPeerId)
		} else {
			log.Warnf("node %s not found", addr)
		}
	}
}
