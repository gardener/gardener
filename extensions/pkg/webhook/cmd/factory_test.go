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

package cmd

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
)

var _ = Describe("FactoryAggregator", func() {
	var (
		wh1, wh2               *extensionswebhook.Webhook
		whFactory1, whFactory2 func(manager.Manager) (*extensionswebhook.Webhook, error)
	)

	BeforeEach(func() {
		wh1 = &extensionswebhook.Webhook{
			Name: "webhook-1",
		}
		//nolint:unparam
		whFactory1 = func(manager.Manager) (*extensionswebhook.Webhook, error) {
			return wh1, nil
		}
		wh2 = &extensionswebhook.Webhook{
			Name: "webhook-2",
		}
		//nolint:unparam
		whFactory2 = func(manager.Manager) (*extensionswebhook.Webhook, error) {
			return wh2, nil
		}
	})

	Describe("#Webhooks", func() {
		It("should return webhooks sorted by name", func() {
			agg := NewFactoryAggregator(FactoryAggregator{whFactory1, whFactory2})
			hooks, err := agg.Webhooks(nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(hooks).To(Equal([]*extensionswebhook.Webhook{wh1, wh2}))

			agg = NewFactoryAggregator(FactoryAggregator{whFactory2, whFactory1})
			hooks, err = agg.Webhooks(nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(hooks).To(Equal([]*extensionswebhook.Webhook{wh1, wh2}))
		})
	})
})
