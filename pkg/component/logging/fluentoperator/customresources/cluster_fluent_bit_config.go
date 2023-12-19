// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package customresources

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

const (
	fluentBitConfigName = "fluent-bit-config"
)

// GetClusterFluentBitConfig returns the ClusterFluentBitConfig used by the Fluent Operator.
func GetClusterFluentBitConfig(fluentBitName string, matchLabels map[string]string) *fluentbitv1alpha2.ClusterFluentBitConfig {
	return &fluentbitv1alpha2.ClusterFluentBitConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: fluentBitConfigName,
			Labels: map[string]string{
				"app.kubernetes.io/name": fluentBitName,
			},
		},
		Spec: fluentbitv1alpha2.FluentBitConfigSpec{
			Service: &fluentbitv1alpha2.Service{
				FlushSeconds: pointer.Int64(30),
				Daemon:       pointer.Bool(false),
				LogLevel:     "error",
				ParsersFile:  "parsers.conf",
				HttpServer:   pointer.Bool(true),
				HttpListen:   "0.0.0.0",
				HttpPort:     pointer.Int32(2020),
			},
			InputSelector: metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			FilterSelector: metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			ParserSelector: metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			OutputSelector: metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
		},
	}
}
