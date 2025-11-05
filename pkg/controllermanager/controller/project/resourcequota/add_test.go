// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcequota

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardenercorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Add", func() {
	var (
		reconciler    *Reconciler
		mockClient    *mockclient.MockClient
		ctrl          *gomock.Controller
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
		project = &gardenercorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: gardenercorev1beta1.ProjectSpec{
				Namespace: &namespaceName,
			},
		}
		shoot = &gardenercorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-shoot",
				Namespace: namespaceName,
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockClient = mockclient.NewMockClient(ctrl)
		reconciler = &Reconciler{
			Client: mockClient,
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("ObjectInProjectNamespace", func() {
		var (
			p predicate.Predicate
		)

		BeforeEach(func() {
			p = reconciler.ObjectInProjectNamespace()
		})

		It("return true for objects that are in a project namespace", func() {
			mockClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardenercorev1beta1.ProjectList{}), client.MatchingFields{gardencore.ProjectNamespace: namespaceName}).DoAndReturn(func(_ context.Context, list *gardenercorev1beta1.ProjectList, _ ...client.ListOption) error {
				list.Items = []gardenercorev1beta1.Project{
					*project,
				}
				return nil
			}).AnyTimes()

			Expect(p.Create(event.CreateEvent{Object: resourceQuota})).To(BeTrue())
			Expect(p.Update(event.UpdateEvent{ObjectNew: resourceQuota})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: resourceQuota})).To(BeTrue())
			Expect(p.Generic(event.GenericEvent{Object: resourceQuota})).To(BeTrue())
		})

		It("return false for objects that are not in a project namespace", func() {
			nonProjectObject := resourceQuota.DeepCopy()
			nonProjectObject.Namespace = namespaceName + "aaa"

			mockClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardenercorev1beta1.ProjectList{}), client.MatchingFields{gardencore.ProjectNamespace: namespaceName + "aaa"}).DoAndReturn(func(_ context.Context, list *gardenercorev1beta1.ProjectList, _ ...client.ListOption) error {
				list.Items = []gardenercorev1beta1.Project{}
				return nil
			}).AnyTimes()

			Expect(p.Create(event.CreateEvent{Object: nonProjectObject})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: nonProjectObject})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: nonProjectObject})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: nonProjectObject})).To(BeFalse())
		})
	})

	Describe("MapShootToResourceQuotasInProject", func() {
		var (
			mapFunc handler.MapFunc
		)

		BeforeEach(func() {
			mapFunc = reconciler.MapShootToResourceQuotasInProject(logr.Discard())
		})

		It("should enqueue requests for ResourceQuotas in the Shoot namespace", func() {
			mockClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ResourceQuotaList{}), client.InNamespace(namespaceName)).DoAndReturn(func(_ context.Context, list *corev1.ResourceQuotaList, _ ...client.ListOption) error {
				list.Items = []corev1.ResourceQuota{
					*resourceQuota,
				}
				return nil
			}).AnyTimes()

			Expect(mapFunc(context.Background(), shoot)).To(Equal([]reconcile.Request{
				{NamespacedName: types.NamespacedName{
					Name:      resourceQuota.Name,
					Namespace: resourceQuota.Namespace,
				}},
			}))
		})

		It("should not enqueue any requests if there are no ResourceQuotas in the Shoot namespace", func() {
			mockClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ResourceQuotaList{}), client.InNamespace(namespaceName)).DoAndReturn(func(_ context.Context, list *corev1.ResourceQuotaList, _ ...client.ListOption) error {
				list.Items = []corev1.ResourceQuota{}
				return nil
			}).AnyTimes()

			Expect(mapFunc(context.Background(), shoot)).To(Equal([]reconcile.Request{}))
		})
	})
})
