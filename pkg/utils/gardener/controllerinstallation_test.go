// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("ControllerInstallation", func() {
	Describe("#NamespaceNameForControllerInstallation", func() {
		It("should return the correct namespace name for the ControllerInstallation", func() {
			controllerInstallation := &gardencorev1beta1.ControllerInstallation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			}
			Expect(NamespaceNameForControllerInstallation(controllerInstallation)).To(Equal("extension-foo"))
		})
	})
})
