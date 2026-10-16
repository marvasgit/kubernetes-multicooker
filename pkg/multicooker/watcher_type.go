package multicooker

import (
	"time"

	"github.com/prometheus/procfs"
)

const (
	ResourceCPU    = "cpu"
	ResourceMemory = "memory"
)

type PressureThresholdEvent struct {
	Message         string
	MeticValue      float64
	IsCurrentlyHigh bool
	Resource        string
}

type Watcher struct {
	TickerInterval    time.Duration
	PressureThreshold float64

	proc            procfs.FS
	isCurrentlyHigh bool
	nodeName        string
}

func NewWatcher(pressureThreshold float64, nodeName string) (*Watcher, error) {
	if pressureThreshold == 0 {
		pressureThreshold = 25
	}

	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		PressureThreshold: pressureThreshold,
		TickerInterval:    15 * time.Second,
		proc:              fs,
		nodeName:          nodeName,
	}, nil
}
