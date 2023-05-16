// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
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

		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		fakeClock = testclock.NewFakeClock(time.Now())
		reconciler = &Reconciler{
			GardenClient: gardenClient,
			SeedClient:   seedClient,
			Config: config.ControllerInstallationCareControllerConfiguration{
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
			now := metav1.Now()
			controllerInstallation.DeletionTimestamp = &now

			Expect(gardenClient.Create(ctx, controllerInstallation)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})
	})

	Context("when care operation gets executed", func() {
		var (
			secret1 = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret1",
					Namespace: gardenNamespace,
				},
				Data: map[string][]byte{
					"foo1": []byte("bar1"),
					"foo2": []byte("bar2"),
				},
			}
			secret2 = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret2",
					Namespace: gardenNamespace,
				},
				Data: map[string][]byte{
					"foo2": []byte("bar2"),
				},
			}
			expectedChecksum = "d285aee1a9342ca3b8c7758589bda8dd7714a4e809ab95d333e54d3e3fed39bd"
		)
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

		DescribeTable("should set correct conditions when managed resource exists", func(managedResource *resourcesv1alpha1.ManagedResource, secrets []corev1.Secret, matchExpectedConditions gomegatypes.GomegaMatcher) {
			Expect(seedClient.Create(ctx, managedResource)).To(Succeed())

			for _, secret := range secrets {
				Expect(seedClient.Create(ctx, &secret)).To(Succeed())
			}

			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{
				RequeueAfter: syncPeriodDuration,
			}))

			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(controllerInstallation), controllerInstallation)).To(Succeed())
			Expect(controllerInstallation.Status.Conditions).To(matchExpectedConditions)
		},
			Entry("managed resource conditions are not set",
				managedResource([]corev1.LocalObjectReference{}, resourcesv1alpha1.ManagedResourceStatus{}),
				[]corev1.Secret{},
				ConsistOf(
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationInstalled, gardencorev1beta1.ConditionFalse, "InstallationPending"),
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationHealthy, gardencorev1beta1.ConditionFalse, "ControllerNotHealthy"),
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationProgressing, gardencorev1beta1.ConditionTrue, "ControllerNotRolledOut"),
				),
			),
			Entry("managed resource is not healthy",
				managedResource(
					[]corev1.LocalObjectReference{},
					resourcesv1alpha1.ManagedResourceStatus{
						Conditions: notHealthyConditions(),
					},
				),
				[]corev1.Secret{},
				ConsistOf(
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationInstalled, gardencorev1beta1.ConditionFalse, "InstallationPending"),
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationHealthy, gardencorev1beta1.ConditionFalse, "ControllerNotHealthy"),
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationProgressing, gardencorev1beta1.ConditionTrue, "ControllerNotRolledOut"),
				),
			),
			Entry("managed resource has no secrets data checksum set",
				managedResource(
					[]corev1.LocalObjectReference{{Name: secret1.Name}, {Name: secret2.Name}},
					resourcesv1alpha1.ManagedResourceStatus{
						Conditions: healthyConditions(),
					},
				),
				[]corev1.Secret{secret1, secret2},
				ConsistOf(
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationInstalled, gardencorev1beta1.ConditionFalse, "InstallationPending"),
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationHealthy, gardencorev1beta1.ConditionTrue, "ControllerHealthy"),
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationProgressing, gardencorev1beta1.ConditionFalse, "ControllerRolledOut"),
				),
			),
			Entry("managed resource has wrong secrets data checksum set",
				managedResource(
					[]corev1.LocalObjectReference{{Name: secret1.Name}, {Name: secret2.Name}},
					resourcesv1alpha1.ManagedResourceStatus{
						Conditions:          healthyConditions(),
						SecretsDataChecksum: pointer.String("wrong"),
					},
				),
				[]corev1.Secret{secret1, secret2},
				ConsistOf(
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationInstalled, gardencorev1beta1.ConditionFalse, "InstallationPending"),
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationHealthy, gardencorev1beta1.ConditionTrue, "ControllerHealthy"),
					conditionWithTypeStatusAndReason(gardencorev1beta1.ControllerInstallationProgressing, gardencorev1beta1.ConditionFalse, "ControllerRolledOut"),
				),
			),
			Entry("managed resource is healthy",
				managedResource(
					[]corev1.LocalObjectReference{{Name: secret1.Name}, {Name: secret2.Name}},
					resourcesv1alpha1.ManagedResourceStatus{
						Conditions:          healthyConditions(),
						SecretsDataChecksum: &expectedChecksum,
					},
				),
				[]corev1.Secret{secret1, secret2},
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
		conditionWithTypeStatusReasonAndMesssage(gardencorev1beta1.ControllerInstallationInstalled, gardencorev1beta1.ConditionUnknown, reason, message),
		conditionWithTypeStatusReasonAndMesssage(gardencorev1beta1.ControllerInstallationHealthy, gardencorev1beta1.ConditionUnknown, reason, message),
		conditionWithTypeStatusReasonAndMesssage(gardencorev1beta1.ControllerInstallationProgressing, gardencorev1beta1.ConditionUnknown, reason, message),
	)
}

func conditionWithTypeStatusAndReason(condType gardencorev1beta1.ConditionType, status gardencorev1beta1.ConditionStatus, reason string) gomegatypes.GomegaMatcher {
	return conditionWithTypeStatusReasonAndMesssage(condType, status, reason, "")
}

func conditionWithTypeStatusReasonAndMesssage(condType gardencorev1beta1.ConditionType, status gardencorev1beta1.ConditionStatus, reason, message string) gomegatypes.GomegaMatcher {
	return And(OfType(condType), WithStatus(status), WithReason(reason), WithMessage(message))
}

func healthyConditions() []gardencorev1beta1.Condition {
	return []gardencorev1beta1.Condition{
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
	}
}

func notHealthyConditions() []gardencorev1beta1.Condition {
	return []gardencorev1beta1.Condition{
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
	}
}

func managedResource(secretRefs []corev1.LocalObjectReference, status resourcesv1alpha1.ManagedResourceStatus) *resourcesv1alpha1.ManagedResource {
	return &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerInstallationName,
			Namespace: gardenNamespace,
		},
		Spec: resourcesv1alpha1.ManagedResourceSpec{
			SecretRefs: secretRefs,
		},
		Status: status,
	}
}
