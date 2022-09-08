package handler

import (
	"cbsignal/client"
	"cbsignal/hub"
	"cbsignal/nodes"
	"cbsignal/redis"
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
	if s.Cli.HasBlacklistPeer(toPeerId) {
		return
	}
	resp := nodes.SignalResp{
		Action: "reject",
		FromPeerId: s.Cli.PeerId,
		Reason: s.Msg.Reason,
	}
	if target, ok := hub.GetClient(toPeerId); ok {
		hub.SendJsonToClient(target, resp)
		s.Cli.EnqueueBlacklistPeer(toPeerId)
		return
	}
	if addr, err := redis.GetRemotePeerAddr(toPeerId); err == nil {
		// 如果是本节点
		if addr == nodes.GetSelfAddr() {
			return
		}
		node, ok := nodes.GetNode(addr)
		if ok {
			err = node.SendMsgSignal(&resp, toPeerId)
			if err != nil {
				log.Warnf("SendMsgSignal to remote failed " + err.Error())
				return
			}
			s.Cli.EnqueueBlacklistPeer(toPeerId)
		} else {
			log.Warnf("node %s not found", addr)
		}
	}
}
