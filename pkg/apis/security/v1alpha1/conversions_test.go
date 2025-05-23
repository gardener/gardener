// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener/pkg/apis/security"
	. "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
)

var _ = Describe("conversion", func() {
	var (
		targetType = "gardener"
		obj        = &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "garden",
				Name:      "configmap",
			},
			Data: map[string]string{
				"key": "value",
			},
		}
	)

	Describe("#Convert_v1alpha1_TargetSystem_To_security_TargetSystem", func() {
		It("should properly convert", func() {
			in := &TargetSystem{
				Type: targetType,
				ProviderConfig: &runtime.RawExtension{
					Object: obj,
				},
			}
			out := &security.TargetSystem{}

			Expect(Convert_v1alpha1_TargetSystem_To_security_TargetSystem(in, out, nil)).To(Succeed())

			Expect(out.Type).To(Equal(targetType))
			Expect(out.ProviderConfig).To(Equal(obj))
		})
	})

	Describe("#Convert_security_TargetSystem_To_v1alpha1_TargetSystem", func() {
		It("should properly convert", func() {
			in := &security.TargetSystem{
				Type:           targetType,
				ProviderConfig: obj,
			}
			out := &TargetSystem{}

			Expect(Convert_security_TargetSystem_To_v1alpha1_TargetSystem(in, out, nil)).To(Succeed())

			Expect(out.Type).To(Equal(targetType))
			Expect(out.ProviderConfig.Object).To(Equal(obj))
		})
	})

})
