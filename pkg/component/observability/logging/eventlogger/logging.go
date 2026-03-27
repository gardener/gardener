// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package eventlogger

import (
	"fmt"

	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2filter "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/filter"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
)

// CentralLoggingConfiguration returns a fluent-bit parser and filter for the event-logger logs.
func CentralLoggingConfiguration() (component.CentralLoggingConfig, error) {
	return component.CentralLoggingConfig{Filters: generateClusterFilters()}, nil
}

func generateClusterFilters() []*fluentbitv1alpha2.ClusterFilter {
	return []*fluentbitv1alpha2.ClusterFilter{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   v1beta1constants.DeploymentNameEventLogger,
				Labels: map[string]string{v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource},
			},
			Spec: fluentbitv1alpha2.FilterSpec{
				Match: fmt.Sprintf("kubernetes.*%s*%s*", v1beta1constants.DeploymentNameEventLogger, name),
				FilterItems: []fluentbitv1alpha2.FilterItem{
					{
						Lua: &fluentbitv1alpha2filter.Lua{
							Script: corev1.ConfigMapKeySelector{
								Key: "stringify_records_nest_log.lua",
								LocalObjectReference: corev1.LocalObjectReference{
									Name: v1beta1constants.ConfigMapNameFluentBitLua,
								},
							},
							Call: "stringify_records_nest_log",
						},
					},
					{
						RecordModifier: &fluentbitv1alpha2filter.RecordModifier{
							Records: []string{"job event-logging"},
						},
					},
				},
			},
		},
	}
}
