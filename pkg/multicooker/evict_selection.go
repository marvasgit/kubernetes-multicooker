package multicooker

import (
	"math"
	"sort"
	"time"

	"github.com/golang/glog"
	v1 "k8s.io/api/core/v1"
)

type PodCandidateSet []PodCandidate

func (s PodCandidateSet) Len() int {
	return len(s)
}

func (s PodCandidateSet) Less(i, j int) bool {
	return s[i].Score < s[j].Score
}

func (s PodCandidateSet) Swap(i, j int) {
	x := s[i]
	s[i] = s[j]
	s[j] = x
}

type PodCandidate struct {
	Pod   *v1.Pod
	Score int
}

func PodCandidateSetFromPodList(l *v1.PodList) PodCandidateSet {
	s := make(PodCandidateSet, len(l.Items))

	for i := range l.Items {
		s[i] = PodCandidate{
			Pod:   &l.Items[i],
			Score: 0,
		}
	}

	return s
}

func (s PodCandidateSet) scoreByQOSClass() {
	for i := range s {
		switch s[i].Pod.Status.QOSClass {
		case v1.PodQOSBestEffort:
			s[i].Score += 100
		case v1.PodQOSBurstable:
			s[i].Score += 100
		case v1.PodQOSGuaranteed:
			s[i].Score -= 10000
		}
	}
}

func (s PodCandidateSet) scoreByAge(minPodAge time.Duration) {
	now := time.Now()
	for i, pod := range s {
		if pod.Pod.Status.StartTime == nil {
			s[i].Score -= 10000
			continue
		}
		delta := now.Sub(pod.Pod.Status.StartTime.Time)
		if delta < minPodAge {
			s[i].Score -= 10000
			continue
		}
		age := int64(delta / time.Second)
		if age < 1 {
			age = 1
		}
		s[i].Score += int(math.Floor(math.Log1p(float64(age))))
	}
}

func (s PodCandidateSet) scoreByOwnerType() {
	for i := range s {
		// do not evict Pods without owner; these will probably not be re-scheduled if evicted
		if len(s[i].Pod.OwnerReferences) == 0 {
			s[i].Score -= 1000
		}

		for j := range s[i].Pod.OwnerReferences {
			o := &s[i].Pod.OwnerReferences[j]

			switch o.Kind {
			case "ReplicaSet":
				s[i].Score += 100
			case "StatefulSet":
				s[i].Score -= 10000
			case "DaemonSet":
				s[i].Score -= 10000
			}
		}
	}
}

func (s PodCandidateSet) scoreByCriticality() {
	for i := range s {
		if s[i].Pod.Namespace == "kube-system" {
			s[i].Score -= 10000
		}

		switch s[i].Pod.Spec.PriorityClassName {
		case "system-cluster-critical":
			s[i].Score -= 10000
		case "system-node-critical":
			s[i].Score -= 10000
		}

		if _, ok := s[i].Pod.Annotations["scheduler.alpha.kubernetes.io/critical-pod"]; ok {
			s[i].Score -= 10000
		}
	}
}

// scoreByMemoryLimit prefers pods that can grow without bound: a container
// without a memory limit is the likeliest source of node memory pressure.
func (s PodCandidateSet) scoreByMemoryLimit() {
	for i := range s {
		for _, c := range s[i].Pod.Spec.Containers {
			if _, ok := c.Resources.Limits[v1.ResourceMemory]; !ok {
				s[i].Score += 50
				break
			}
		}
	}
}

func (s PodCandidateSet) SelectPodForEviction(minPodAge time.Duration, resource string) *v1.Pod {
	s.scoreByAge(minPodAge)
	s.scoreByQOSClass()
	if resource == ResourceMemory {
		s.scoreByMemoryLimit()
	}
	s.scoreByOwnerType()
	s.scoreByCriticality()

	sort.Stable(sort.Reverse(s))

	for i := range s {
		glog.Infof("eviction candidate: %s/%s (score of %d)", s[i].Pod.Namespace, s[i].Pod.Name, s[i].Score)
	}

	for i := range s {
		if s[i].Score < 0 {
			continue
		}

		glog.Infof("selected candidate: %s/%s (score of %d)", s[i].Pod.Namespace, s[i].Pod.Name, s[i].Score)
		return s[i].Pod
	}

	return nil
}
