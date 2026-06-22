// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	. "github.com/gardener/gardener/pkg/api/core/v1/helper"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
)

var _ = Describe("Extension", func() {
	DescribeTable("#GetSecretsForOCIRepository",
		func(ociRepository *gardencorev1.OCIRepository, expected []string) {
			Expect(GetSecretsForOCIRepository(ociRepository)).To(Equal(expected))
		},

		Entry("nil OCIRepository", nil, []string(nil)),
		Entry("no secret refs", &gardencorev1.OCIRepository{}, []string(nil)),
		Entry("only CABundleSecretRef",
			&gardencorev1.OCIRepository{
				CABundleSecretRef: &corev1.LocalObjectReference{Name: "ca-bundle"},
			},
			[]string{"ca-bundle"},
		),
		Entry("only PullSecretRef",
			&gardencorev1.OCIRepository{
				PullSecretRef: &corev1.LocalObjectReference{Name: "pull-secret"},
			},
			[]string{"pull-secret"},
		),
		Entry("both CABundleSecretRef and PullSecretRef",
			&gardencorev1.OCIRepository{
				CABundleSecretRef: &corev1.LocalObjectReference{Name: "ca-bundle"},
				PullSecretRef:     &corev1.LocalObjectReference{Name: "pull-secret"},
			},
			[]string{"ca-bundle", "pull-secret"},
		),
	)
})
