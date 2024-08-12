// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

const gardenSubsystem = "garden"

type gardenCollector struct {
	runtimeClient client.Reader
	log           logr.Logger

	gardenConditions *prometheus.Desc
}

func newGardenCollector(k8sClient client.Reader, log logr.Logger) *gardenCollector {
	c := &gardenCollector{
		runtimeClient: k8sClient,
		log:           log,
	}
	c.setMetricDefinitions()
	return c
}

func (c *gardenCollector) setMetricDefinitions() {
	c.gardenConditions = prometheus.NewDesc(
		prometheus.BuildFQName(metricPrefix, gardenSubsystem, "condition"),
		"Condition state of the Garden. Possible values: -1=Unknown|0=Unhealthy|1=Healthy|2=Progressing",
		[]string{
			"name",
			"condition",
			"operation",
		},
		nil,
	)
}

func (c *gardenCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.gardenConditions
}

func (c *gardenCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	gardenList := &operatorv1alpha1.GardenList{}
	if err := c.runtimeClient.List(ctx, gardenList); err != nil {
		c.log.Error(err, "Failed to list gardens")
		return
	}

	for i := range gardenList.Items {
		garden := gardenList.Items[i]
		if garden.Status.LastOperation == nil {
			continue
		}
		lastOperation := string(garden.Status.LastOperation.Type)

		for i := range garden.Status.Conditions {
			condition := garden.Status.Conditions[i]
			if condition.Type == "" {
				continue
			}
			ch <- prometheus.MustNewConstMetric(
				c.gardenConditions,
				prometheus.GaugeValue,
				mapConditionStatus(condition.Status),
				[]string{
					garden.Name,
					string(condition.Type),
					lastOperation,
				}...,
			)
		}
	}
}
