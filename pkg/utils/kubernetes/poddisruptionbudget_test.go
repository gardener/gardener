// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubernetes

import (
	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	policyv1 "k8s.io/api/policy/v1"
)

var _ = Describe("#SetUnhealthyPodEvictionPolicy", func() {
	var pdb *policyv1.PodDisruptionBudget

	BeforeEach(func() {
		pdb = &policyv1.PodDisruptionBudget{}
	})

	It("should set the UnhealthyPodEvictionPolicy field if version is >= 1.26", func() {
		SetUnhealthyPodEvictionPolicy(pdb, semver.MustParse("1.26.0"))

		Expect(pdb.Spec.UnhealthyPodEvictionPolicy).To(PointTo(Equal(policyv1.AlwaysAllow)))
	})

	It("should not set the UnhealthyPodEvictionPolicy field if version is < 1.26", func() {
		SetUnhealthyPodEvictionPolicy(pdb, semver.MustParse("1.25.0"))

		Expect(pdb.Spec.UnhealthyPodEvictionPolicy).To(BeNil())
	})
})
