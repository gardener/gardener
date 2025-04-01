// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd_test

import (
	"context"
	"fmt"
	"time"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/component/etcd/etcd"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	mocktime "github.com/gardener/gardener/third_party/mock/go/time"
)

var _ = Describe("#Wait", func() {
	var (
		ctrl    *gomock.Controller
		c       client.Client
		sm      secretsmanager.Interface
		log     logr.Logger
		mockNow *mocktime.MockNow
		now     time.Time

		waiter      *retryfake.Ops
		cleanupFunc func()

		ctx  = context.TODO()
		name = "etcd-" + testRole

		etcd     Interface
		expected *druidcorev1alpha1.Etcd
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockNow = mocktime.NewMockNow(ctrl)
		now = time.Now()

		s := runtime.NewScheme()
		Expect(corev1.AddToScheme(s)).To(Succeed())
		Expect(appsv1.AddToScheme(s)).To(Succeed())
		Expect(networkingv1.AddToScheme(s)).To(Succeed())
		Expect(vpaautoscalingv1.AddToScheme(s)).To(Succeed())
		Expect(druidcorev1alpha1.AddToScheme(s)).To(Succeed())
		Expect(monitoringv1alpha1.AddToScheme(s)).To(Succeed())
		Expect(monitoringv1.AddToScheme(s)).To(Succeed())
		c = fake.NewClientBuilder().WithScheme(s).Build()

		sm = fakesecretsmanager.New(c, testNamespace)
		log = logr.Discard()

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd", Namespace: testNamespace}})).To(Succeed())

		waiter = &retryfake.Ops{MaxAttempts: 1}
		cleanupFunc = test.WithVars(
			&retry.Until, waiter.Until,
			&retry.UntilTimeout, waiter.UntilTimeout,
		)

		etcd = New(log, c, testNamespace, sm, Values{
			Replicas:        ptr.To[int32](3),
			Role:            testRole,
			Class:           ClassNormal,
			StorageCapacity: "20Gi",
			MaintenanceTimeWindow: gardencorev1beta1.MaintenanceTimeWindow{
				Begin: "1234",
				End:   "5678",
			},
			EvictionRequirement: ptr.To(v1beta1constants.EvictionRequirementInMaintenanceWindowOnly),
		})

		expected = &druidcorev1alpha1.Etcd{
			TypeMeta: metav1.TypeMeta{
				APIVersion: druidcorev1alpha1.SchemeGroupVersion.String(),
				Kind:       "Etcd",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNamespace,
				Annotations: map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
					v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
				},
			},
			Spec: druidcorev1alpha1.EtcdSpec{
				Replicas: 3,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
		cleanupFunc()
	})

	It("should return error when it's not found", func() {
		Expect(etcd.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
	})

	It("should return error when it's not ready", func() {
		defer test.WithVars(
			&TimeNow, mockNow.Do,
		)()
		mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()
		delete(expected.Annotations, v1beta1constants.GardenerOperation)
		expected.Status.LastErrors = []druidcorev1alpha1.LastError{}
		expected.Status.ObservedGeneration = ptr.To[int64](expected.Generation)
		expected.Status.Conditions = []druidcorev1alpha1.Condition{
			{
				Type:   druidcorev1alpha1.ConditionTypeAllMembersUpdated,
				Status: druidcorev1alpha1.ConditionTrue,
			},
		}
		expected.Status.Ready = ptr.To(false)

		Expect(c.Create(ctx, expected)).To(Succeed(), "creating etcd succeeds")
		Expect(etcd.Wait(ctx)).To(MatchError(ContainSubstring("is not ready yet")))
	})

	It("should return error if we haven't observed the latest timestamp annotation", func() {
		defer test.WithVars(
			&TimeNow, mockNow.Do,
		)()
		mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

		By("Deploy")
		// Deploy should fill internal state with the added timestamp annotation
		Expect(etcd.Deploy(ctx)).To(Succeed())

		By("Patch object")
		patch := client.MergeFrom(expected.DeepCopy())
		expected.Status.LastErrors = nil
		// remove operation annotation, add old timestamp annotation
		expected.Annotations = map[string]string{
			v1beta1constants.GardenerTimestamp: now.Add(-time.Millisecond).UTC().Format(time.RFC3339Nano),
		}
		expected.Status.Conditions = []druidcorev1alpha1.Condition{
			{
				Type:   druidcorev1alpha1.ConditionTypeAllMembersUpdated,
				Status: druidcorev1alpha1.ConditionTrue,
			},
		}
		expected.Status.Ready = ptr.To(true)
		Expect(c.Patch(ctx, expected, patch)).To(Succeed(), "patching etcd succeeds")

		By("Wait")
		Expect(etcd.Wait(ctx)).NotTo(Succeed(), "etcd indicates error")
	})

	It("should return no error if etcd replicas set to 0", func() {
		defer test.WithVars(
			&TimeNow, mockNow.Do,
		)()
		mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

		By("Deploy")
		etcd.SetReplicas(ptr.To[int32](0))
		Expect(etcd.Deploy(ctx)).To(Succeed())

		By("Patch object")
		patch := client.MergeFrom(expected.DeepCopy())
		expected.Status.ObservedGeneration = ptr.To[int64](0)
		expected.Status.LastErrors = nil
		// remove operation annotation, add up-to-date timestamp annotation
		expected.Annotations = map[string]string{
			v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
		}
		// assume that condition checks and readiness are false, to show that these don't matter when replicas is 0
		expected.Status.Conditions = []druidcorev1alpha1.Condition{
			{
				Type:   druidcorev1alpha1.ConditionTypeAllMembersUpdated,
				Status: druidcorev1alpha1.ConditionUnknown,
			},
		}
		expected.Status.Ready = ptr.To(false)
		Expect(c.Patch(ctx, expected, patch)).To(Succeed(), "patching etcd succeeds")

		By("Wait")
		Expect(etcd.Wait(ctx)).To(Succeed(), "etcd is ready")
	})

	It("should return error if AllMembersUpdated condition is not set", func() {
		defer test.WithVars(
			&TimeNow, mockNow.Do,
		)()
		mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

		By("Deploy")
		// Deploy should fill internal state with the added timestamp annotation
		Expect(etcd.Deploy(ctx)).To(Succeed())

		By("Patch object")
		patch := client.MergeFrom(expected.DeepCopy())
		expected.Status.ObservedGeneration = ptr.To[int64](0)
		expected.Status.LastErrors = nil
		// remove operation annotation, add up-to-date timestamp annotation
		expected.Annotations = map[string]string{
			v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
		}
		expected.Status.Ready = ptr.To(true)
		Expect(c.Patch(ctx, expected, patch)).To(Succeed(), "patching etcd succeeds")

		By("Wait")
		Expect(etcd.Wait(ctx)).To(MatchError(ContainSubstring("condition AllMembersUpdated is not present")))
	})

	It("should return error if AllMembersUpdated condition is set but not true", func() {
		defer test.WithVars(
			&TimeNow, mockNow.Do,
		)()
		mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

		By("Deploy")
		// Deploy should fill internal state with the added timestamp annotation
		Expect(etcd.Deploy(ctx)).To(Succeed())

		By("Patch object")
		patch := client.MergeFrom(expected.DeepCopy())
		expected.Status.ObservedGeneration = ptr.To[int64](0)
		expected.Status.LastErrors = nil
		// remove operation annotation, add up-to-date timestamp annotation
		expected.Annotations = map[string]string{
			v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
		}
		expected.Status.Conditions = []druidcorev1alpha1.Condition{
			{
				Type:   druidcorev1alpha1.ConditionTypeAllMembersUpdated,
				Status: druidcorev1alpha1.ConditionFalse,
			},
		}
		expected.Status.Ready = ptr.To(true)
		Expect(c.Patch(ctx, expected, patch)).To(Succeed(), "patching etcd succeeds")

		By("Wait")
		Expect(etcd.Wait(ctx)).To(MatchError(ContainSubstring("condition AllMembersUpdated is False")))
	})

	It("should return error if it's not ready", func() {
		defer test.WithVars(
			&TimeNow, mockNow.Do,
		)()
		mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

		By("Deploy")
		// Deploy should fill internal state with the added timestamp annotation
		Expect(etcd.Deploy(ctx)).To(Succeed())

		By("Patch object")
		patch := client.MergeFrom(expected.DeepCopy())
		expected.Status.ObservedGeneration = ptr.To[int64](0)
		expected.Status.LastErrors = nil
		// remove operation annotation, add up-to-date timestamp annotation
		expected.Annotations = map[string]string{
			v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
		}
		expected.Status.Conditions = []druidcorev1alpha1.Condition{
			{
				Type:   druidcorev1alpha1.ConditionTypeAllMembersUpdated,
				Status: druidcorev1alpha1.ConditionTrue,
			},
		}
		expected.Status.Ready = ptr.To(false)
		Expect(c.Patch(ctx, expected, patch)).To(Succeed(), "patching etcd succeeds")

		By("Wait")
		Expect(etcd.Wait(ctx)).To(MatchError(ContainSubstring("is not ready yet")))
	})

	It("should return no error when is ready", func() {
		defer test.WithVars(
			&TimeNow, mockNow.Do,
		)()
		mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

		By("Deploy")
		// Deploy should fill internal state with the added timestamp annotation
		Expect(etcd.Deploy(ctx)).To(Succeed())

		By("Patch object")
		delete(expected.Annotations, v1beta1constants.GardenerTimestamp)
		patch := client.MergeFrom(expected.DeepCopy())
		expected.Status.ObservedGeneration = ptr.To[int64](0)
		expected.Status.LastErrors = nil
		// remove operation annotation, add up-to-date timestamp annotation
		expected.Annotations = map[string]string{
			v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
		}
		expected.Status.Conditions = []druidcorev1alpha1.Condition{
			{
				Type:   druidcorev1alpha1.ConditionTypeAllMembersUpdated,
				Status: druidcorev1alpha1.ConditionTrue,
			},
		}
		expected.Status.Ready = ptr.To(true)
		Expect(c.Patch(ctx, expected, patch)).To(Succeed(), "patching etcd succeeds")

		By("Wait")
		Expect(etcd.Wait(ctx)).To(Succeed(), "etcd is ready")
	})
})

var _ = Describe("#CheckEtcdObject", func() {
	var (
		obj *druidcorev1alpha1.Etcd
	)

	BeforeEach(func() {
		obj = &druidcorev1alpha1.Etcd{
			Spec: druidcorev1alpha1.EtcdSpec{Replicas: 3},
		}
	})

	It("should return error for non-etcd object", func() {
		Expect(CheckEtcdObject(&corev1.ConfigMap{})).NotTo(Succeed())
	})

	It("should return error if reconciliation failed", func() {
		obj.Status.LastErrors = []druidcorev1alpha1.LastError{{Code: "ERROR_FOO", Description: "foo", ObservedAt: metav1.Now()}}
		err := CheckEtcdObject(obj)
		Expect(err).To(MatchError(fmt.Sprintf("errors during reconciliation: %+v", obj.Status.LastErrors)))
		Expect(retry.IsRetriable(err)).To(BeTrue())
	})

	It("should return error if etcd is marked for deletion", func() {
		now := metav1.Now()
		obj.SetDeletionTimestamp(&now)
		Expect(CheckEtcdObject(obj)).To(MatchError("unexpectedly has a deletion timestamp"))
	})

	It("should return error if observedGeneration is not set", func() {
		Expect(CheckEtcdObject(obj)).To(MatchError("observed generation not recorded"))
	})

	It("should return error if observedGeneration is outdated", func() {
		obj.SetGeneration(1)
		obj.Status.ObservedGeneration = ptr.To[int64](0)
		Expect(CheckEtcdObject(obj)).To(MatchError("observed generation outdated (0/1)"))
	})

	It("should return error if operation annotation is not removed yet", func() {
		obj.SetGeneration(1)
		obj.Status.ObservedGeneration = ptr.To[int64](1)
		metav1.SetMetaDataAnnotation(&obj.ObjectMeta, v1beta1constants.GardenerOperation, "reconcile")
		Expect(CheckEtcdObject(obj)).To(MatchError("gardener operation \"reconcile\" is not yet picked up by etcd-druid"))
	})

	It("should not return error if replicas is set to 0, even if AllMembersUpdated condition and readiness are not true ", func() {
		obj.SetGeneration(1)
		obj.Spec.Replicas = 0
		obj.Status.ObservedGeneration = ptr.To[int64](1)
		obj.Status.Conditions = []druidcorev1alpha1.Condition{{Type: druidcorev1alpha1.ConditionTypeAllMembersUpdated, Status: druidcorev1alpha1.ConditionFalse}}
		obj.Status.Ready = ptr.To(false)
		Expect(CheckEtcdObject(obj)).To(Succeed())
	})

	It("should return error if condition AllMembersUpdated is not present", func() {
		obj.SetGeneration(1)
		obj.Status.ObservedGeneration = ptr.To[int64](1)
		Expect(CheckEtcdObject(obj)).To(MatchError("condition AllMembersUpdated is not present"))
	})

	It("should return error if condition AllMembersUpdated is not true", func() {
		obj.SetGeneration(1)
		obj.Status.ObservedGeneration = ptr.To[int64](1)
		obj.Status.Conditions = []druidcorev1alpha1.Condition{{Type: druidcorev1alpha1.ConditionTypeAllMembersUpdated, Status: druidcorev1alpha1.ConditionFalse}}
		Expect(CheckEtcdObject(obj)).To(MatchError(ContainSubstring("condition AllMembersUpdated is False")))
	})

	It("should return error if status.ready==nil", func() {
		obj.SetGeneration(1)
		obj.Status.ObservedGeneration = ptr.To[int64](1)
		obj.Status.Conditions = []druidcorev1alpha1.Condition{{Type: druidcorev1alpha1.ConditionTypeAllMembersUpdated, Status: druidcorev1alpha1.ConditionTrue}}
		Expect(CheckEtcdObject(obj)).To(MatchError("is not ready yet"))
	})

	It("should return error if status.ready==false", func() {
		obj.SetGeneration(1)
		obj.Status.ObservedGeneration = ptr.To[int64](1)
		obj.Status.Conditions = []druidcorev1alpha1.Condition{{Type: druidcorev1alpha1.ConditionTypeAllMembersUpdated, Status: druidcorev1alpha1.ConditionTrue}}
		obj.Status.Ready = ptr.To(false)
		Expect(CheckEtcdObject(obj)).To(MatchError("is not ready yet"))
	})

	It("should not return error if object is ready", func() {
		obj.SetGeneration(1)
		obj.Status.ObservedGeneration = ptr.To[int64](1)
		obj.Status.Conditions = []druidcorev1alpha1.Condition{{Type: druidcorev1alpha1.ConditionTypeAllMembersUpdated, Status: druidcorev1alpha1.ConditionTrue}}
		obj.Status.Ready = ptr.To(true)
		obj.Status.Replicas = 3
		Expect(CheckEtcdObject(obj)).To(Succeed())
	})
})
