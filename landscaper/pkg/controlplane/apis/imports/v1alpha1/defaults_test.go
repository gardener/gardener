// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1alpha1_test

import (
	"k8s.io/utils/pointer"

	. "github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Defaults", func() {
	Describe("#SetDefaults_Imports", func() {
		It("should enable the Seed restriction webhook if the Seed Authorizer is enabled", func() {
			var obj = &Imports{
				Rbac: &Rbac{
					SeedAuthorizer: &SeedAuthorizer{Enabled: pointer.Bool(true)},
				},
				GardenerAdmissionController: &GardenerAdmissionController{
					Enabled: true,
				},
			}
			SetDefaults_Imports(obj)

			Expect(obj).To(Equal(&Imports{
				Rbac: &Rbac{
					SeedAuthorizer: &SeedAuthorizer{Enabled: pointer.Bool(true)},
				},
				GardenerAdmissionController: &GardenerAdmissionController{
					Enabled: true,
					SeedRestriction: &SeedRestriction{
						Enabled: true,
					},
				},
			}))
		})
	})
})
