// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package conversion_test

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	admissioncontrollerv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/operator/v1alpha1/conversion"
)

var _ = Describe("conversion", func() {
	Describe("#ConvertToAdmissionControllerResourceAdmissionConfiguration", func() {
		It("should return 'nil' when given config is 'nil'", func() {
			Expect(ConvertToAdmissionControllerResourceAdmissionConfiguration(nil)).To(BeNil())
		})

		It("should convert given config", func() {
			mode := operatorv1alpha1.ResourceAdmissionWebhookMode("block")

			operatorConfig := &operatorv1alpha1.ResourceAdmissionConfiguration{
				Limits: []operatorv1alpha1.ResourceLimit{
					{
						APIVersions: []string{"v1beta1"},
						APIGroups:   []string{"core.gardener.cloud"},
						Resources:   []string{"shoots"},
						Size:        resource.MustParse("1Ki"),
					},
					{
						APIVersions: []string{"v1"},
						APIGroups:   []string{""},
						Resources:   []string{"secrets", "configmaps"},
						Size:        resource.MustParse("100Ki"),
					},
				},
				UnrestrictedSubjects: []rbacv1.Subject{},
				OperationMode:        &mode,
			}

			admissionControllerConfig := ConvertToAdmissionControllerResourceAdmissionConfiguration(operatorConfig)

			Expect(reflect.ValueOf(operatorConfig.UnrestrictedSubjects).Pointer()).To(Equal(reflect.ValueOf(admissionControllerConfig.UnrestrictedSubjects).Pointer()))
			Expect(admissionControllerConfig.OperationMode).To(PointTo(Equal(admissioncontrollerv1alpha1.ResourceAdmissionWebhookMode("block"))))
			Expect(admissionControllerConfig.Limits).To(HaveLen(len(operatorConfig.Limits)))
			Expect(admissionControllerConfig.Limits).To(ConsistOf(
				admissioncontrollerv1alpha1.ResourceLimit{
					APIGroups:   operatorConfig.Limits[0].APIGroups,
					APIVersions: operatorConfig.Limits[0].APIVersions,
					Resources:   operatorConfig.Limits[0].Resources,
					Size:        operatorConfig.Limits[0].Size,
				},
				admissioncontrollerv1alpha1.ResourceLimit{
					APIGroups:   operatorConfig.Limits[1].APIGroups,
					APIVersions: operatorConfig.Limits[1].APIVersions,
					Resources:   operatorConfig.Limits[1].Resources,
					Size:        operatorConfig.Limits[1].Size,
				},
			))
		})
	})
})
