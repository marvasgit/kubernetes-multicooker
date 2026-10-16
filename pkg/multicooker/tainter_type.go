package multicooker

import (
	"github.com/golang/glog"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

const ComponentName = "multicooker"

const TaintKey = "multicooker/load-exceeded"

type Tainter struct {
	client   kubernetes.Interface
	recorder record.EventRecorder
	nodeName string
	nodeRef  *v1.ObjectReference

	taintKey         string
	taintEffect      v1.TaintEffect
	exceededReason   string
	exceededMessage  string
	recoveredReason  string
	recoveredMessage string
}

// NewTainter manages the CPU taint (TaintKey).
func NewTainter(c kubernetes.Interface, nodeName string) (*Tainter, error) {
	t := newTainter(c, nodeName)
	t.taintKey = TaintKey
	t.taintEffect = v1.TaintEffectPreferNoSchedule
	t.exceededReason = "CPUPressureExceeded"
	t.exceededMessage = "pressure over 5 minutes on node was %.2f, tainting node"
	t.recoveredReason = "LoadThresholdDeceeded"
	t.recoveredMessage = "pressure on node was %.2f over 5 minutes. untainting node"
	return t, nil
}

// NewMemoryTainter manages the memory taint (MemoryTaintKey).
func NewMemoryTainter(c kubernetes.Interface, nodeName string) (*Tainter, error) {
	t := newTainter(c, nodeName)
	t.taintKey = MemoryTaintKey
	t.taintEffect = v1.TaintEffectNoSchedule
	t.exceededReason = "MemoryPressureExceeded"
	t.exceededMessage = "available memory on node is %.2f%%, tainting node"
	t.recoveredReason = "MemoryPressureRecovered"
	t.recoveredMessage = "available memory on node recovered to %.2f%%. untainting node"
	return t, nil
}

func newTainter(c kubernetes.Interface, nodeName string) *Tainter {
	b := record.NewBroadcaster()
	b.StartLogging(glog.Infof)
	b.StartRecordingToSink(&typedv1.EventSinkImpl{
		Interface: c.CoreV1().Events(""),
	})

	r := b.NewRecorder(scheme.Scheme, v1.EventSource{Host: nodeName, Component: ComponentName + "/tainter"})

	nodeRef := &v1.ObjectReference{
		Kind: "Node",
		Name: nodeName,
		UID:  types.UID(nodeName),
	}

	return &Tainter{
		client:   c,
		recorder: r,
		nodeName: nodeName,
		nodeRef:  nodeRef,
	}
}
