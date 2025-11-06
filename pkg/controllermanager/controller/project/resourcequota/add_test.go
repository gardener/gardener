// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcequota_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/controller/project/resourcequota"
)

var _ = Describe("Add", func() {
	var (
		reconciler    *resourcequota.Reconciler
		fakeClient    client.Client
		ctx           context.Context
		namespaceName = "garden-test"
		resourceQuota = &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-resourcequota",
				Namespace: namespaceName,
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					"count/configmaps": resource.MustParse("2"),
				},
			},
		}
		project = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: &namespaceName,
			},
		}
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-shoot",
				Namespace: namespaceName,
			},
		}
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeClient = fake.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithIndex(&gardencorev1beta1.Project{}, core.ProjectNamespace, indexer.ProjectNamespaceIndexerFunc).Build()

		reconciler = &resourcequota.Reconciler{
			Client: fakeClient,
		}
	})

	Describe("#ObjectInProjectNamespace", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.ObjectInProjectNamespace(ctx, GinkgoLogr)
		})

		It("return true for objects that are in a project namespace", func() {
			Expect(fakeClient.Create(ctx, project)).To(Succeed())

			Expect(p.Create(event.CreateEvent{Object: resourceQuota})).To(BeTrue())
			Expect(p.Update(event.UpdateEvent{ObjectNew: resourceQuota})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: resourceQuota})).To(BeTrue())
			Expect(p.Generic(event.GenericEvent{Object: resourceQuota})).To(BeTrue())
		})

		It("return false for objects that are not in a project namespace", func() {
			nonProjectObject := resourceQuota.DeepCopy()
			nonProjectObject.Namespace = namespaceName + "aaa"

			Expect(fakeClient.Create(ctx, nonProjectObject)).To(Succeed())

			Expect(p.Create(event.CreateEvent{Object: nonProjectObject})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: nonProjectObject})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: nonProjectObject})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: nonProjectObject})).To(BeFalse())
		})
	})

	Describe("#MapShootToResourceQuotasInProject", func() {
		It("should enqueue requests for ResourceQuotas in the Shoot namespace", func() {
			Expect(fakeClient.Create(ctx, resourceQuota)).To(Succeed())

			Expect(reconciler.MapShootToResourceQuotasInProject(GinkgoLogr)(ctx, shoot)).To(Equal([]reconcile.Request{
				{NamespacedName: types.NamespacedName{
					Name:      resourceQuota.Name,
					Namespace: resourceQuota.Namespace,
				}},
			}))
		})

		It("should not enqueue any requests if there are no ResourceQuotas in the Shoot namespace", func() {
			Expect(reconciler.MapShootToResourceQuotasInProject(GinkgoLogr)(ctx, shoot)).To(BeEmpty())
		})
	})
})
