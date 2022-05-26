package signaling

var (
	Token string
)

const (
	SIGNAL_SERVICE    = "SignalService"
	SIGNAL_BATCH      = ".SignalBatch"
	LOGIN             = ".Login"
	PONG              = ".Pong"
)

//type RpcResp struct {
//	Success bool
//	Reason  string
//}
//
//type SignalReq struct {
//	ToPeerId string
//	Data     []byte
//}
//
//type SignalBatchReq struct {
//	Items []SignalReq
//}
//
//type Auth struct {
//	Token string
//	From string
//}
//
//type Ping struct {
//}
//
//type Pong struct {
//	NumClient int
//}
