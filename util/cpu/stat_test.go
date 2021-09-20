package cpu

import (
	"sync/atomic"
	"testing"
	"time"

	//"github.com/stretchr/testify/assert"
)

var (
	gCPU  int64
	decay = 0.95
)

func TestStat(t *testing.T) {
	time.Sleep(time.Second * 2)
	var s Stat
	var i Info
	ReadStat(&s)
	i = GetInfo()

	t.Log("Usage")
	t.Log(s.Usage)
	t.Log("Frequency")
	t.Log(i.Frequency)
	t.Log("Quota")
	t.Log(i.Quota)                // 核心数

	//assert.NotZero(t, s.Usage)
	//assert.NotZero(t, i.Frequency)
	//assert.NotZero(t, i.Quota)
}

func TestCPUPercent(t *testing.T)  {
	go Cpuproc()
	for {
		time.Sleep(time.Second)
		//p, err := cpu.Percent(500*time.Millisecond, true)
		//if err != nil {
		//	t.Error(err)
		//	continue
		//}
		//t.Log(len(p))
		//for _, v := range p {
		//	t.Log(v)
		//	t.Log("\n")
		//}
		t.Log(gCPU)                 // 阈值800
	}
}

func Cpuproc() {
	ticker := time.NewTicker(time.Millisecond * 500) // same to cpu sample rate
	defer func() {
		ticker.Stop()
		if err := recover(); err != nil {
			go Cpuproc()
		}
	}()

	// EMA algorithm: https://blog.csdn.net/m0_38106113/article/details/81542863
	for range ticker.C {
		stat := &Stat{}
		ReadStat(stat)
		prevCPU := atomic.LoadInt64(&gCPU)
		curCPU := int64(float64(prevCPU)*decay + float64(stat.Usage)*(1.0-decay))
		atomic.StoreInt64(&gCPU, curCPU)
	}
}
