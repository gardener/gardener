// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package refallowlist_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/seed/refallowlist"
)

var _ = Describe("SeedEventHandler", func() {
	var (
		ctx   context.Context
		queue workqueue.TypedRateLimitingInterface[Request]
		hdlr  handler.TypedEventHandler[client.Object, Request]
	)

	encodeConfig := func(spec gardencorev1beta1.SeedSpec) runtime.RawExtension {
		cfg := &gardenletconfigv1alpha1.GardenletConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
				Kind:       "GardenletConfiguration",
			},
			SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{Spec: spec},
			},
		}
		raw, err := encoding.EncodeGardenletConfiguration(cfg)
		Expect(err).NotTo(HaveOccurred())
		return *raw
	}

	refA := corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Namespace: "garden", Name: "secret-a"}
	refB := corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Namespace: "garden", Name: "secret-b"}

	specWith := func(refs ...corev1.ObjectReference) gardencorev1beta1.SeedSpec {
		spec := gardencorev1beta1.SeedSpec{}
		for i, r := range refs {
			switch i {
			case 0:
				spec.DNS.Internal = &gardencorev1beta1.SeedDNSProviderConfig{
					Type:           "test",
					Domain:         "test.example.com",
					CredentialsRef: r,
				}
			case 1:
				spec.DNS.Defaults = []gardencorev1beta1.SeedDNSProviderConfig{
					{Type: "test", Domain: "default.example.com", CredentialsRef: r},
				}
			}
		}
		return spec
	}

	newGardenlet := func(name string, spec gardencorev1beta1.SeedSpec) *seedmanagementv1alpha1.Gardenlet {
		return &seedmanagementv1alpha1.Gardenlet{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       seedmanagementv1alpha1.GardenletSpec{Config: encodeConfig(spec)},
		}
	}

	noSeedConfig := func(name string) *seedmanagementv1alpha1.Gardenlet {
		cfg := &gardenletconfigv1alpha1.GardenletConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
				Kind:       "GardenletConfiguration",
			},
		}
		raw, err := encoding.EncodeGardenletConfiguration(cfg)
		Expect(err).NotTo(HaveOccurred())
		return &seedmanagementv1alpha1.Gardenlet{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       seedmanagementv1alpha1.GardenletSpec{Config: *raw},
		}
	}

	requests := func() []Request {
		var items []Request
		for queue.Len() > 0 {
			item, _ := queue.Get()
			items = append(items, item)
			queue.Done(item)
		}
		return items
	}

	BeforeEach(func() {
		ctx = context.Background()
		queue = workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[Request]())
		hdlr = SeedEventHandler(func(obj *seedmanagementv1alpha1.Gardenlet) *runtime.RawExtension {
			return &obj.Spec.Config
		})
	})

	Describe("#Update", func() {
		It("should only enqueue removed refs for removal", func() {
			old := newGardenlet("seed-a", specWith(refA, refB))
			updated := newGardenlet("seed-a", specWith(refB))

			hdlr.Update(ctx, event.TypedUpdateEvent[client.Object]{ObjectOld: old, ObjectNew: updated}, queue)

			Expect(requests()).To(ConsistOf(
				Request{APIVersion: "v1", Kind: "Secret", Namespace: "garden", Name: "secret-a", SeedName: "seed-a", Remove: true},
			))
		})

		It("should only enqueue newly added refs for addition", func() {
			old := newGardenlet("seed-a", specWith(refA))
			updated := newGardenlet("seed-a", specWith(refA, refB))

			hdlr.Update(ctx, event.TypedUpdateEvent[client.Object]{ObjectOld: old, ObjectNew: updated}, queue)

			Expect(requests()).To(ConsistOf(
				Request{APIVersion: "v1", Kind: "Secret", Namespace: "garden", Name: "secret-b", SeedName: "seed-a", Remove: false},
			))
		})

		It("should not enqueue unchanged refs", func() {
			old := newGardenlet("seed-a", specWith(refA))
			updated := newGardenlet("seed-a", specWith(refA))

			hdlr.Update(ctx, event.TypedUpdateEvent[client.Object]{ObjectOld: old, ObjectNew: updated}, queue)

			Expect(queue.Len()).To(Equal(0))
		})

		It("should enqueue all old refs for removal when SeedConfig becomes nil", func() {
			old := newGardenlet("seed-a", specWith(refA, refB))
			updated := noSeedConfig("seed-a")

			hdlr.Update(ctx, event.TypedUpdateEvent[client.Object]{ObjectOld: old, ObjectNew: updated}, queue)

			Expect(requests()).To(ConsistOf(
				Request{APIVersion: "v1", Kind: "Secret", Namespace: "garden", Name: "secret-a", SeedName: "seed-a", Remove: true},
				Request{APIVersion: "v1", Kind: "Secret", Namespace: "garden", Name: "secret-b", SeedName: "seed-a", Remove: true},
			))
		})
	})

	Describe("#Delete", func() {
		It("should enqueue all refs for removal", func() {
			obj := newGardenlet("seed-a", specWith(refA, refB))

			hdlr.Delete(ctx, event.TypedDeleteEvent[client.Object]{Object: obj}, queue)

			Expect(requests()).To(ConsistOf(
				Request{APIVersion: "v1", Kind: "Secret", Namespace: "garden", Name: "secret-a", SeedName: "seed-a", Remove: true},
				Request{APIVersion: "v1", Kind: "Secret", Namespace: "garden", Name: "secret-b", SeedName: "seed-a", Remove: true},
			))
		})
	})
})
