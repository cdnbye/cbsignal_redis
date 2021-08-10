package handler

import (
	"cbsignal/client"
	"cbsignal/hub"
	"cbsignal/redis"
	"cbsignal/rpcservice"
	"encoding/json"
	"github.com/lexkong/log"
)

type SignalHandler struct {
	Msg   *SignalMsg
	Cli   *client.Client
}

func (s *SignalHandler)Handle() {
	//h := hub.GetInstance()
	//log.Infof("load client Msg %v", s.Msg)

	if s.Cli.PeerId == "" {
		log.Warnf("PeerId is not valid")
		return
	}

	/*
	 判断对等端是不是本地的，如果不是查询redis发送rpc
	 */
	toPeerId := s.Msg.ToPeerId
	if s.Cli.HasNotFoundOrRejectPeer(toPeerId) {
		return
	}
	signalResp := SignalResp{
		Action: "signal",
		FromPeerId: s.Cli.PeerId,
		Data: s.Msg.Data,
	}
	if target, ok := hub.GetClient(toPeerId); ok {
		//log.Infof("SendJsonToClient %s", toPeerId)
		if err, fatal := hub.SendJsonToClient(target, signalResp); err != nil {
			log.Warnf("%s send signal to peer %s error %s", s.Cli.PeerId, target.PeerId, err)
			if !fatal {
				s.handlePeerNotFound(toPeerId)
			}
		}
		return
	}

	if addr, err := redis.GetRemotePeerRpcAddr(toPeerId); err == nil {
		node, ok := rpcservice.GetNode(addr)
		if ok {
			if !node.IsAlive() {
				log.Warnf("node %s is not alive when send signal", node.Addr())
				s.handlePeerNotFound(toPeerId)
				return
			}

			b, err := json.Marshal(signalResp)
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
				s.handlePeerNotFound(toPeerId)
				return
			}
			if !resp.Success {
				//log.Warnf("SendMsgSignal failed reason " + resp.Reason)
				log.Warnf(resp.Reason)
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


