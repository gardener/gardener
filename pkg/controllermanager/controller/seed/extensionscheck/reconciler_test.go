// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensionscheck_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/seed/extensionscheck"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Reconciler", func() {
	const (
		seedName           = "test"
		syncPeriodDuration = 30 * time.Second
	)

	var (
		ctx context.Context
		c   client.Client

		seed                    *gardencorev1beta1.Seed
		controllerInstallations []*gardencorev1beta1.ControllerInstallation
		matchExpectedCondition  types.GomegaMatcher

		fakeClock *testclock.FakeClock

		reconciler reconcile.Reconciler
		request    reconcile.Request
	)

	BeforeEach(func() {
		ctx = context.Background()
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{Name: seedName},
		}
		request = reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)}

		fakeClock = testclock.NewFakeClock(time.Now().Round(time.Second))

		c = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.GardenScheme).
			WithObjects(seed).
			WithStatusSubresource(seed).
			WithIndex(&gardencorev1beta1.ControllerInstallation{}, core.SeedRefName, indexer.ControllerInstallationSeedRefNameIndexerFunc).
			Build()
		conf := controllermanagerconfigv1alpha1.SeedExtensionsCheckControllerConfiguration{
			SyncPeriod: &metav1.Duration{Duration: syncPeriodDuration},
		}
		reconciler = &Reconciler{
			Client: c,
			Config: conf,
			Clock:  fakeClock,
		}

		matchExpectedCondition = matchConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionTrue, "AllExtensionsReady", "All extensions installed into the seed cluster are ready and healthy.")
	})

	JustBeforeEach(func() {
		for _, obj := range controllerInstallations {
			Expect(c.Create(ctx, obj)).To(Succeed())
		}
	})

	AfterEach(func() {
		if err := c.Get(ctx, request.NamespacedName, seed); !apierrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred())
			Expect(seed.Status.Conditions).To(ConsistOf(matchExpectedCondition))
		}
	})

	It("should do nothing if Seed is gone", func() {
		Expect(c.Delete(ctx, seed)).To(Succeed())
		Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{}))
	})

	Context("no ControllerInstallations exist", func() {
		It("should set ExtensionsReady to True (AllExtensionsReady)", func() {
			result, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{RequeueAfter: syncPeriodDuration}))
		})
	})

	Context("all ControllerInstallations are not installed", func() {
		BeforeEach(func() {
			matchExpectedCondition = matchConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, "NotAllExtensionsInstalled", `Some extensions are not installed: map[foo-1:extension was not yet installed foo-3:extension was not yet installed]`)

			c1 := &gardencorev1beta1.ControllerInstallation{}
			c1.SetName("foo-1")
			c1.Spec.SeedRef.Name = seedName

			c2 := c1.DeepCopy()
			c2.SetName("foo-2")
			c2.Spec.SeedRef.Name = "not-seed-2"

			c3 := c1.DeepCopy()
			c3.SetName("foo-3")

			controllerInstallations = []*gardencorev1beta1.ControllerInstallation{c1, c2, c3}
		})

		It("should set ExtensionsReady to False (NotAllExtensionsInstalled)", func() {
			Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{RequeueAfter: syncPeriodDuration}))
		})
	})

	Context("all ControllerInstallations valid, installed and healthy", func() {
		BeforeEach(func() {
			c1 := &gardencorev1beta1.ControllerInstallation{}
			c1.SetName("foo-1")
			c1.Spec.SeedRef.Name = seedName
			c1.Status.Conditions = []gardencorev1beta1.Condition{
				{Type: "Valid", Status: gardencorev1beta1.ConditionTrue},
				{Type: "Installed", Status: gardencorev1beta1.ConditionTrue},
				{Type: "Healthy", Status: gardencorev1beta1.ConditionTrue},
				{Type: "Progressing", Status: gardencorev1beta1.ConditionFalse},
				{Type: "RandomType", Status: gardencorev1beta1.ConditionTrue},
				{Type: "AnotherRandomType", Status: gardencorev1beta1.ConditionFalse},
			}

			c2 := c1.DeepCopy()
			c2.SetName("foo-2")

			controllerInstallations = []*gardencorev1beta1.ControllerInstallation{c1, c2}
		})

		It("should set ExtensionsReady to True (AllExtensionsReady)", func() {
			Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{RequeueAfter: syncPeriodDuration}))
		})

		It("should update ExtensionsReady condition if it already exists", func() {
			existingCondition := gardencorev1beta1.Condition{
				Type:               "ExtensionsReady",
				Status:             gardencorev1beta1.ConditionFalse,
				Reason:             "NotAllExtensionsInstalled",
				Message:            `Some extensions are not installed: map[foo-1:extension was not yet installed foo-3:extension was not yet installed]`,
				LastTransitionTime: metav1.NewTime(fakeClock.Now().Add(-time.Minute)),
				LastUpdateTime:     metav1.NewTime(fakeClock.Now().Add(-time.Minute)),
			}
			seed.Status.Conditions = []gardencorev1beta1.Condition{existingCondition}
			Expect(c.Status().Update(ctx, seed)).To(Succeed())

			Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{RequeueAfter: syncPeriodDuration}))
		})
	})

	Context("when ControllerInstallation conditions are not successful", func() {
		var tests = func(failedCondition gardencorev1beta1.Condition, reason, message string) {
			BeforeEach(func() {
				c1 := &gardencorev1beta1.ControllerInstallation{}
				c1.SetName("foo-1")
				c1.Spec.SeedRef.Name = seedName
				c1.Status.Conditions = []gardencorev1beta1.Condition{
					{Type: "Valid", Status: gardencorev1beta1.ConditionTrue},
					{Type: "Installed", Status: gardencorev1beta1.ConditionTrue},
					{Type: "Healthy", Status: gardencorev1beta1.ConditionTrue},
					{Type: "Progressing", Status: gardencorev1beta1.ConditionFalse},
					{Type: "RandomType", Status: gardencorev1beta1.ConditionTrue},
					{Type: "AnotherRandomType", Status: gardencorev1beta1.ConditionFalse},
				}

				c2 := c1.DeepCopy()
				c2.SetName("foo-2")
				for i, condition := range c2.Status.Conditions {
					if condition.Type == failedCondition.Type {
						c2.Status.Conditions[i].Status = failedCondition.Status
					}
				}

				controllerInstallations = []*gardencorev1beta1.ControllerInstallation{c1, c2}
			})

			It("should set ExtensionsReady to False", func() {
				matchExpectedCondition = matchConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, reason, message)
				Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{RequeueAfter: syncPeriodDuration}))
			})

			Context("when ExtensionsReady condition threshold is set", func() {
				BeforeEach(func() {
					conf := controllermanagerconfigv1alpha1.SeedExtensionsCheckControllerConfiguration{
						SyncPeriod: &metav1.Duration{Duration: syncPeriodDuration},
						ConditionThresholds: []controllermanagerconfigv1alpha1.ConditionThreshold{{
							Type:     string(gardencorev1beta1.SeedExtensionsReady),
							Duration: metav1.Duration{Duration: time.Minute},
						}},
					}
					reconciler = &Reconciler{
						Client: c,
						Config: conf,
						Clock:  fakeClock,
					}
				})

				It("should set ExtensionsReady to Progressing if it was previously True", func() {
					existingCondition := gardencorev1beta1.Condition{
						Type:               "ExtensionsReady",
						Status:             gardencorev1beta1.ConditionTrue,
						Reason:             "AllExtensionsReady",
						Message:            "All extensions installed into the seed cluster are ready and healthy.",
						LastTransitionTime: metav1.NewTime(fakeClock.Now().Add(-time.Second)),
						LastUpdateTime:     metav1.NewTime(fakeClock.Now().Add(-time.Second)),
					}
					seed.Status.Conditions = []gardencorev1beta1.Condition{existingCondition}
					Expect(c.Status().Update(ctx, seed)).To(Succeed())

					matchExpectedCondition = matchConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message)
					Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{RequeueAfter: syncPeriodDuration}))
				})

				It("should set ExtensionsReady to False if it was previously Progressing and threshold has expired", func() {
					existingCondition := gardencorev1beta1.Condition{
						Type:               "ExtensionsReady",
						Status:             gardencorev1beta1.ConditionProgressing,
						Reason:             reason,
						Message:            message,
						LastTransitionTime: metav1.NewTime(fakeClock.Now().Add(-90 * time.Second)),
						LastUpdateTime:     metav1.NewTime(fakeClock.Now().Add(-90 * time.Second)),
					}
					seed.Status.Conditions = []gardencorev1beta1.Condition{existingCondition}
					Expect(c.Status().Update(ctx, seed)).To(Succeed())

					matchExpectedCondition = matchConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, reason, message)
					Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{RequeueAfter: syncPeriodDuration}))
				})
			})
		}

		Context("one ControllerInstallations is invalid", func() {
			tests(
				gardencorev1beta1.Condition{Type: gardencorev1beta1.ControllerInstallationValid, Status: gardencorev1beta1.ConditionFalse},
				"NotAllExtensionsValid",
				`Some extensions are not valid: map[foo-2:]`,
			)
		})

		Context("one ControllerInstallation is not installed", func() {
			tests(
				gardencorev1beta1.Condition{Type: gardencorev1beta1.ControllerInstallationInstalled, Status: gardencorev1beta1.ConditionFalse},
				"NotAllExtensionsInstalled",
				`Some extensions are not installed: map[foo-2:]`,
			)
		})

		Context("one ControllerInstallation is not healthy", func() {
			tests(
				gardencorev1beta1.Condition{Type: gardencorev1beta1.ControllerInstallationHealthy, Status: gardencorev1beta1.ConditionFalse},
				"NotAllExtensionsHealthy",
				`Some extensions are not healthy: map[foo-2:]`,
			)
		})

		Context("one ControllerInstallation is still progressing", func() {
			tests(
				gardencorev1beta1.Condition{Type: gardencorev1beta1.ControllerInstallationProgressing, Status: gardencorev1beta1.ConditionTrue},
				"SomeExtensionsProgressing",
				`Some extensions are progressing: map[foo-2:]`,
			)
		})
	})
})

func matchConditionWithStatusReasonAndMessage(status gardencorev1beta1.ConditionStatus, reason, message string) types.GomegaMatcher {
	return And(OfType(gardencorev1beta1.SeedExtensionsReady), WithStatus(status), WithReason(reason), WithMessage(message))
}
