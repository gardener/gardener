// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extauthzserver

import (
	"fmt"

	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2filter "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/filter"
	fluentbitv1alpha2parser "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/parser"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
)

// CentralLoggingConfiguration returns a fluent-bit parser and filter for the ext-authz-server logs in seed/shoot.
func CentralLoggingConfiguration() (component.CentralLoggingConfig, error) {
	return component.CentralLoggingConfig{Filters: generateClusterFilters(""), Parsers: generateClusterParsers("")}, nil
}

// CentralLoggingConfigurationForGarden returns a fluent-bit parser and filter for the ext-authz-server logs in garden.
func CentralLoggingConfigurationForGarden() (component.CentralLoggingConfig, error) {
	return component.CentralLoggingConfig{Filters: generateClusterFilters(operatorv1alpha1.VirtualGardenNamePrefix), Parsers: generateClusterParsers(operatorv1alpha1.VirtualGardenNamePrefix)}, nil
}

func generateClusterFilters(prefix string) []*fluentbitv1alpha2.ClusterFilter {
	return []*fluentbitv1alpha2.ClusterFilter{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   prefix + v1beta1constants.DeploymentNameExtAuthzServer,
				Labels: map[string]string{v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource},
			},
			Spec: fluentbitv1alpha2.FilterSpec{
				Match: fmt.Sprintf("kubernetes.*%s*%s*", prefix+v1beta1constants.DeploymentNameExtAuthzServer, name),
				FilterItems: []fluentbitv1alpha2.FilterItem{
					{
						Parser: &fluentbitv1alpha2filter.Parser{
							KeyName:     "log",
							Parser:      name + "-parser",
							ReserveData: ptr.To(true),
						},
					},
				},
			},
		},
	}
}

func generateClusterParsers(prefix string) []*fluentbitv1alpha2.ClusterParser {
	return []*fluentbitv1alpha2.ClusterParser{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   prefix + name + "-parser",
				Labels: map[string]string{v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource},
			},
			Spec: fluentbitv1alpha2.ParserSpec{
				Regex: &fluentbitv1alpha2parser.Regex{
					Regex:      "^(?<severity>\\w)(?<time>\\d{4} [^\\s]*)\\s+(?<pid>\\d+)\\s+(?<source>[^ \\]]+)\\] (?<log>.*)$",
					TimeKey:    "time",
					TimeFormat: "%m%d %H:%M:%S.%L",
				},
			},
		},
	}
}
