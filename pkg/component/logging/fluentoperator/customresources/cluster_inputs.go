// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package customresources

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2input "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/input"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// GetClusterInputs Returns the ClusterInputs used by the Fluent Operator.
func GetClusterInputs(labels map[string]string) []*fluentbitv1alpha2.ClusterInput {
	return []*fluentbitv1alpha2.ClusterInput{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "tail-kubernetes",
				Labels: labels,
			},
			Spec: fluentbitv1alpha2.InputSpec{
				Tail: &fluentbitv1alpha2input.Tail{
					Tag:                    "kubernetes.*",
					Path:                   "/var/log/containers/*.log",
					ExcludePath:            "*_garden_fluent-bit-*.log,*_garden_vali-*.log",
					RefreshIntervalSeconds: pointer.Int64(10),
					MemBufLimit:            "30MB",
					SkipLongLines:          pointer.Bool(true),
					DB:                     "/var/fluentbit/flb_kube.db",
					IgnoreOlder:            "30m",
				},
			},
		},
	}
}
