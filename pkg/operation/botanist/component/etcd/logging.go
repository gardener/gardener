// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package etcd

import (
	"fmt"

	fluentbitv1alpha2 "github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2filter "github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2/plugins/filter"
	fluentbitv1alpha2parser "github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2/plugins/parser"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
)

// CentralLoggingConfiguration returns a fluent-bit parser and filter for the etcd and backup-restore sidecar logs.
func CentralLoggingConfiguration() (component.CentralLoggingConfig, error) {
	return component.CentralLoggingConfig{Filters: generateClusterFilters(), Parsers: generateClusterParsers()}, nil
}

func generateClusterFilters() []*fluentbitv1alpha2.ClusterFilter {
	return []*fluentbitv1alpha2.ClusterFilter{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   fmt.Sprintf("%s--%s", statefulSetNamePrefix, containerNameEtcd),
				Labels: map[string]string{v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource},
			},
			Spec: fluentbitv1alpha2.FilterSpec{
				Match: fmt.Sprintf("kubernetes.*%s*%s*", statefulSetNamePrefix, containerNameEtcd),
				FilterItems: []fluentbitv1alpha2.FilterItem{
					{
						Parser: &fluentbitv1alpha2filter.Parser{
							KeyName:     "log",
							Parser:      containerNameEtcd + "-parser",
							ReserveData: pointer.Bool(true),
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   fmt.Sprintf("%s--%s", statefulSetNamePrefix, containerNameBackupRestore),
				Labels: map[string]string{v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource},
			},
			Spec: fluentbitv1alpha2.FilterSpec{
				Match: fmt.Sprintf("kubernetes.*%s*%s*", statefulSetNamePrefix, containerNameBackupRestore),
				FilterItems: []fluentbitv1alpha2.FilterItem{
					{
						Parser: &fluentbitv1alpha2filter.Parser{
							KeyName:     "log",
							Parser:      containerNameBackupRestore + "-parser",
							ReserveData: pointer.Bool(true),
						},
					},
				},
			},
		},
	}
}

func generateClusterParsers() []*fluentbitv1alpha2.ClusterParser {
	return []*fluentbitv1alpha2.ClusterParser{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   containerNameEtcd + "-parser",
				Labels: map[string]string{v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource},
			},
			Spec: fluentbitv1alpha2.ParserSpec{
				Regex: &fluentbitv1alpha2parser.Regex{
					Regex:      "^(?<time>\\d{4}-\\d{2}-\\d{2}\\s+[^ ]*)\\s+(?<severity>\\w+)\\s+\\|\\s+(?<source>[^ :]*):\\s+(?<log>.*)",
					TimeKey:    "time",
					TimeFormat: "%Y-%m-%d %H:%M:%S.%L",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   containerNameBackupRestore + "-parser",
				Labels: map[string]string{v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource},
			},
			Spec: fluentbitv1alpha2.ParserSpec{
				Regex: &fluentbitv1alpha2parser.Regex{
					Regex:      "^time=\"(?<time>\\d{4}-\\d{2}-\\d{2}T[^\"]*)\"\\s+level=(?<severity>\\w+)\\smsg=\"(?<log>.*)\"",
					TimeKey:    "time",
					TimeFormat: "%Y-%m-%dT%H:%M:%S%z",
				},
			},
		},
	}
}
