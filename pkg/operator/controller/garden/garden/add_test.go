// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package garden_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/garden/garden"
)

var _ = Describe("Add", func() {
	Describe("#HasOperationAnnotation", func() {
		var (
			p      predicate.Predicate
			garden *operatorv1alpha1.Garden
		)

		BeforeEach(func() {
			p = (&Reconciler{}).HasOperationAnnotation()
			garden = &operatorv1alpha1.Garden{}
		})

		Describe("#Create", func() {
			It("should return false because no operation annotation present", func() {
				Expect(p.Create(event.CreateEvent{Object: garden})).To(BeFalse())
			})

			DescribeTable("operation annotation present",
				func(operation string, matcher gomegatypes.GomegaMatcher) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", operation)

					Expect(p.Create(event.CreateEvent{Object: garden})).To(matcher)
				},

				Entry("reconcile", "reconcile", BeTrue()),
				Entry("rotate-credentials-start", "rotate-credentials-start", BeTrue()),
				Entry("rotate-credentials-complete", "rotate-credentials-complete", BeTrue()),
				Entry("rotate-ca-start", "rotate-ca-start", BeTrue()),
				Entry("rotate-ca-complete", "rotate-ca-complete", BeTrue()),
				Entry("foo", "foo", BeFalse()),
			)
		})

		Describe("#Update", func() {
			It("should return false because no operation annotation present on old or new object", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: garden, ObjectNew: garden})).To(BeFalse())
			})

			It("should return false because operation annotation present on both old and new object", func() {
				metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "reconcile")
				gardenOld := garden.DeepCopy()

				Expect(p.Update(event.UpdateEvent{ObjectOld: gardenOld, ObjectNew: garden})).To(BeFalse())
			})

			It("should return false because operation annotation present on old object", func() {
				gardenOld := garden.DeepCopy()
				metav1.SetMetaDataAnnotation(&gardenOld.ObjectMeta, "gardener.cloud/operation", "reconcile")

				Expect(p.Update(event.UpdateEvent{ObjectOld: gardenOld, ObjectNew: garden})).To(BeFalse())
			})

			It("should return true when different operation annotation present on old and new object", func() {
				gardenOld := garden.DeepCopy()
				metav1.SetMetaDataAnnotation(&gardenOld.ObjectMeta, "gardener.cloud/operation", "reconcile")
				metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-credentials-start")

				Expect(p.Update(event.UpdateEvent{ObjectOld: gardenOld, ObjectNew: garden})).To(BeTrue())
			})

			It("should return true when parallel operations differ in old and new object", func() {
				gardenOld := garden.DeepCopy()
				metav1.SetMetaDataAnnotation(&gardenOld.ObjectMeta, "gardener.cloud/operation", "rotate-etcd-encryption-key;rotate-ssh-keypair")
				metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-etcd-encryption-key;rotate-ssh-keypair;rotate-ca-start")

				Expect(p.Update(event.UpdateEvent{ObjectOld: gardenOld, ObjectNew: garden})).To(BeTrue())
			})

			It("should return false when parallel operations differ only by order in old and new object", func() {
				gardenOld := garden.DeepCopy()
				metav1.SetMetaDataAnnotation(&gardenOld.ObjectMeta, "gardener.cloud/operation", "rotate-etcd-encryption-key;rotate-ssh-keypair")
				metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-ssh-keypair;rotate-etcd-encryption-key")
				metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "foo", "bar")

				Expect(p.Update(event.UpdateEvent{ObjectOld: gardenOld, ObjectNew: garden})).To(BeFalse())
			})

			DescribeTable("operation annotation present only on new object",
				func(operation string, matcher gomegatypes.GomegaMatcher) {
					gardenOld := garden.DeepCopy()
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", operation)

					Expect(p.Update(event.UpdateEvent{ObjectOld: gardenOld, ObjectNew: garden})).To(matcher)
				},

				Entry("reconcile", "reconcile", BeTrue()),
				Entry("rotate-credentials-start", "rotate-credentials-start", BeTrue()),
				Entry("rotate-credentials-complete", "rotate-credentials-complete", BeTrue()),
				Entry("rotate-ca-start", "rotate-ca-start", BeTrue()),
				Entry("rotate-ca-complete", "rotate-ca-complete", BeTrue()),
				Entry("foo", "foo", BeFalse()),
			)
		})

		Describe("#Delete", func() {
			It("should return false", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeFalse())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})

	Describe("#MapToGarden", func() {
		var (
			ctx           = context.Background()
			runtimeClient client.Client
			reconciler    *Reconciler
		)

		BeforeEach(func() {
			runtimeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()

			reconciler = &Reconciler{
				RuntimeClientSet: fakekubernetes.NewClientSetBuilder().WithClient(runtimeClient).Build(),
			}
		})

		It("should return no request if no Garden exists", func() {
			Expect(reconciler.MapToGarden(logr.Discard())(ctx, nil)).To(BeEmpty())
		})

		It("should return a request with the garden name", func() {
			Expect(runtimeClient.Create(ctx, &operatorv1alpha1.Garden{ObjectMeta: metav1.ObjectMeta{Name: "garden"}})).To(Succeed())

			Expect(reconciler.MapToGarden(logr.Discard())(ctx, nil)).To(ConsistOf(reconcile.Request{NamespacedName: types.NamespacedName{Name: "garden"}}))
		})
	})
})
