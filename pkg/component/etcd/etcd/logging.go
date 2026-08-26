// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"fmt"

	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2filter "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/filter"
	fluentbitv1alpha2parser "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/parser"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
)

// CentralLoggingConfiguration returns a fluent-bit parser and filter for the etcd and backup-restore sidecar logs.
func CentralLoggingConfiguration() (component.CentralLoggingConfig, error) {
	return component.CentralLoggingConfig{Filters: generateClusterFilters(), Parsers: generateClusterParsers()}, nil
}

func generateClusterFilters() []*fluentbitv1alpha2.ClusterFilter {
	return []*fluentbitv1alpha2.ClusterFilter{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   fmt.Sprintf("%s--%s", statefulSetNamePrefix, etcdconstants.ContainerNameEtcd),
				Labels: map[string]string{v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource},
			},
			Spec: fluentbitv1alpha2.FilterSpec{
				Match: fmt.Sprintf("kubernetes.*%s*%s*", statefulSetNamePrefix, etcdconstants.ContainerNameEtcd),
				FilterItems: []fluentbitv1alpha2.FilterItem{
					{
						Parser: &fluentbitv1alpha2filter.Parser{
							KeyName:     "log",
							Parser:      etcdconstants.ContainerNameEtcd + "-parser",
							ReserveData: new(true),
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   fmt.Sprintf("%s--%s", statefulSetNamePrefix, etcdconstants.ContainerNameBackupRestore),
				Labels: map[string]string{v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource},
			},
			Spec: fluentbitv1alpha2.FilterSpec{
				Match: fmt.Sprintf("kubernetes.*%s*%s*", statefulSetNamePrefix, etcdconstants.ContainerNameBackupRestore),
				FilterItems: []fluentbitv1alpha2.FilterItem{
					{
						Parser: &fluentbitv1alpha2filter.Parser{
							KeyName:     "log",
							Parser:      etcdconstants.ContainerNameBackupRestore + "-parser",
							ReserveData: new(true),
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
				Name:   etcdconstants.ContainerNameEtcd + "-parser",
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
				Name:   etcdconstants.ContainerNameBackupRestore + "-parser",
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
