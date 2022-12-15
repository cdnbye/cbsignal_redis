package handler

import (
	"cbsignal/client"
	"cbsignal/hub"
	"cbsignal/nodes"
	"cbsignal/redis"
	"cbsignal/util/log"
)

type SignalHandler struct {
	Msg SignalMsg
	Cli *client.Client
}

func (s *SignalHandler) Handle() {
	//h := hub.GetInstance()
	//log.Infof("load client Msg %v", s.Msg)

	defer func() {
		//log.Warnf("signalMsgPool.Put(s)")
		signalMsgPool.Put(s)
	}()

	cli := s.Cli
	toPeerId := s.Msg.ToPeerId
	key := keyForFilter(cli.PeerId, toPeerId)
	if _, ok := filter.Get(key); ok {
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
				s.handlePeerNotFound(key, toPeerId)
			}
		}
		return
	}
	if addr, err := redis.GetRemotePeerAddr(toPeerId); err == nil {
		// 如果是本节点
		if addr == nodes.GetSelfAddr() {
			s.handlePeerNotFound(key, toPeerId)
			return
		}
		node, ok := nodes.GetNode(addr)
		if ok {
			err = node.SendMsgSignal(&signalResp, toPeerId)
			if err != nil {
				log.Warnf("SendMsgSignal to remote failed " + err.Error())
				s.handlePeerNotFound(key, toPeerId)
				return
			}
		} else {
			log.Warnf("node %s not found", addr)
			s.handlePeerNotFound(key, toPeerId)
		}
		return
	} else {
		if err != redis.ErrRedisNil {
			log.Warnf(err.Error())
		}
	}

	log.Infof("Peer %s not found", s.Msg.ToPeerId)
	s.handlePeerNotFound(key, toPeerId)
}
