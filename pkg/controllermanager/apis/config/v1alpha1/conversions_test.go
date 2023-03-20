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

package v1alpha1_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	. "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Conversion", func() {
	Context("ControllerManagerConfiguration conversions", func() {
		Describe("#Convert_v1alpha1_QuotaConfiguration_To_config_QuotaConfiguration", func() {
			var (
				out *config.QuotaConfiguration
				in  *QuotaConfiguration
			)

			BeforeEach(func() {
				in = &QuotaConfiguration{}
				out = &config.QuotaConfiguration{}
			})

			It("should successfully convert the containing ResourceQuota", func() {
				resourceQuota := &corev1.ResourceQuota{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "ResourceQuota",
					},
					Spec: corev1.ResourceQuotaSpec{
						Hard: map[corev1.ResourceName]resource.Quantity{
							"shoots.core.gardener.cloud": resource.MustParse("1"),
						},
					},
				}

				data, err := json.Marshal(resourceQuota)
				Expect(err).ToNot(HaveOccurred())

				in.Config = runtime.RawExtension{
					Raw: data,
				}

				Expect(Convert_v1alpha1_QuotaConfiguration_To_config_QuotaConfiguration(in, out, nil)).To(Succeed())
				Expect(out.Config).To(Equal(resourceQuota))
			})

			It("should skip the conversion", func() {
				Expect(Convert_v1alpha1_QuotaConfiguration_To_config_QuotaConfiguration(in, out, nil)).To(Succeed())
				Expect(out.Config).To(BeNil())
			})

			It("should fail to convert the containing ResourceQuota", func() {
				resourceQuota := &corev1.ResourceQuota{
					Spec: corev1.ResourceQuotaSpec{
						Hard: map[corev1.ResourceName]resource.Quantity{
							"shoots.core.gardener.cloud": resource.MustParse("1"),
						},
					},
				}

				data, err := json.Marshal(resourceQuota)
				Expect(err).ToNot(HaveOccurred())

				in.Config = runtime.RawExtension{
					Raw: data,
				}

				Expect(Convert_v1alpha1_QuotaConfiguration_To_config_QuotaConfiguration(in, out, nil)).To(matchers.BeMissingKindError())
			})
		})
	})
})
