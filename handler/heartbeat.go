package handler

import (
	"cbsignal/client"
	"cbsignal/hub"
	"cbsignal/redis"
	"github.com/lexkong/log"
)

type HeartbeatHandler struct {
	Cli   *client.Client
}

func (s *HeartbeatHandler)Handle() {

	log.Infof("receive heartbeat from %s", s.Cli.PeerId)
	s.Cli.UpdateTs()

	resp := SignalResp{
		Action: "pong",
	}
	hub.SendJsonToClient(s.Cli, resp)

	/*
	更新redis节点过期时间
	 */
	if err := redis.UpdateLocalPeerExpiration(s.Cli.PeerId); err != nil {
		log.Error("UpdateLocalPeerExpiration", err)
	}
}
