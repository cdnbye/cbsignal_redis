package handler

import (
	"cbsignal/client"
	"github.com/bytedance/sonic"
)

type Handler interface {
	Handle()
}

type SignalMsg struct {
	Action   string      `json:"action"`
	ToPeerId string      `json:"to_peer_id"`
	Data     interface{} `json:"data"`
	Reason   string      `json:"reason"`
}

func NewHandler(message []byte, cli *client.Client) (Handler, error) {
	signal := SignalMsg{}
	if err := sonic.Unmarshal(message, &signal); err != nil {
		//log.Println(err)
		return nil, err
	}
	return NewHandlerMsg(signal, cli)
}

func NewHandlerMsg(signal SignalMsg, cli *client.Client) (Handler, error) {
	switch signal.Action {
	case "signal":
		return &SignalHandler{Msg: &signal, Cli: cli}, nil
	case "ping":
		return &HeartbeatHandler{Cli: cli}, nil
	case "reject":
		return &RejectHandler{Msg: &signal, Cli: cli}, nil
	default:
		return &ExceptionHandler{Msg: &signal, Cli: cli}, nil
	}
}
