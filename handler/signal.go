package handler

import (
	"cbsignal/client"
	"cbsignal/hub"
	"cbsignal/nodes"
	"cbsignal/redis"
	"cbsignal/util/log"
)

type SignalHandler struct {
	Msg *SignalMsg
	Cli *client.Client
}

func (s *SignalHandler) Handle() {
	//h := hub.GetInstance()
	//log.Infof("load client Msg %v", s.Msg)

	cli := s.Cli

	if cli.PeerId == "" {
		log.Warnf("PeerId is not valid")
		return
	}

	toPeerId := s.Msg.ToPeerId
	//if cli.HasBlacklistPeer(toPeerId) {
	//	return
	//}
	if redis.IsNotFoundPeer(toPeerId) {
		return
	}
	signalResp := nodes.SignalResp{
		Action:     "signal",
		FromPeerId: cli.PeerId,
		Data:       s.Msg.Data,
	}
	if target, ok := hub.GetClient(toPeerId); ok {
		//log.Infof("SendJsonToClient %s", toPeerId)
		if err, fatal := hub.SendJsonToClient(target, signalResp); err != nil {
			log.Infof("%s send signal to peer %s error %s", cli.PeerId, target.PeerId, err)
			if !fatal {
				s.handlePeerNotFound(toPeerId)
			}
		}
		return
	}
	if addr, err := redis.GetRemotePeerAddr(toPeerId); err == nil {
		// 如果是本节点
		if addr == nodes.GetSelfAddr() {
			s.handlePeerNotFound(toPeerId)
			return
		}
		node, ok := nodes.GetNode(addr)
		if ok {
			err = node.SendMsgSignal(&signalResp, toPeerId)
			if err != nil {
				log.Warnf("SendMsgSignal to remote failed " + err.Error())
				s.handlePeerNotFound(toPeerId)
				return
			}
		} else {
			log.Warnf("node %s not found", addr)
			s.handlePeerNotFound(toPeerId)
		}
		return
	} else {
		log.Info(err.Error())
	}

	log.Infof("Peer %s not found", s.Msg.ToPeerId)
	s.handlePeerNotFound(toPeerId)
}

func (s *SignalHandler) handlePeerNotFound(toPeerId string) {
	// 发送一次后，同一peerId下次不再发送，节省sysCall
	//if !s.Cli.HasBlacklistPeer(toPeerId) {
	//	s.Cli.EnqueueBlacklistPeer(toPeerId)
	//	resp := nodes.SignalResp{
	//		Action:     "signal",
	//		FromPeerId: s.Msg.ToPeerId,
	//	}
	//	hub.SendJsonToClient(s.Cli, resp)
	//}
	redis.SetNotFoundPeer(toPeerId)
	resp := nodes.SignalResp{
		Action:     "signal",
		FromPeerId: s.Msg.ToPeerId,
	}
	hub.SendJsonToClient(s.Cli, resp)
}
