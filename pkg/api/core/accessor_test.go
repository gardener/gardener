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

package core_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	. "github.com/gardener/gardener/pkg/api/core"
	"github.com/gardener/gardener/pkg/apis/core"
)

var _ = Describe("Accessor", func() {
	Describe("#Accessor", func() {
		It("Should succeed to create an accessor", func() {
			shoot := &core.Shoot{}
			shootAcessor, err := Accessor(shoot)
			Expect(err).To(Not(HaveOccurred()))
			Expect(shoot).To(Equal(shootAcessor))
		})

		It("Should fail to create an accessor because of the missing implementation", func() {
			secretBinding := &corev1.Secret{}
			_, err := Accessor(secretBinding)
			Expect(err).To(HaveOccurred())
		})
	})
})
