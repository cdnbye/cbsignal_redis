package handler

import (
	"cbsignal/client"
	"cbsignal/util/ecache"
	"github.com/bytedance/sonic"
	"sync"
)

var (
	filter *ecache.Cache
)

//Initialing pool
var signalMsgPool = sync.Pool{
	New: func() interface{} {
		return &SignalHandler{}
	},
}

func init() {
	filter = ecache.NewLRUCache(16, 300, 0)
}

func keyForFilter(from, to string) string {
	return from + to
}

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
		hdr := signalMsgPool.Get().(*SignalHandler)
		hdr.Msg = signal
		hdr.Cli = cli
		return hdr, nil
	case "ping":
		return &HeartbeatHandler{Cli: cli}, nil
	case "reject":
		return &RejectHandler{Msg: signal, Cli: cli}, nil
	default:
		return &ExceptionHandler{Msg: signal, Cli: cli}, nil
	}
}
