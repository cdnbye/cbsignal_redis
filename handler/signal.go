package handler

import (
	"cbsignal/client"
	"cbsignal/hub"
	"cbsignal/redis"
	"cbsignal/rpcservice"
	"github.com/lexkong/log"
)

type SignalHandler struct {
	Msg   *SignalMsg
	Cli   *client.Client
}

func (s *SignalHandler)Handle() {
	//h := hub.GetInstance()
	//log.Infof("load client Msg %v", s.Msg)

	cli := s.Cli

	if cli.PeerId == "" {
		log.Warnf("PeerId is not valid")
		return
	}

	/*
	 判断对等端是不是本地的，如果是的话获取节点发送
	 判断是否在RemotePeers缓存，是的话获取rpc addr发送
	 判断是否redis有，是的话获取rpc addr发送
	 发送peer not found
	 */
	toPeerId := s.Msg.ToPeerId
	if cli.HasNotFoundOrRejectPeer(toPeerId) {
		return
	}
	signalResp := SignalResp{
		Action: "signal",
		FromPeerId: cli.PeerId,
		Data: s.Msg.Data,
	}
	if addr, ok := cli.GetRemotePeer(toPeerId); ok {
		//log.Infof("signal GetRemotePeer %s addr %s", toPeerId, addr)
		node, ok := rpcservice.GetNode(addr)
		if ok {
			err := node.SendMsgSignal(signalResp, toPeerId)
			if err != nil {
				log.Warnf("SendMsgSignal to remote failed " + err.Error())
				s.handlePeerNotFound(toPeerId)
			}
		}
		return
	}
	if target, ok := hub.GetClient(toPeerId); ok {
		//log.Infof("SendJsonToClient %s", toPeerId)
		if err, fatal := hub.SendJsonToClient(target, signalResp); err != nil {
			log.Warnf("%s send signal to peer %s error %s", cli.PeerId, target.PeerId, err)
			if !fatal {
				s.handlePeerNotFound(toPeerId)
			}
		}
		return
	}
	if addr, err := redis.GetRemotePeerRpcAddr(toPeerId); err == nil {
		node, ok := rpcservice.GetNode(addr)
		if ok {
			err = node.SendMsgSignal(signalResp, toPeerId)
			if err != nil {
				log.Warnf("SendMsgSignal to remote failed " + err.Error())
				s.handlePeerNotFound(toPeerId)
				return
			}
		} else {
			log.Warnf("node %s not found", addr)
			s.handlePeerNotFound(toPeerId)
		}
		cli.EnqueueRemotePeer(toPeerId, addr)
		return
	} else {
		log.Info(err.Error())
	}

	log.Infof("Peer %s not found", s.Msg.ToPeerId)
	s.handlePeerNotFound(toPeerId)
}

func (s *SignalHandler)handlePeerNotFound(toPeerId string)  {
	//hub.SendJsonToClient(s.Cli.PeerId, resp)
	// 发送一次后，同一peerId下次不再发送，节省sysCall
	if !s.Cli.HasNotFoundOrRejectPeer(toPeerId) {
		s.Cli.EnqueueNotFoundOrRejectPeer(toPeerId)
		resp := SignalResp{
			Action: "signal",
			FromPeerId: s.Msg.ToPeerId,
		}
		hub.SendJsonToClient(s.Cli, resp)
	}
}


