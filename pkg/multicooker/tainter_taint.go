package multicooker

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang/glog"
	"github.com/marvasgit/kubernetes-multicooker/pkg/jsonpatch"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (t *Tainter) IsNodeTainted() (bool, error) {
	node, err := t.client.CoreV1().Nodes().Get(context.TODO(), t.nodeName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	for i := range node.Spec.Taints {
		if node.Spec.Taints[i].Key == t.taintKey {
			return true, nil
		}
	}

	return false, nil
}

func (t *Tainter) IsMulticookerDisabled() (bool, error) {
	node, err := t.client.CoreV1().Nodes().Get(context.TODO(), t.nodeName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	v, found := node.Labels["multicooker.enabled"]

	if found && strings.ToLower(v) == "false" {
		return true, nil
	}

	return false, nil
}

func (t *Tainter) TaintNode(evt PressureThresholdEvent) error {
	node, err := t.client.CoreV1().Nodes().Get(context.TODO(), t.nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	nodeCopy := node.DeepCopy()

	if nodeCopy.Spec.Taints == nil {
		nodeCopy.Spec.Taints = make([]v1.Taint, 0, 1)
	}

	for i := range nodeCopy.Spec.Taints {
		if nodeCopy.Spec.Taints[i].Key == t.taintKey {
			glog.Infof("wanted to taint node %s, but taint already exists", nodeCopy.Name)
			return nil
		}
	}

	nodeCopy.Spec.Taints = append(nodeCopy.Spec.Taints, v1.Taint{
		Key:    t.taintKey,
		Value:  "true",
		Effect: t.taintEffect,
	})

	_, err = t.client.CoreV1().Nodes().Update(context.TODO(), nodeCopy, metav1.UpdateOptions{})

	t.recorder.Eventf(t.nodeRef, v1.EventTypeWarning, t.exceededReason, t.exceededMessage, evt.MeticValue)

	if err != nil {
		t.recorder.Eventf(t.nodeRef, v1.EventTypeWarning, "NodePatchError", "could not patch node: %s", err.Error())
		return err
	}

	return nil
}

func (t *Tainter) UntaintNode(evt PressureThresholdEvent) error {
	node, err := t.client.CoreV1().Nodes().Get(context.TODO(), t.nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	taintIndex := -1

	for i, taint := range node.Spec.Taints {
		if taint.Key == t.taintKey {
			taintIndex = i
			break
		}
	}

	if taintIndex == -1 {
		glog.Infof("wanted to remove taint from node %s, but taint was already gone", node.Name)
		return nil
	}

	t.recorder.Eventf(t.nodeRef, v1.EventTypeNormal, t.recoveredReason, t.recoveredMessage, evt.MeticValue)

	_, err = t.client.CoreV1().Nodes().Patch(context.TODO(), t.nodeName, types.JSONPatchType, jsonpatch.PatchList{{
		Op:    "test",
		Path:  fmt.Sprintf("/spec/taints/%d/key", taintIndex),
		Value: t.taintKey,
	}, {
		Op:    "remove",
		Path:  fmt.Sprintf("/spec/taints/%d", taintIndex),
		Value: "",
	}}.ToJSON(), metav1.PatchOptions{})

	if err != nil {
		t.recorder.Eventf(t.nodeRef, v1.EventTypeWarning, "NodePatchError", "could not patch node: %s", err.Error())
		return err
	}

	return nil
}
