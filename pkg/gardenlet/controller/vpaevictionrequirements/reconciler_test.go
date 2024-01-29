// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpaevictionrequirements_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/controller/vpaevictionrequirements"
	"github.com/gardener/gardener/pkg/utils/timewindow"
)

var _ = Describe("Reconciler", func() {
	var (
		ctx                    = context.TODO()
		reconciler             reconcile.Reconciler
		fakeClock              = testclock.NewFakeClock(time.Now())
		request                reconcile.Request
		seedClient             client.Client
		vpa                    *vpaautoscalingv1.VerticalPodAutoscaler
		upscaleOnlyRequirement = []*vpaautoscalingv1.EvictionRequirement{{
			Resources:         []corev1.ResourceName{corev1.ResourceMemory, corev1.ResourceCPU},
			ChangeRequirement: vpaautoscalingv1.TargetHigherThanRequests,
		}}
		maintenanceWindowBegin string
		maintenanceWindowEnd   string
		maintenanceTimeWindow  *timewindow.MaintenanceTimeWindow
	)

	BeforeEach(func() {
		testSchemeBuilder := runtime.NewSchemeBuilder(
			kubernetes.AddGardenSchemeToScheme,
			vpaautoscalingv1.AddToScheme,
		)
		testScheme := runtime.NewScheme()
		Expect(testSchemeBuilder.AddToScheme(testScheme)).To(Succeed())

		seedClient = fakeclient.NewClientBuilder().WithScheme(testScheme).Build()
		reconciler = &vpaevictionrequirements.Reconciler{
			Config:     config.VPAEvictionRequirementsControllerConfiguration{ConcurrentSyncs: ptr.To(5)},
			Clock:      fakeClock,
			SeedClient: seedClient,
		}

		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vpa",
				Namespace: "test-namespace",
				Labels: map[string]string{
					constants.LabelVPAEvictionRequirementsController: constants.EvictionRequirementManagedByController,
				},
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{},
			},
		}
	})

	JustBeforeEach(func() {
		Expect(seedClient.Create(ctx, vpa)).To(Succeed())

		request = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      vpa.Name,
				Namespace: vpa.Namespace,
			},
		}
	})

	Context("VPA is annotated with downscale-in-maintenance-window-only", func() {
		BeforeEach(func() {
			metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, constants.AnnotationVPAEvictionRequirementDownscaleRestriction, constants.EvictionRequirementInMaintenanceWindowOnly)
		})

		When("the Shoot is outside its maintenance window", func() {
			BeforeEach(func() {
				maintenanceWindowBegin = fakeClock.Now().Add(5 * time.Hour).Format("150405-0700")
				maintenanceWindowEnd = fakeClock.Now().Add(6 * time.Hour).Format("150405-0700")
				maintenanceTimeWindow, _ = timewindow.ParseMaintenanceTimeWindow(maintenanceWindowBegin, maintenanceWindowEnd)
				metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, constants.AnnotationShootMaintenanceWindow, maintenanceWindowBegin+","+maintenanceWindowEnd)
			})

			It("should add an EvictionRequirement that prevents downscaling and requeue at the beginning of the next Shoot maintenance window", func() {
				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(maintenanceTimeWindow.AdjustedBegin(fakeClock.Now()).Sub(fakeClock.Now())))

				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(vpa), vpa)).To(Succeed())
				Expect(vpa.Spec.UpdatePolicy.EvictionRequirements).To(ConsistOf(upscaleOnlyRequirement))
			})
		})

		When("the Shoot is inside its maintenance window", func() {
			BeforeEach(func() {
				maintenanceWindowBegin = fakeClock.Now().Format("150405-0700")
				maintenanceWindowEnd = fakeClock.Now().Add(1 * time.Hour).Format("150405-0700")
				maintenanceTimeWindow, _ = timewindow.ParseMaintenanceTimeWindow(maintenanceWindowBegin, maintenanceWindowEnd)
				metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, constants.AnnotationShootMaintenanceWindow, maintenanceWindowBegin+","+maintenanceWindowEnd)
				vpa.Spec.UpdatePolicy.EvictionRequirements = upscaleOnlyRequirement
			})

			It("should remove the EvictionRequirement to allow downscaling and requeue for the end of the maintenance window", func() {
				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(maintenanceTimeWindow.AdjustedEnd(fakeClock.Now()).Sub(fakeClock.Now())))

				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(vpa), vpa)).To(Succeed())
				Expect(vpa.Spec.UpdatePolicy.EvictionRequirements).To(BeEmpty())
			})
		})
	})

	Context("the VPA is annotated with downscale-never", func() {
		BeforeEach(func() {
			metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, constants.AnnotationVPAEvictionRequirementDownscaleRestriction, constants.EvictionRequirementNever)
		})

		It("should add an Evictionrequirement that prevents downscaling and not requeue", func() {
			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(vpa), vpa)).To(Succeed())
			Expect(vpa.Spec.UpdatePolicy.EvictionRequirements).To(ConsistOf(upscaleOnlyRequirement))
		})
	})

	Context("VPA is not annotated with a downscale-restriction", func() {
		It("should log an error, but not return it, such that it doesn't retry", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Eventually(logBuffer).Should(gbytes.Say(fmt.Sprintf("annotation %s not found, although marker label %s is present", constants.AnnotationVPAEvictionRequirementDownscaleRestriction, constants.LabelVPAEvictionRequirementsController)))
		})
	})

	Context("VPA is not annotated with maintenance window, although downscale-restriction is set to in-maintenance-window-only", func() {
		BeforeEach(func() {
			metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, constants.AnnotationVPAEvictionRequirementDownscaleRestriction, constants.EvictionRequirementInMaintenanceWindowOnly)
		})

		It("should log an error, but not return it, such that it doesn't retry", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Eventually(logBuffer).Should(gbytes.Say("didn't find maintenance window annotation, but VPA had label to be downscaled in maintenance only"))
		})
	})

	Context("VPA is annotated incorrectly: maintenance window isn't splittable in <start>,<end>", func() {
		BeforeEach(func() {
			metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, constants.AnnotationVPAEvictionRequirementDownscaleRestriction, constants.EvictionRequirementInMaintenanceWindowOnly)
			metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, constants.AnnotationShootMaintenanceWindow, maintenanceWindowBegin+maintenanceWindowEnd)
		})

		It("should log an error, but not return it, such that it doesn't retry", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Eventually(logBuffer).Should(gbytes.Say("error during parsing the maintenance window from annotation. Value is not in format '<begin>,<end>"))
		})
	})

	Context("VPA is annotated with an un-parsable maintenance window time", func() {
		BeforeEach(func() {
			metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, constants.AnnotationVPAEvictionRequirementDownscaleRestriction, constants.EvictionRequirementInMaintenanceWindowOnly)
			metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, constants.AnnotationShootMaintenanceWindow, "unparseable start time"+","+maintenanceWindowEnd)
		})

		It("should log an error, but not return it, such that it doesn't retry", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Eventually(logBuffer).Should(gbytes.Say("Error during parsing the maintenance window from start and end time"))
		})
	})
})
