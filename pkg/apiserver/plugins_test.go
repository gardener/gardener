// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	genericoptions "k8s.io/apiserver/pkg/server/options"
)

var _ = Describe("AllPluginNames", func() {
	It("expects default plugins", func() {
		// if this test breaks, the default admission plugins in the API server library have changed
		admissionOpts := genericoptions.NewAdmissionOptions()
		// we can't automatically insert our admission plugins in the right order
		// we should reevaluate, what's the correct order for the plugins, when the default list of plugins changes
		Expect(strings.Join(admissionOpts.RecommendedPluginOrder, ",")).To(Equal("NamespaceLifecycle,MutatingAdmissionPolicy,MutatingAdmissionWebhook,ValidatingAdmissionPolicy,ValidatingAdmissionWebhook"))
		Expect(strings.Join(admissionOpts.Plugins.Registered(), ",")).To(Equal("MutatingAdmissionPolicy,MutatingAdmissionWebhook,NamespaceLifecycle,ValidatingAdmissionPolicy,ValidatingAdmissionWebhook"))
	})
})
