// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package copybackupstask_test

import (
	"context"
	"time"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	. "github.com/gardener/gardener/pkg/component/etcd/copybackupstask"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CopyBackupsTask", func() {
	var (
		ctx        context.Context
		fakeClient client.Client
		log        logr.Logger

		expected            *druidcorev1alpha1.EtcdCopyBackupsTask
		values              *Values
		etcdCopyBackupsTask Interface
		s                   *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.TODO()
		log = logr.Discard()

		s = runtime.NewScheme()
		Expect(druidcorev1alpha1.AddToScheme(s)).To(Succeed())
		fakeClient = fakeclient.NewClientBuilder().WithScheme(s).Build()

		expected = &druidcorev1alpha1.EtcdCopyBackupsTask{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bar",
				Namespace: "shoot--foo--bar",
			},
			Spec: druidcorev1alpha1.EtcdCopyBackupsTaskSpec{
				SourceStore: druidcorev1alpha1.StoreSpec{},
				TargetStore: druidcorev1alpha1.StoreSpec{},
			},
		}

		values = &Values{
			Name:      expected.Name,
			Namespace: expected.Namespace,
		}
		etcdCopyBackupsTask = New(log, fakeClient, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
	})

	Describe("#Deploy", func() {
		It("should create the EtcdCopyBackupsTask", func() {
			Expect(etcdCopyBackupsTask.Deploy(ctx)).To(Succeed())

			actual := &druidcorev1alpha1.EtcdCopyBackupsTask{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expected), actual)).To(Succeed())
			Expect(actual.Spec.PodLabels).To(Equal(map[string]string{
				"networking.gardener.cloud/to-dns":             "allowed",
				"networking.gardener.cloud/to-public-networks": "allowed",
			}))
			Expect(actual.Spec.SourceStore).To(Equal(expected.Spec.SourceStore))
			Expect(actual.Spec.TargetStore).To(Equal(expected.Spec.TargetStore))
		})
	})

	Describe("#Destroy", func() {
		It("should not return error if EtcdCopyBackupsTask resource doesn't exist", func() {
			Expect(etcdCopyBackupsTask.Destroy(ctx)).To(Succeed())
		})

		It("should properly delete EtcdCopyBackupsTask resource if it exists", func() {
			Expect(fakeClient.Create(ctx, expected.DeepCopy())).To(Succeed())
			Expect(etcdCopyBackupsTask.Destroy(ctx)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expected), &druidcorev1alpha1.EtcdCopyBackupsTask{})).To(BeNotFoundError())
		})
	})

	Describe("#Wait", func() {
		It("should return error if EtcdCopyBackupsTask resource is not found", func() {
			Expect(etcdCopyBackupsTask.Wait(ctx)).To(BeNotFoundError())
		})

		It("should return error if observed generation is nil", func() {
			Expect(fakeClient.Create(ctx, expected.DeepCopy())).To(Succeed())
			Expect(etcdCopyBackupsTask.Wait(ctx)).To(MatchError(ContainSubstring("observed generation not recorded")))
		})

		It("should return error if observed generation is not updated", func() {
			obj := expected.DeepCopy()
			obj.Generation = 1
			obj.Status.ObservedGeneration = ptr.To[int64](0)
			Expect(fakeClient.Create(ctx, obj)).To(Succeed())
			Expect(etcdCopyBackupsTask.Wait(ctx)).To(MatchError(ContainSubstring("observed generation outdated (0/1)")))
		})

		It("should return error if EtcdCopyBackupsTask reconciliation encountered error", func() {
			errorText := "some error"
			obj := expected.DeepCopy()
			obj.Status.ObservedGeneration = &expected.Generation
			obj.Status.LastError = &errorText
			Expect(fakeClient.Create(ctx, obj)).To(Succeed())
			Expect(etcdCopyBackupsTask.Wait(ctx)).To(MatchError(ContainSubstring(errorText)))
		})

		It("should return error if expected Successful or Failed conditions are not added yet", func() {
			obj := expected.DeepCopy()
			obj.Status.ObservedGeneration = &expected.Generation
			Expect(fakeClient.Create(ctx, obj)).To(Succeed())
			Expect(etcdCopyBackupsTask.Wait(ctx)).To(MatchError(ContainSubstring("expected condition")))
		})

		It("should return error if Failed condition with status True has been added", func() {
			var callCount int

			fakeClient = fakeclient.NewClientBuilder().WithScheme(s).WithInterceptorFuncs(interceptor.Funcs{
				Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					etcdCopyBackupsTask, ok := obj.(*druidcorev1alpha1.EtcdCopyBackupsTask)
					if !ok {
						return nil
					}

					callCount++
					etcdCopyBackupsTask.Status.ObservedGeneration = &expected.Generation
					etcdCopyBackupsTask.Status.Conditions = []druidcorev1alpha1.Condition{
						{
							Type:    druidcorev1alpha1.EtcdCopyBackupsTaskFailed,
							Status:  druidcorev1alpha1.ConditionTrue,
							Reason:  "reason",
							Message: "message",
						},
					}
					return nil
				},
			}).Build()

			etcdCopyBackupsTask = New(log, fakeClient, values, time.Millisecond, 250*time.Millisecond, 500*time.Millisecond)
			Expect(etcdCopyBackupsTask.Wait(ctx)).To(HaveOccurred())
			Expect(callCount).To(BeNumerically(">", 1), "should have retried multiple times")
		})

		It("should be successful if Successful condition with status True has been added", func() {
			obj := expected.DeepCopy()
			obj.Status.ObservedGeneration = &expected.Generation
			obj.Status.Conditions = []druidcorev1alpha1.Condition{
				{
					Type:    druidcorev1alpha1.EtcdCopyBackupsTaskSucceeded,
					Status:  druidcorev1alpha1.ConditionTrue,
					Reason:  "reason",
					Message: "message",
				},
			}
			Expect(fakeClient.Create(ctx, obj)).To(Succeed())
			Expect(etcdCopyBackupsTask.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should be successful if EtcdCopyBackupsTask resource does not exist", func() {
			Expect(etcdCopyBackupsTask.WaitCleanup(ctx)).To(Succeed())
		})

		It("should return error if EtcdCopyBackupsTask resource does not get deleted", func() {
			Expect(fakeClient.Create(ctx, expected.DeepCopy())).To(Succeed())
			Expect(etcdCopyBackupsTask.WaitCleanup(ctx)).To(HaveOccurred())
		})
	})
})
