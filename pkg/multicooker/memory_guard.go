package multicooker

import (
	"fmt"

	"github.com/prometheus/procfs"
)

const MemoryTaintKey = "multicooker/memory-pressure"

// MemoryGuard watches the node's available memory (MemAvailable / MemTotal
// from /proc/meminfo). Thresholds are percentages of available memory, so
// lower values mean more pressure - the inverse of the CPU thresholds.
type MemoryGuard struct {
	TaintThreshold float64
	EvictThreshold float64

	proc procfs.FS
}

func NewMemoryGuard(taintThreshold, evictThreshold float64) (*MemoryGuard, error) {
	if taintThreshold <= 0 || taintThreshold >= 100 {
		return nil, fmt.Errorf("memory taint threshold must be between 0 and 100, got %.2f", taintThreshold)
	}
	if evictThreshold < 0 || evictThreshold > taintThreshold {
		return nil, fmt.Errorf("memory evict threshold must be between 0 and the taint threshold (%.2f), got %.2f", taintThreshold, evictThreshold)
	}

	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return nil, err
	}

	return &MemoryGuard{
		TaintThreshold: taintThreshold,
		EvictThreshold: evictThreshold,
		proc:           fs,
	}, nil
}

func (g *MemoryGuard) Check() (PressureThresholdEvent, error) {
	mi, err := g.proc.Meminfo()
	if err != nil {
		return PressureThresholdEvent{}, fmt.Errorf("failed to read meminfo: %w", err)
	}

	if mi.MemTotal == nil || mi.MemAvailable == nil || *mi.MemTotal == 0 {
		return PressureThresholdEvent{}, fmt.Errorf("could not read MemTotal/MemAvailable from meminfo")
	}

	return g.evaluate(float64(*mi.MemAvailable) / float64(*mi.MemTotal) * 100), nil
}

func (g *MemoryGuard) evaluate(availablePercent float64) PressureThresholdEvent {
	isHigh := availablePercent < g.TaintThreshold

	action := "is above"
	if isHigh {
		action = "dropped below"
	}

	return PressureThresholdEvent{
		Message:         fmt.Sprintf("Memory available=(%.2f%%) %s the threshold=(%.2f%%)", availablePercent, action, g.TaintThreshold),
		MeticValue:      availablePercent,
		IsCurrentlyHigh: isHigh,
		Resource:        ResourceMemory,
	}
}

// ShouldEvict reports whether available memory is low enough to evict a pod; an evict threshold of 0 disables eviction.
func (g *MemoryGuard) ShouldEvict(evt PressureThresholdEvent) bool {
	return g.EvictThreshold > 0 && evt.MeticValue < g.EvictThreshold
}
