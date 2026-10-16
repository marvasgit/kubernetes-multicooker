package config

type StartupFlags struct {
	KubeConfig     string
	TaintThreshold float64
	EvictThreshold float64
	EvictBackoff   string
	MinPodAge      string
	NodeName       string
	MetricsPort    int
	TargetMetric   int
	UseAvarage     bool

	MemoryTaintThreshold float64
	MemoryEvictThreshold float64
}
