// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pkg_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/plugin/pkg"
)

var _ = Describe("AllPluginNames", func() {
	It("must end with specific plugins", func() {
		// it's important for these admission plugins to be invoked at the end, ensure correct order here
		Expect(strings.Join(AllPluginNames(), ",")).To(HaveSuffix(",MutatingAdmissionPolicy,MutatingAdmissionWebhook,ValidatingAdmissionPolicy,ValidatingAdmissionWebhook,ResourceQuota"))
	})
})
