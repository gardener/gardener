// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcequota_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/controllermanager/controller/project/resourcequota"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Project ResourceQuota", func() {
	var (
		ctrl          *gomock.Controller
		runtimeClient *mockclient.MockClient
		reconciler    resourcequota.Reconciler
		resourceQuota *corev1.ResourceQuota

		name          = "test-resource-quota"
		namespaceName = "garden-test"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		runtimeClient = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("Reconcile", func() {
		BeforeEach(func() {
			reconciler = resourcequota.Reconciler{
				Client: runtimeClient,
			}

			resourceQuota = &corev1.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespaceName,
				},
				Spec: corev1.ResourceQuotaSpec{
					Hard: corev1.ResourceList{
						"count/configmaps":                 resource.MustParse("1"),
						"count/secrets":                    resource.MustParse("1"),
						"count/shoots.core.gardener.cloud": resource.MustParse("4"),
					},
				},
			}

			runtimeClient.EXPECT().Get(
				gomock.Any(),
				gomock.Any(),
				gomock.AssignableToTypeOf(&corev1.ResourceQuota{}),
			).DoAndReturn(func(_ context.Context, namespacedName types.NamespacedName, rq *corev1.ResourceQuota, _ ...client.GetOption) error {
				if namespacedName.Namespace == namespaceName {
					resourceQuota.DeepCopyInto(rq)
					return nil
				}
				return errors.New("error retrieving object from store")
			}).AnyTimes()

			runtimeClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ResourceQuota{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, rq *corev1.ResourceQuota, _ client.Patch, _ ...client.PatchOption) error {
					*resourceQuota = *rq
					return nil
				},
			).AnyTimes()
		})

		It("should adapt configmap and secret quotas to fit all shoots", func() {
			request := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespaceName, Name: name}}
			_, err := reconciler.Reconcile(context.Background(), request)
			Expect(err).NotTo(HaveOccurred())

			expectedHard := corev1.ResourceList{
				"count/configmaps":                 resource.MustParse("8"),  //  4 Shoots * 2 ConfigMaps each
				"count/secrets":                    resource.MustParse("16"), // 4 Shoots * 4 Secrets each
				"count/shoots.core.gardener.cloud": resource.MustParse("4"),
			}

			Expect(resourceQuota.Spec.Hard).To(Equal(expectedHard))
		})

		It("should not adapt the quota if configmap and secret quotas are sufficient", func() {
			sufficientHard := corev1.ResourceList{
				"count/configmaps":                 resource.MustParse("10"),
				"count/secrets":                    resource.MustParse("20"),
				"count/shoots.core.gardener.cloud": resource.MustParse("4"),
			}
			resourceQuota.Spec.Hard = sufficientHard

			request := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespaceName, Name: name}}
			_, err := reconciler.Reconcile(context.Background(), request)
			Expect(err).NotTo(HaveOccurred())

			Expect(resourceQuota.Spec.Hard).To(Equal(sufficientHard))
		})
	})
})
