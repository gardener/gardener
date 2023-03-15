// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package eventlogger

import (
	"fmt"

	fluentbitv1alpha2 "github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2filter "github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2/plugins/filter"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
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
						Nest: &fluentbitv1alpha2filter.Nest{
							Operation:   "lift",
							NestedUnder: "log",
							
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
