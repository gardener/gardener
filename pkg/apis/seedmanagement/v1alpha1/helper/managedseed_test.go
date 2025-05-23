// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"
)

var _ = Describe("Helper", func() {
	Describe("#GetBootstrap", func() {
		It("should return the correct Bootstrap value", func() {
			Expect(GetBootstrap(ptr.To(seedmanagementv1alpha1.BootstrapToken))).To(Equal(seedmanagementv1alpha1.BootstrapToken))
			Expect(GetBootstrap(ptr.To(seedmanagementv1alpha1.BootstrapServiceAccount))).To(Equal(seedmanagementv1alpha1.BootstrapServiceAccount))
			Expect(GetBootstrap(ptr.To(seedmanagementv1alpha1.BootstrapNone))).To(Equal(seedmanagementv1alpha1.BootstrapNone))
			Expect(GetBootstrap(nil)).To(Equal(seedmanagementv1alpha1.BootstrapNone))
		})
	})
})
