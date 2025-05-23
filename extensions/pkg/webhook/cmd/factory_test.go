// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	. "github.com/gardener/gardener/extensions/pkg/webhook/cmd"
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
