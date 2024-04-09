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
		Expect(strings.Join(admissionOpts.RecommendedPluginOrder, ",")).To(Equal("NamespaceLifecycle,MutatingAdmissionWebhook,ValidatingAdmissionPolicy,ValidatingAdmissionWebhook"))
		Expect(strings.Join(admissionOpts.Plugins.Registered(), ",")).To(Equal("MutatingAdmissionWebhook,NamespaceLifecycle,ValidatingAdmissionPolicy,ValidatingAdmissionWebhook"))
	})
})
