// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package activity_test

import (
	"context"
	"errors"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/project/activity"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Project Activity", func() {
	var (
		projectName   string
		namespaceName string

		project *gardencorev1beta1.Project

		reconciler reconcile.Reconciler
		request    reconcile.Request

		fakeClock *testclock.FakeClock

		ctrl                   *gomock.Controller
		k8sGardenRuntimeClient *mockclient.MockClient
		mockStatusWriter       *mockclient.MockStatusWriter
		ctx                    context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		k8sGardenRuntimeClient = mockclient.NewMockClient(ctrl)
		mockStatusWriter = mockclient.NewMockStatusWriter(ctrl)

		ctx = context.TODO()

		projectName = "name"
		namespaceName = "namespace"

		project = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectName,
			},
			Status: gardencorev1beta1.ProjectStatus{
				LastActivityTimestamp: &metav1.Time{Time: time.Date(1, 1, 1, 1, 1, 1, 1, time.UTC)},
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: &namespaceName,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("Reconciler", func() {
		BeforeEach(func() {
			fakeClock = testclock.NewFakeClock(time.Now())
			reconciler = &Reconciler{
				Client: k8sGardenRuntimeClient,
				Clock:  fakeClock,
			}

			k8sGardenRuntimeClient.EXPECT().Get(
				gomock.Any(),
				gomock.Any(),
				gomock.AssignableToTypeOf(&gardencorev1beta1.Project{}),
			).DoAndReturn(func(_ context.Context, namespacedName client.ObjectKey, obj *gardencorev1beta1.Project, _ ...client.GetOption) error {
				if reflect.DeepEqual(namespacedName.Namespace, namespaceName) {
					project.DeepCopyInto(obj)
					return nil
				}
				return errors.New("error retrieving object from store")
			})

			k8sGardenRuntimeClient.EXPECT().Status().Return(mockStatusWriter).AnyTimes()

			mockStatusWriter.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Project{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, prj *gardencorev1beta1.Project, _ client.Patch, _ ...client.PatchOption) error {
					*project = *prj
					return nil
				},
			).AnyTimes()
		})

		Context("#Reconcile", func() {
			It("should update the lastActivityTimestamp to now", func() {
				request = reconcile.Request{NamespacedName: types.NamespacedName{Name: project.Name, Namespace: namespaceName}}
				_, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())

				now := &metav1.Time{Time: fakeClock.Now()}
				Expect(project.Status.LastActivityTimestamp).To(Equal(now))
			})

			It("should fail reconcile because the project can't be retrieved", func() {
				request = reconcile.Request{NamespacedName: types.NamespacedName{Name: project.Name, Namespace: namespaceName + "other"}}
				_, err := reconciler.Reconcile(ctx, request)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
