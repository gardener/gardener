// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/util/workqueue"
)

var (
	workqueueLabels = []string{"queue"}
	workqueueLength = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "garden_cm_workqueue_items_total",
		Help: "Current count of item in the workqueue.",
	}, workqueueLabels)

	workqueueAdds = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "garden_cm_workqueue_items_adds_total",
		Help: "Count of item additions to a workqueue.",
	}, workqueueLabels)

	workqueueLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "garden_cm_workqueue_latency_milliseconds",
		Help: "Time in milliseconds an item remains in the workqueue before it get processed.",
	}, workqueueLabels)

	deprecatedWorkqueueLatency = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: "garden_cm_workqueue_latency_milliseconds_deprecated",
		Help: "Time in milliseconds an item remains in the workqueue before it get processed.",
	}, workqueueLabels)

	workqueueDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "garden_cm_workqueue_duration_milliseconds",
		Help: "Processing duration in milliseconds of an workqueue item.",
	}, workqueueLabels)

	deprecatedWorkqueueDuration = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: "garden_cm_workqueue_duration_milliseconds_deprecated",
		Help: "Processing duration in milliseconds of an workqueue item.",
	}, workqueueLabels)

	workqueueRetries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "garden_cm_workqueue_retries_total",
		Help: "Count of item processing retries in a workqueue.",
	}, workqueueLabels)

	workqueueUnfinishedWork = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "garden_cm_workqueue_unfinishedwork_seconds",
		Help: "Unfinished work in seconds.",
	}, workqueueLabels)

	workqueueLongestRunningProcessorMicroseconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "garden_cm_workqueue_longest_running_processor_microseconds",
		Help: "Longest running processor in microseconds.",
	}, workqueueLabels)
)

type workqueueMetricProvider struct{}

func (p workqueueMetricProvider) NewDepthMetric(name string) workqueue.GaugeMetric {
	return workqueueLength.With(prometheus.Labels{"queue": name})
}

func (p workqueueMetricProvider) NewDeprecatedDepthMetric(name string) workqueue.GaugeMetric {
	return p.NewDepthMetric(name)
}

func (p workqueueMetricProvider) NewAddsMetric(name string) workqueue.CounterMetric {
	return workqueueAdds.With(prometheus.Labels{"queue": name})
}

func (p workqueueMetricProvider) NewDeprecatedAddsMetric(name string) workqueue.CounterMetric {
	return p.NewAddsMetric(name)
}

func (p workqueueMetricProvider) NewLatencyMetric(name string) workqueue.HistogramMetric {
	return workqueueLatency.With(prometheus.Labels{"queue": name})
}

func (p workqueueMetricProvider) NewDeprecatedLatencyMetric(name string) workqueue.SummaryMetric {
	return deprecatedWorkqueueLatency.With(prometheus.Labels{"queue": name})
}

func (p workqueueMetricProvider) NewWorkDurationMetric(name string) workqueue.HistogramMetric {
	return workqueueDuration.With(prometheus.Labels{"queue": name})
}

func (p workqueueMetricProvider) NewDeprecatedWorkDurationMetric(name string) workqueue.SummaryMetric {
	return deprecatedWorkqueueDuration.With(prometheus.Labels{"queue": name})
}

func (p workqueueMetricProvider) NewRetriesMetric(name string) workqueue.CounterMetric {
	return workqueueRetries.With(prometheus.Labels{"queue": name})
}

func (p workqueueMetricProvider) NewDeprecatedRetriesMetric(name string) workqueue.CounterMetric {
	return p.NewRetriesMetric(name)
}

func (p workqueueMetricProvider) NewUnfinishedWorkSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return workqueueUnfinishedWork.With(prometheus.Labels{"queue": name})
}

func (p workqueueMetricProvider) NewDeprecatedUnfinishedWorkSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return p.NewUnfinishedWorkSecondsMetric(name)
}

func (p workqueueMetricProvider) NewLongestRunningProcessorSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return workqueueLongestRunningProcessorMicroseconds.With(prometheus.Labels{"queue": name})
}

func (p workqueueMetricProvider) NewDeprecatedLongestRunningProcessorMicrosecondsMetric(name string) workqueue.SettableGaugeMetric {
	return p.NewLongestRunningProcessorSecondsMetric(name)
}

// RegisterWorkqueMetrics creates and register a provider for workqueue metrics
// which is used to map the data collected by the workqueues to the proper metrics.
// The provider needs to be registered, before creating workqueues otherwise it wouldn't be noticed.
func RegisterWorkqueMetrics() {
	prometheus.MustRegister(workqueueLength)
	prometheus.MustRegister(workqueueAdds)
	prometheus.MustRegister(workqueueLatency)
	prometheus.MustRegister(deprecatedWorkqueueLatency)
	prometheus.MustRegister(workqueueDuration)
	prometheus.MustRegister(deprecatedWorkqueueDuration)
	prometheus.MustRegister(workqueueRetries)
	prometheus.MustRegister(workqueueUnfinishedWork)
	prometheus.MustRegister(workqueueLongestRunningProcessorMicroseconds)
	workqueue.SetProvider(workqueueMetricProvider{})
}
