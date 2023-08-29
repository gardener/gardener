/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// origin: https://github.com/kubernetes/kubernetes/blob/1467b588060812a11c3e556f645ce0a949bb4b36/pkg/apis/core/validation/validation_test.go
// Modifications are under
// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

package core_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/gardener/gardener/pkg/utils/validation/kubernetes/core"
)

var _ = Describe("#Tolerations", func() {
	var fieldPath *field.Path

	BeforeEach(func() {
		fieldPath = field.NewPath("tolerations")
	})

	DescribeTable(
		"#Success cases",
		func(tolerations []corev1.Toleration) {
			Expect(ValidateTolerations(tolerations, fieldPath)).To(BeEmpty())
		},

		// origin: https://github.com/kubernetes/kubernetes/blob/1467b588060812a11c3e556f645ce0a949bb4b36/pkg/apis/core/validation/validation_test.go#L9646
		Entry(
			"populate forgiveness tolerations with exists operator in annotations.",
			[]corev1.Toleration{{Key: "foo", Operator: "Exists", Value: "", Effect: "NoExecute", TolerationSeconds: &[]int64{60}[0]}},
		),

		// origin: https://github.com/kubernetes/kubernetes/blob/1467b588060812a11c3e556f645ce0a949bb4b36/pkg/apis/core/validation/validation_test.go#L9653
		Entry(
			"populate forgiveness tolerations with equal operator in annotations.",
			[]corev1.Toleration{{Key: "foo", Operator: "Equal", Value: "bar", Effect: "NoExecute", TolerationSeconds: &[]int64{60}[0]}},
		),

		// origin: https://github.com/kubernetes/kubernetes/blob/1467b588060812a11c3e556f645ce0a949bb4b36/pkg/apis/core/validation/validation_test.go#L9660
		Entry(
			"populate tolerations equal operator in annotations.",
			[]corev1.Toleration{{Key: "foo", Operator: "Equal", Value: "bar", Effect: "NoSchedule"}},
		),

		// origin: https://github.com/kubernetes/kubernetes/blob/1467b588060812a11c3e556f645ce0a949bb4b36/pkg/apis/core/validation/validation_test.go#L9674
		Entry(
			"empty key with Exists operator is OK for toleration, empty toleration key means match all taint keys.",
			[]corev1.Toleration{{Operator: "Exists", Effect: "NoSchedule"}},
		),

		// origin: https://github.com/kubernetes/kubernetes/blob/1467b588060812a11c3e556f645ce0a949bb4b36/pkg/apis/core/validation/validation_test.go#L9681
		Entry(
			"empty operator is OK for toleration, defaults to Equal.",
			[]corev1.Toleration{{Key: "foo", Value: "bar", Effect: "NoSchedule"}},
		),

		// origin: https://github.com/kubernetes/kubernetes/blob/1467b588060812a11c3e556f645ce0a949bb4b36/pkg/apis/core/validation/validation_test.go#L9688
		Entry(
			"empty effect is OK for toleration, empty toleration effect means match all taint effects.",
			[]corev1.Toleration{{Key: "foo", Operator: "Equal", Value: "bar"}},
		),

		// origin: https://github.com/kubernetes/kubernetes/blob/1467b588060812a11c3e556f645ce0a949bb4b36/pkg/apis/core/validation/validation_test.go#L9695
		Entry(
			"negative tolerationSeconds is OK for toleration.",
			[]corev1.Toleration{{Key: "node.kubernetes.io/not-ready", Operator: "Exists", Effect: "NoExecute", TolerationSeconds: &[]int64{-2}[0]}},
		),
	)

	DescribeTable(
		"#Error cases",
		func(tolerations []corev1.Toleration, expectedError string) {
			// origin: https://github.com/kubernetes/kubernetes/blob/1467b588060812a11c3e556f645ce0a949bb4b36/pkg/apis/core/validation/validation_test.go#L10950-L10959
			errs := ValidateTolerations(tolerations, fieldPath)
			Expect(errs).NotTo(BeEmpty())
			Expect(errs.ToAggregate().Error()).To(ContainSubstring(expectedError))
		},

		// origin: https://github.com/kubernetes/kubernetes/blob/1467b588060812a11c3e556f645ce0a949bb4b36/pkg/apis/core/validation/validation_test.go#L10465
		Entry(
			"invalid toleration key",
			[]corev1.Toleration{{Key: "nospecialchars^=@", Operator: "Equal", Value: "bar", Effect: "NoSchedule"}},
			"tolerations[0].key",
		),

		// origin: https://github.com/kubernetes/kubernetes/blob/1467b588060812a11c3e556f645ce0a949bb4b36/pkg/apis/core/validation/validation_test.go#L10475
		Entry(
			"invalid toleration operator",
			[]corev1.Toleration{{Key: "foo", Operator: "In", Value: "bar", Effect: "NoSchedule"}},
			"tolerations[0].operator",
		),

		// origin: https://github.com/kubernetes/kubernetes/blob/1467b588060812a11c3e556f645ce0a949bb4b36/pkg/apis/core/validation/validation_test.go#L10485
		Entry(
			"value must be empty when `operator` is 'Exists'",
			[]corev1.Toleration{{Key: "foo", Operator: "Exists", Value: "bar", Effect: "NoSchedule"}},
			"tolerations[0].operator",
		),

		// origin: https://github.com/kubernetes/kubernetes/blob/1467b588060812a11c3e556f645ce0a949bb4b36/pkg/apis/core/validation/validation_test.go#L10496
		Entry(
			"operator must be 'Exists' when `key` is empty",
			[]corev1.Toleration{{Operator: "Equal", Value: "bar", Effect: "NoSchedule"}},
			"tolerations[0].operator",
		),

		// origin: https://github.com/kubernetes/kubernetes/blob/1467b588060812a11c3e556f645ce0a949bb4b36/pkg/apis/core/validation/validation_test.go#L10506
		Entry(
			"effect must be 'NoExecute' when `TolerationSeconds` is set",
			[]corev1.Toleration{{Key: "node.kubernetes.io/not-ready", Operator: "Exists", Effect: "NoSchedule", TolerationSeconds: &[]int64{20}[0]}},
			"tolerations[0].effect",
		),
	)
})
