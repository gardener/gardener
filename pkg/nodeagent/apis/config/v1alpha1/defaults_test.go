// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
)

var _ = Describe("Defaults", func() {
	Describe("OperatorConfiguration", func() {
		Describe("Controller configuration", func() {
			Describe("Operating System Config controller", func() {
				It("should default the object", func() {
					obj := &OperatingSystemConfigControllerConfig{}

					SetDefaults_OperatingSystemConfigControllerConfig(obj)

					Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 10 * time.Minute})))
				})

				It("should not overwrite existing values", func() {
					obj := &OperatingSystemConfigControllerConfig{
						SyncPeriod: &metav1.Duration{Duration: time.Second},
					}

					SetDefaults_OperatingSystemConfigControllerConfig(obj)

					Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Second})))
				})
			})
		})
	})
})
