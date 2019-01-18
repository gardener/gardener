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
)

var (
	collector          controllerCollector
	metricsInitialized bool

	// ControllerWorkerSum is a metric descriptor which collects the current amount of workers per controller.
	ControllerWorkerSum = prometheus.NewDesc("garden_cm_worker_amount", "Count of currently running controller workers", []string{"controller"}, nil)

	// ScrapeFailures is a metric descriptor which counts the amount scrape issues grouped by kind.
	ScrapeFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "garden_scrape_failure_total",
		Help: "Total count of scraping failures, grouped by kind/group of metric(s)",
	}, []string{"kind"})
)

// RegisterControllerMetrics initializes the collection of Controller related metrics.
// This function ensures to run only once for avoiding multiple controller registration.
func RegisterControllerMetrics(controllers ...ControllerMetricsCollector) {
	if metricsInitialized {
		panic("Controller Manager metrics are already initialized")
	}

	// Register scrape failure metric.
	prometheus.MustRegister(ScrapeFailures)

	// Create a controllerCollector, pass the metrics descriptors for metrics which should be registered
	// and the collectors which should collect the metrics. At the end register the collector.
	collector = controllerCollector{
		controllers: controllers,
		metricDescs: []*prometheus.Desc{ControllerWorkerSum},
	}
	prometheus.MustRegister(collector)

	metricsInitialized = true
}

// ControllerMetricsCollector is an interface implemented by any controller
// bundled in the Gardener Controller Manager which wants to expose metrics.
type ControllerMetricsCollector interface {
	// CollectMetrics is called by the controllerCollector when collecting metrics.
	// The implemtation sends the collected metrics to the given channel.
	CollectMetrics(ch chan<- prometheus.Metric)
}

type controllerCollector struct {
	controllers []ControllerMetricsCollector
	metricDescs []*prometheus.Desc
}

// Describe is required to implement by the prometheus.Collector interface
// which is used to implement a Prometheus collector. Describe will be invoked
// when the collctor gets registered and is used to register metric descriptors
// for the metrics which should be collected by the collector.
func (c controllerCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.metricDescs {
		ch <- desc
	}
}

// Collect is required to implement by the prometheus.Collector interface
// which is used to implement a Prometheus collector. Collect will be invoked
// when the metrics endpoint of the app is called and is used to ask the
// registered controllers to expose the metrics for the registered descriptors.
func (c controllerCollector) Collect(ch chan<- prometheus.Metric) {
	for _, controller := range c.controllers {
		controller.CollectMetrics(ch)
	}
}
