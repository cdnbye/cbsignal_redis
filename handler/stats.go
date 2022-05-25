package handler

import (
	"cbsignal/hub"
	"cbsignal/redis"
	"cbsignal/rpcservice"
	"cbsignal/util/cpu"
	"fmt"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"
)

const (
	HEALTH_CHECK_CPU_LIMIT = 60
)

type SignalInfo struct {
	Version string `json:"version"`
	CurrentConnections int `json:"current_connections"`
	TotalConnections int `json:"total_connections"`
	NumInstance int `json:"num_instance"`
	RateLimit          int64  `json:"rate_limit,omitempty"`
	SecurityEnabled    bool `json:"security_enabled,omitempty"`
	NumGoroutine       int  `json:"num_goroutine"`
	NumPerMap          []int `json:"num_per_map"`
	CpuUsage           int64  `json:"cpu_usage"`
	RedisConnected     bool  `json:"redis_connected"`
}

type Resp struct {
	Ret int `json:"ret"`
	Data *SignalInfo `json:"data"`
}

var (
	G_CPU  int64
	decay      = 0.7
	StatsToken string
)

func init() {
	// 监控cpu使用率
	go cpuproc()

}

func HealthCheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cpuUsage := atomic.LoadInt64(&G_CPU)/10
		if cpuUsage >= HEALTH_CHECK_CPU_LIMIT {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(fmt.Sprintf("service overloaded, cpu %d", cpuUsage)))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("service normal, cpu %d", cpuUsage)))
	}
}

func StatsHandler(info SignalInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !checkStatsToken(r) {
			w.WriteHeader(403)
			return
		}
		info.NumGoroutine = runtime.NumGoroutine()
		info.NumPerMap = hub.GetClientNumPerMap()
		if redis.IsAlive {
			info.RedisConnected = true
			info.CurrentConnections = hub.GetClientNum()
		} else {
			info.RedisConnected = false
			info.CurrentConnections = 0
		}
		info.TotalConnections = info.CurrentConnections + rpcservice.GetTotalNumClient()
		info.NumInstance = rpcservice.GetNumNode() + 1
		info.CpuUsage = atomic.LoadInt64(&G_CPU)/10
		w.Header().Set("Access-Control-Allow-Origin", "*")
		resp := Resp{
			Ret:  0,
			Data: &info,
		}
		b, err := json.MarshalIndent(resp, "", "   ")
		if err != nil {
			resp, _ := json.Marshal(Resp{
				Ret:  -1,
				Data: nil,
			})
			w.Write(resp)
			return
		}
		w.Write(b)
	}
}

func VersionHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !checkStatsToken(r) {
			w.WriteHeader(403)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write([]byte(fmt.Sprintf("%s", version)))

	}
}

func CountHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !checkStatsToken(r) {
			w.WriteHeader(403)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if redis.IsAlive {
			w.Write([]byte(fmt.Sprintf("%d", hub.GetClientNum())))
		} else {
			w.Write([]byte(fmt.Sprintf("0")))
		}

	}
}

func TotalCountHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !checkStatsToken(r) {
			w.WriteHeader(403)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write([]byte(fmt.Sprintf("%d", hub.GetClientNum() + rpcservice.GetTotalNumClient())))
	}
}

func checkStatsToken(r *http.Request) bool {
	if StatsToken == "" {
		return true
	}
	return r.URL.Query().Get("token") == StatsToken
}

func cpuproc() {
	ticker := time.NewTicker(time.Millisecond * 500) // same to cpu sample rate
	defer func() {
		ticker.Stop()
		if err := recover(); err != nil {
			go cpuproc()
		}
	}()

	// EMA algorithm: https://blog.csdn.net/m0_38106113/article/details/81542863
	for range ticker.C {
		stat := &cpu.Stat{}
		cpu.ReadStat(stat)
		prevCPU := atomic.LoadInt64(&G_CPU)
		curCPU := int64(float64(prevCPU)*decay + float64(stat.Usage)*(1.0-decay))
		atomic.StoreInt64(&G_CPU, curCPU)
		//log.Warnf("gCPU %d", gCPU)
	}
}
