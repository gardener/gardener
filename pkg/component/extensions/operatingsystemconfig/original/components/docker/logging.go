// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package docker

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2filter "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/filter"
	fluentbitv1alpha2input "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/input"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	comp "github.com/gardener/gardener/pkg/component"
)

var (
	journaldServiceName = "journald-docker"
)

// CentralLoggingConfiguration returns a fluent-bit parser and filter for the docker logs.
func CentralLoggingConfiguration() (comp.CentralLoggingConfig, error) {
	return comp.CentralLoggingConfig{Inputs: generateClusterInputs(), Filters: generateClusterFilters()}, nil
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
					Tag:           "journald.docker",
					ReadFromTail:  "on",
					SystemdFilter: []string{"_SYSTEMD_UNIT=docker.service"},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   journaldServiceName + "-monitor",
				Labels: map[string]string{v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource},
			},
			Spec: fluentbitv1alpha2.InputSpec{
				Systemd: &fluentbitv1alpha2input.Systemd{
					Tag:           "journald.docker-monitor",
					ReadFromTail:  "on",
					SystemdFilter: []string{"_SYSTEMD_UNIT=docker-monitor.service"},
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
				Match: "journald.docker",
				FilterItems: []fluentbitv1alpha2.FilterItem{
					{
						RecordModifier: &fluentbitv1alpha2filter.RecordModifier{
							Records: []string{"hostname ${NODE_NAME}", "unit docker"},
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   journaldServiceName + "-monitor",
				Labels: map[string]string{v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource},
			},
			Spec: fluentbitv1alpha2.FilterSpec{
				Match: "journald.docker-monitor",
				FilterItems: []fluentbitv1alpha2.FilterItem{
					{
						RecordModifier: &fluentbitv1alpha2filter.RecordModifier{
							Records: []string{"hostname ${NODE_NAME}", "unit docker-monitor"},
						},
					},
				},
			},
		},
	}
}
