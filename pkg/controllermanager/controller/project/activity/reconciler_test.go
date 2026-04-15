// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package activity_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/project/activity"
)

var _ = Describe("Project Activity", func() {
	var (
		projectName   string
		namespaceName string

		project *gardencorev1beta1.Project

		reconciler reconcile.Reconciler
		request    reconcile.Request

		fakeClock *testclock.FakeClock

		fakeClient client.Client
		ctx        context.Context
	)

	BeforeEach(func() {
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

	Describe("Reconciler", func() {
		BeforeEach(func() {
			fakeClock = testclock.NewFakeClock(time.Now())
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithStatusSubresource(&gardencorev1beta1.Project{}).
				Build()
			reconciler = &Reconciler{
				Client: fakeClient,
				Clock:  fakeClock,
			}

			Expect(fakeClient.Create(ctx, project)).To(Succeed())
		})

		Context("#Reconcile", func() {
			It("should update the lastActivityTimestamp to now", func() {
				request = reconcile.Request{NamespacedName: types.NamespacedName{Name: project.Name}}
				_, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeClient.Get(ctx, client.ObjectKey{Name: projectName}, project)).To(Succeed())
				// JSON serialization of metav1.Time truncates to second precision, so compare with truncated time
				now := &metav1.Time{Time: fakeClock.Now().Truncate(time.Second)}
				Expect(project.Status.LastActivityTimestamp).To(Equal(now))
			})

			It("should fail reconcile because the project can't be retrieved", func() {
				fakeErr := errors.New("error retrieving object from store")
				fakeClient = fakeclient.NewClientBuilder().
					WithScheme(kubernetes.GardenScheme).
					WithInterceptorFuncs(interceptor.Funcs{
						Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
							return fakeErr
						},
					}).
					Build()
				reconciler = &Reconciler{
					Client: fakeClient,
					Clock:  fakeClock,
				}

				request = reconcile.Request{NamespacedName: types.NamespacedName{Name: project.Name, Namespace: namespaceName + "other"}}
				_, err := reconciler.Reconcile(ctx, request)
				Expect(err).To(MatchError(fakeErr))
			})
		})
	})
})
