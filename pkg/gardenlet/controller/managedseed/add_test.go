// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package managedseed_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/managedseed"
)

var _ = Describe("Add", func() {
	var (
		fakeClient client.Client
		reconciler *Reconciler
		p          predicate.Predicate
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		reconciler = &Reconciler{
			GardenClient:          fakeClient,
			GardenNamespaceGarden: v1beta1constants.GardenNamespace,
		}
	})

	Describe("#SeedPredicate", func() {
		var (
			seed *gardencorev1beta1.Seed
		)

		BeforeEach(func() {
			seed = &gardencorev1beta1.Seed{}
			p = reconciler.SeedPredicate()
		})

		It("should return true", func() {
			seed.Labels = map[string]string{
				"name.seed.gardener.cloud/foo": "true",
				"name.seed.gardener.cloud/bar": "true",
			}

			Expect(p.Create(event.TypedCreateEvent[client.Object]{Object: seed})).To(BeTrue())
			Expect(p.Update(event.TypedUpdateEvent[client.Object]{ObjectNew: seed})).To(BeTrue())
			Expect(p.Delete(event.TypedDeleteEvent[client.Object]{Object: seed})).To(BeTrue())
			Expect(p.Generic(event.TypedGenericEvent[client.Object]{Object: seed})).To(BeTrue())
		})

		It("should return true for self-hosted shoot clusters with a single name label", func() {
			seed.Labels = map[string]string{
				"name.seed.gardener.cloud/root":                 "true",
				"seed.gardener.cloud/self-hosted-shoot-cluster": "true",
			}

			Expect(p.Create(event.TypedCreateEvent[client.Object]{Object: seed})).To(BeTrue())
			Expect(p.Update(event.TypedUpdateEvent[client.Object]{ObjectNew: seed})).To(BeTrue())
			Expect(p.Delete(event.TypedDeleteEvent[client.Object]{Object: seed})).To(BeTrue())
			Expect(p.Generic(event.TypedGenericEvent[client.Object]{Object: seed})).To(BeTrue())
		})

		It("should return false", func() {
			Expect(p.Create(event.TypedCreateEvent[client.Object]{Object: seed})).To(BeFalse())
			Expect(p.Update(event.TypedUpdateEvent[client.Object]{ObjectNew: seed})).To(BeFalse())
			Expect(p.Delete(event.TypedDeleteEvent[client.Object]{Object: seed})).To(BeFalse())
			Expect(p.Generic(event.TypedGenericEvent[client.Object]{Object: seed})).To(BeFalse())
		})
	})
})
