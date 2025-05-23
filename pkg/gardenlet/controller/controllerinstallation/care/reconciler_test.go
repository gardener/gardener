// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/care"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

const (
	controllerInstallationName = "foo"
	gardenNamespace            = "garden"
	syncPeriodDuration         = 2 * time.Second
)

var _ = Describe("Reconciler", func() {
	var (
		ctx context.Context

		gardenClient client.Client
		seedClient   client.Client

		controllerInstallation *gardencorev1beta1.ControllerInstallation
		request                reconcile.Request

		reconciler reconcile.Reconciler
		fakeClock  *testclock.FakeClock
	)

	BeforeEach(func() {
		ctx = context.Background()

		controllerInstallation = &gardencorev1beta1.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				Name: controllerInstallationName,
			},
			Spec: gardencorev1beta1.ControllerInstallationSpec{
				SeedRef: corev1.ObjectReference{
					Name: "foo-seed",
				},
			},
		}

		request = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: controllerInstallationName,
			},
		}

		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithStatusSubresource(&gardencorev1beta1.ControllerInstallation{}).Build()
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		fakeClock = testclock.NewFakeClock(time.Now())
		reconciler = &Reconciler{
			GardenClient: gardenClient,
			SeedClient:   seedClient,
			Config: gardenletconfigv1alpha1.ControllerInstallationCareControllerConfiguration{
				SyncPeriod: &metav1.Duration{Duration: syncPeriodDuration},
			},
			Clock:           fakeClock,
			GardenNamespace: gardenNamespace,
		}
	})

	Context("when care operation does not get executed", func() {
		It("should not return error during reconciliation if ControllerInstallation resource is missing", func() {
			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should not return error during reconciliation if ControllerInstallation resource has deletionTimestamp", func() {
			Expect(gardenClient.Create(ctx, controllerInstallation)).To(Succeed())
			Expect(gardenClient.Delete(ctx, controllerInstallation)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})
	})

	Context("when care operation gets executed", func() {
		JustBeforeEach(func() {
			Expect(gardenClient.Create(ctx, controllerInstallation)).To(Succeed())
		})

		It("should set conditions to Unknown if managed resource does not exist yet", func() {
			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{
				RequeueAfter: time.Second,
			}))

			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
			Expect(controllerInstallation.Status.Conditions).To(consistOfConditionsInUnknownStatus("SeedReadError", "Failed to get ManagedResource"))
		})

		DescribeTable("should set correct conditions when managed resource exists", func(managedResource *resourcesv1alpha1.ManagedResource, matchExpectedConditions gomegatypes.GomegaMatcher) {
			Expect(seedClient.Create(ctx, managedResource)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{
				RequeueAfter: syncPeriodDuration,
			}))

			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
			Expect(controllerInstallation.Status.Conditions).To(matchExpectedConditions)
		},
			Entry("managed resource conditions are not set",
				managedResource(nil),
				ConsistOf(
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationInstalled, gardencorev1beta1.ConditionFalse, "InstallationPending"),
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationHealthy, gardencorev1beta1.ConditionFalse, "ControllerNotHealthy"),
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationProgressing, gardencorev1beta1.ConditionTrue, "ControllerNotRolledOut"),
				),
			),
			Entry("managed resource is not healthy",
				notHealthyManagedResource(),
				ConsistOf(
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationInstalled, gardencorev1beta1.ConditionFalse, "InstallationPending"),
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationHealthy, gardencorev1beta1.ConditionFalse, "ControllerNotHealthy"),
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationProgressing, gardencorev1beta1.ConditionTrue, "ControllerNotRolledOut"),
				),
			),
			Entry("managed resource is healthy",
				healthyManagedResource(),
				ConsistOf(
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationInstalled, gardencorev1beta1.ConditionTrue, "InstallationSuccessful"),
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationHealthy, gardencorev1beta1.ConditionTrue, "ControllerHealthy"),
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationProgressing, gardencorev1beta1.ConditionFalse, "ControllerRolledOut"),
				),
			),
		)
	})
})

func consistOfConditionsInUnknownStatus(reason, message string) gomegatypes.GomegaMatcher {
	return ConsistOf(
		conditionWithTypeStatusReasonAndMessage(gardencorev1beta1.ControllerInstallationInstalled, gardencorev1beta1.ConditionUnknown, reason, message),
		conditionWithTypeStatusReasonAndMessage(gardencorev1beta1.ControllerInstallationHealthy, gardencorev1beta1.ConditionUnknown, reason, message),
		conditionWithTypeStatusReasonAndMessage(gardencorev1beta1.ControllerInstallationProgressing, gardencorev1beta1.ConditionUnknown, reason, message),
	)
}

func conditionWithTypeStatusAndReason(condType gardencorev1beta1.ConditionType, status gardencorev1beta1.ConditionStatus, reason string) gomegatypes.GomegaMatcher {
	return conditionWithTypeStatusReasonAndMessage(condType, status, reason, "")
}

func conditionWithTypeStatusReasonAndMessage(condType gardencorev1beta1.ConditionType, status gardencorev1beta1.ConditionStatus, reason, message string) gomegatypes.GomegaMatcher {
	return And(OfType(condType), WithStatus(status), WithReason(reason), WithMessage(message))
}

func healthyManagedResource() *resourcesv1alpha1.ManagedResource {
	return managedResource(
		[]gardencorev1beta1.Condition{
			{
				Type:   resourcesv1alpha1.ResourcesApplied,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   resourcesv1alpha1.ResourcesHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   resourcesv1alpha1.ResourcesProgressing,
				Status: gardencorev1beta1.ConditionFalse,
			},
		})
}

func notHealthyManagedResource() *resourcesv1alpha1.ManagedResource {
	return managedResource(
		[]gardencorev1beta1.Condition{
			{
				Type:    resourcesv1alpha1.ResourcesApplied,
				Reason:  "NotApplied",
				Message: "Resources are not applied",
				Status:  gardencorev1beta1.ConditionFalse,
			},
			{
				Type:    resourcesv1alpha1.ResourcesHealthy,
				Reason:  "NotHealthy",
				Message: "Resources are not healthy",
				Status:  gardencorev1beta1.ConditionFalse,
			},
			{
				Type:    resourcesv1alpha1.ResourcesProgressing,
				Reason:  "ResourcesProgressing",
				Message: "Resources are progressing",
				Status:  gardencorev1beta1.ConditionTrue,
			},
		})
}

func managedResource(conditions []gardencorev1beta1.Condition) *resourcesv1alpha1.ManagedResource {
	return &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerInstallationName,
			Namespace: gardenNamespace,
		},
		Status: resourcesv1alpha1.ManagedResourceStatus{
			Conditions: conditions,
		},
	}
}
