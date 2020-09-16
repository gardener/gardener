// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	collector          controllerCollector
	metricsInitialized bool
)

// NewMetricDescriptor returns a new metric descriptor.
func NewMetricDescriptor(name, description string) *prometheus.Desc {
	return prometheus.NewDesc(name, description, []string{"controller"}, nil)
}

// NewCounterVec returns a new counter vector.
func NewCounterVec(name, help string) *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: name,
		Help: help,
	}, []string{"kind"})
}

// RegisterControllerMetrics initializes the collection of Controller related metrics.
// This function ensures to run only once for avoiding multiple controller registration.
func RegisterControllerMetrics(metricsDesc *prometheus.Desc, scrapeFailureMetric *prometheus.CounterVec, controllers ...ControllerMetricsCollector) {
	if metricsInitialized {
		panic("Controller Manager metrics are already initialized")
	}

	// Register scrape failure metric.
	prometheus.MustRegister(scrapeFailureMetric)

	// Create a controllerCollector, pass the metrics descriptors for metrics which should be registered
	// and the collectors which should collect the metrics. At the end register the collector.
	collector = controllerCollector{
		controllers: controllers,
		metricDescs: []*prometheus.Desc{metricsDesc},
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
// when the collector gets registered and is used to register metric descriptors
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
