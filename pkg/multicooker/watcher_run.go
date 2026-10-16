package multicooker

import (
	"fmt"
	"time"

	"github.com/golang/glog"
)

func (w *Watcher) SetAsHigh(high bool) {
	w.isCurrentlyHigh = high
}

func (w *Watcher) Run(closeChan chan struct{}, useAvarage bool, targetMetic int) (<-chan PressureThresholdEvent, <-chan error) {
	var initTargetMetric int
	result := make(chan PressureThresholdEvent)
	metricsChan := make(chan [3]float64)
	errs := make(chan error)
	ticker := time.Tick(w.TickerInterval)

	if targetMetic < 1 || targetMetic > 3 {
		glog.Errorf("Target metric is not in available %d choosing midle one", targetMetic)
		targetMetic = 2
	}
	initTargetMetric = targetMetic

	go func() {
		defer func() {
			close(result)
			close(errs)
		}()
		for {
			select {
			case <-ticker:
				go w.CollectMetricsData(closeChan, useAvarage, metricsChan, errs)
				res := <-metricsChan

				if res[targetMetic-1] >= w.PressureThreshold {
					w.isCurrentlyHigh = true
					//once pressure is high we go more reactive for eviction
					targetMetic = 1
				} else if res[0] < w.PressureThreshold && res[1] < w.PressureThreshold && res[2] < w.PressureThreshold {
					w.isCurrentlyHigh = false
					targetMetic = initTargetMetric
				}

				result <- PressureThresholdEvent{
					Message:    w.generateEventMessage(useAvarage, res, targetMetic),
					MeticValue: res[targetMetic-1], IsCurrentlyHigh: w.isCurrentlyHigh, Resource: ResourceCPU}

			case <-closeChan:
				glog.Error("ClosingChan stopping")
				return
			}
		}
	}()

	return result, errs
}

func (w *Watcher) generateEventMessage(useAvarage bool, metrics [3]float64, targetMetric int) string {

	messagetype, metricName := w.metricsMessageData(targetMetric, useAvarage, metrics)

	var action string
	if w.isCurrentlyHigh {
		action = "exceeded"
	} else {
		action = "deceeded"
	}
	return fmt.Sprintf("%s Metric %s %s=(%.2f) the threshold=(%.2f)", messagetype, metricName, action, metrics[targetMetric-1], w.PressureThreshold)
}

func (w *Watcher) metricsMessageData(targetMetric int, useAvarage bool, metrics [3]float64) (string, string) {
	var messagetype string
	var metricName string
	var infoMessage string
	avaNames := [3]string{"Load1", "Load5", "Load15"}
	pressureNames := [3]string{"Avg10", "Avg60", "Avg300"}

	if useAvarage {
		messagetype = "LoadAvg"
		metricName = avaNames[targetMetric-1]
		infoMessage = generateMetricsMessage(targetMetric, metrics, avaNames, w.PressureThreshold)
	} else {
		messagetype = "Pressure"
		metricName = pressureNames[targetMetric-1]
		infoMessage = generateMetricsMessage(targetMetric, metrics, pressureNames, w.PressureThreshold)
	}

	glog.Infof("NODE: %s Current %s state:  %s", w.nodeName, messagetype, infoMessage)

	return messagetype, metricName
}

func generateMetricsMessage(targetMetic int, metrics [3]float64, names [3]string, threshold float64) string {
	var message string
	for i := 0; i < len(metrics); i++ {
		message += fmt.Sprintf(" %s |%.2f| ", names[i], metrics[i])

		if i == targetMetic-1 {
			message += fmt.Sprintf("  THRESHOLD |%.2f|", threshold)
		}
	}

	return message
}
