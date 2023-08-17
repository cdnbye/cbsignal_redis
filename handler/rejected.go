package handler

import (
	"cbsignal/client"
	"cbsignal/hub"
	"cbsignal/nodes"
	"cbsignal/redis"
	"cbsignal/util/log"
)

type RejectHandler struct {
	Msg SignalMsg
	Cli *client.Client
}

func (s *RejectHandler) Handle() {
	//判断节点是否还在线
	toPeerId := s.Msg.ToPeerId
	key := keyForFilter(s.Cli.PeerId, toPeerId)
	if _, ok := filter.Get(key); ok {
		return
	}
	//log.Warnf("reject reason %s", s.Msg.Reason)
	resp := nodes.SignalResp{
		Action: "reject",
		Reason: s.Msg.Reason,
	}
	if s.Cli.IsPolling {
		resp.From = s.Cli.PeerId
	} else {
		resp.FromPeerId = s.Cli.PeerId
	}
	if target, ok := hub.GetClient(toPeerId); ok {
		hub.SendJsonToClient(target, resp)
		filter.Put(key, nil)
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
			filter.Put(key, nil)
		} else {
			log.Warnf("node %s not found", addr)
		}
	}
}
