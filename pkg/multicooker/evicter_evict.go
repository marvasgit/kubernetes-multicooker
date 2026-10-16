package multicooker

import (
	"context"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	betav1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
)

var (
	prometheusNamespace = "multicooker"
	podsEvictedTotal    = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: prometheusNamespace,
		Name:      "pods_evicted_total",
		Help:      "total number of pods evicted on this node",
	})
)

func init() {
	prometheus.MustRegister(podsEvictedTotal)
}

func (e *Evicter) CanEvict() bool {
	if e.lastEviction.IsZero() {
		return true
	}

	evictionTime := time.Now().Sub(e.lastEviction)
	return evictionTime > e.backoff
}

func (e *Evicter) EvictPod(evt PressureThresholdEvent) (bool, error) {

	if !e.CanEvict() {
		glog.Infof("eviction threshold exceeded; still in back-off")
		return false, nil
	}

	glog.Infof("searching for pod to evict")

	fieldSelector := fields.OneTermEqualSelector("spec.nodeName", e.nodeName)

	podsOnNode, err := e.client.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
		FieldSelector: fieldSelector.String(),
	})

	if err != nil {
		return false, err
	}

	candidates := PodCandidateSetFromPodList(podsOnNode)
	podToEvict := candidates.SelectPodForEviction(e.minPodAge, evt.Resource)

	if podToEvict == nil {
		e.recorder.Eventf(e.nodeRef, corev1.EventTypeWarning, "NoPodToEvict", "wanted to evict Pod, but no suitable candidate found")
		return false, nil
	}

	eviction := betav1.Eviction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podToEvict.ObjectMeta.Name,
			Namespace: podToEvict.ObjectMeta.Namespace,
		},
	}

	podsEvictedTotal.Inc()

	glog.Infof("eviction: %+v", eviction)

	e.lastEviction = time.Now()

	reason := "EvictHighLoad"
	if evt.Resource == ResourceMemory {
		reason = "EvictLowMemory"
	}

	e.recorder.Eventf(podToEvict, corev1.EventTypeWarning, reason, "evicting pod due to %s", evt.Message)
	e.recorder.Eventf(e.nodeRef, corev1.EventTypeWarning, reason, "evicting pod due to high %s pressure on node: %s", evt.Resource, evt.Message)

	err = e.client.CoreV1().Pods(podToEvict.Namespace).Evict(context.TODO(), &eviction)
	return true, err
}
