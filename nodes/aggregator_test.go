package nodes

import (
	message "cbsignal/protobuf"
	"fmt"
	"testing"
	"time"
)

func TestAggregator(t *testing.T) {
	batchProcess := func(items []*message.SignalReq) error {
		t.Logf("handler %d items", len(items))
		return nil
	}

	errorHandler := func(err error, items []*message.SignalReq, batchProcessFunc BatchProcessFunc, aggregator *Aggregator) {
		if err == nil {
			t.FailNow()
		}
		t.Logf("Receive error, item size is %d", len(items))
	}

	aggregator := NewAggregator(batchProcess, func(option AggregatorOption) AggregatorOption {
		option.BatchSize = 20
		option.Workers = 1
		option.LingerTime = 10 * time.Millisecond
		option.ErrorHandler = errorHandler
		return option
	})

	aggregator.Start()

	for i := 0; i < 1000; i++ {
		m := &message.SignalReq{
			ToPeerId: fmt.Sprintf("%d", i),
		}
		aggregator.TryEnqueue(m)
		time.Sleep(5*time.Millisecond)
	}

	//aggregator.SafeStop()
	//time.Sleep(1*time.Minute)
}
