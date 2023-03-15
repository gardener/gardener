// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package customresources

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2parser "github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2/plugins/parser"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// GetClusterParsers returns the ClusterParsers used by the Fluent Operator.
func GetClusterParsers(labels map[string]string) []*fluentbitv1alpha2.ClusterParser {
	return []*fluentbitv1alpha2.ClusterParser{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "docker-parser",
				Labels: labels,
			},
			Spec: fluentbitv1alpha2.ParserSpec{
				JSON: &fluentbitv1alpha2parser.JSON{
					TimeKey:    "time",
					TimeFormat: "%Y-%m-%dT%H:%M:%S.%L%z",
					TimeKeep:   pointer.BoolPtr(true),
				},
				Decoders: []fluentbitv1alpha2.Decorder{
					{
						DecodeFieldAs: "json log",
					},
				},
			},
		},
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
					TimeKeep:   pointer.BoolPtr(true),
				},
				Decoders: []fluentbitv1alpha2.Decorder{
					{
						DecodeFieldAs: "json log",
					},
				},
			},
		},
	}
}
