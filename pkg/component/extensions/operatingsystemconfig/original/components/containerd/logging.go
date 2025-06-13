// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2filter "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/filter"
	fluentbitv1alpha2input "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/input"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
)

var (
	journaldServiceName = "journald-containerd"
)

// CentralLoggingConfiguration returns a fluent-bit parser and filter for the containerd logs.
func CentralLoggingConfiguration() (component.CentralLoggingConfig, error) {
	return component.CentralLoggingConfig{Inputs: generateClusterInputs(), Filters: generateClusterFilters()}, nil
}

func generateClusterInputs() []*fluentbitv1alpha2.ClusterInput {
	return []*fluentbitv1alpha2.ClusterInput{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   journaldServiceName,
				Labels: map[string]string{v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource},
			},
			Spec: fluentbitv1alpha2.InputSpec{
				Systemd: &fluentbitv1alpha2input.Systemd{
					Tag:           "journald.containerd",
					ReadFromTail:  "on",
					SystemdFilter: []string{"_SYSTEMD_UNIT=containerd.service"},
				},
			},
		},
	}
}

func generateClusterFilters() []*fluentbitv1alpha2.ClusterFilter {
	return []*fluentbitv1alpha2.ClusterFilter{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   journaldServiceName,
				Labels: map[string]string{v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource},
			},
			Spec: fluentbitv1alpha2.FilterSpec{
				Match: "journald.containerd",
				FilterItems: []fluentbitv1alpha2.FilterItem{
					{
						RecordModifier: &fluentbitv1alpha2filter.RecordModifier{
							Records: []string{"hostname ${NODE_NAME}", "unit containerd"},
						},
					},
				},
			},
		},
	}
}
