// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	policyv1 "k8s.io/api/policy/v1"

	. "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("#SetAlwaysAllowEviction", func() {
	var pdb *policyv1.PodDisruptionBudget

	BeforeEach(func() {
		pdb = &policyv1.PodDisruptionBudget{}
	})

	It("should set the UnhealthyPodEvictionPolicy field if version is >= 1.26", func() {
		SetAlwaysAllowEviction(pdb, semver.MustParse("1.26.0"))

		Expect(pdb.Spec.UnhealthyPodEvictionPolicy).To(PointTo(Equal(policyv1.AlwaysAllow)))
	})

	It("should not set the UnhealthyPodEvictionPolicy field if version is < 1.26", func() {
		SetAlwaysAllowEviction(pdb, semver.MustParse("1.25.0"))

		Expect(pdb.Spec.UnhealthyPodEvictionPolicy).To(BeNil())
	})
})
