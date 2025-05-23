// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package conversion_test

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
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
			Expect(admissionControllerConfig.OperationMode).To(PointTo(Equal(admissioncontrollerconfigv1alpha1.ResourceAdmissionWebhookMode("block"))))
			Expect(admissionControllerConfig.Limits).To(HaveLen(len(operatorConfig.Limits)))
			Expect(admissionControllerConfig.Limits).To(ConsistOf(
				admissioncontrollerconfigv1alpha1.ResourceLimit{
					APIGroups:   operatorConfig.Limits[0].APIGroups,
					APIVersions: operatorConfig.Limits[0].APIVersions,
					Resources:   operatorConfig.Limits[0].Resources,
					Size:        operatorConfig.Limits[0].Size,
				},
				admissioncontrollerconfigv1alpha1.ResourceLimit{
					APIGroups:   operatorConfig.Limits[1].APIGroups,
					APIVersions: operatorConfig.Limits[1].APIVersions,
					Resources:   operatorConfig.Limits[1].Resources,
					Size:        operatorConfig.Limits[1].Size,
				},
			))
		})
	})
})
