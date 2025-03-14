// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fluentcustomresources

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2parser "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/parser"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// GetClusterParsers returns the ClusterParsers used by the Fluent Operator.
func GetClusterParsers(labels map[string]string) []*fluentbitv1alpha2.ClusterParser {
	return []*fluentbitv1alpha2.ClusterParser{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "containerd-parser",
				Labels: labels,
			},
			Spec: fluentbitv1alpha2.ParserSpec{
				Regex: &fluentbitv1alpha2parser.Regex{
					Regex:      "^(?<time>[^ ]+) (stdout|stderr) ([^ ]*) (?<log>.*)$",
					TimeKey:    "time",
					TimeFormat: "%Y-%m-%dT%H:%M:%S.%L%z",
					TimeKeep:   ptr.To(true),
				},
				Decoders: []fluentbitv1alpha2.Decorder{ // spellchecker:disable-line
					{
						DecodeFieldAs: "json log",
					},
				},
			},
		},
	}
}
