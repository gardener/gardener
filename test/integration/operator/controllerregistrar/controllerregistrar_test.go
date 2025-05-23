// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerregistrar_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/operator/controller/controllerregistrar"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Controller Registrar controller tests", Ordered, func() {
	var (
		operatorCtx, operatorCancel = context.WithCancel(ctx)
		garden                      *operatorv1alpha1.Garden

		controller1      *testController
		controller1Added bool

		controller2      *testController
		controller2Added bool
	)

	BeforeEach(OncePerOrdered, func() {
		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name:   gardenName,
				Labels: map[string]string{testID: testRunID},
			},
			Spec: operatorv1alpha1.GardenSpec{
				RuntimeCluster: operatorv1alpha1.RuntimeCluster{
					Networking: operatorv1alpha1.RuntimeNetworking{
						Pods:     []string{"10.1.0.0/16"},
						Services: []string{"10.2.0.0/16"},
					},
					Ingress: operatorv1alpha1.Ingress{
						Domains: []operatorv1alpha1.DNSDomain{{Name: "ingress.runtime-garden.local.gardener.cloud"}},
						Controller: gardencorev1beta1.IngressController{
							Kind: "nginx",
						},
					},
					Provider: operatorv1alpha1.Provider{
						Zones: []string{"a", "b", "c"},
					},
					Settings: &operatorv1alpha1.Settings{
						VerticalPodAutoscaler: &operatorv1alpha1.SettingVerticalPodAutoscaler{
							Enabled: ptr.To(true),
						},
					},
				},
				VirtualCluster: operatorv1alpha1.VirtualCluster{
					DNS: operatorv1alpha1.DNS{
						Domains: []operatorv1alpha1.DNSDomain{{Name: "virtual-garden.local.gardener.cloud"}},
					},
					Gardener: operatorv1alpha1.Gardener{
						ClusterIdentity: "test",
					},
					Kubernetes: operatorv1alpha1.Kubernetes{
						Version: "1.26.3",
					},
					Maintenance: operatorv1alpha1.Maintenance{
						TimeWindow: gardencorev1beta1.MaintenanceTimeWindow{
							Begin: "220000+0100",
							End:   "230000+0100",
						},
					},
					Networking: operatorv1alpha1.Networking{
						Services: []string{"100.64.0.0/13"},
					},
				},
			},
		}

		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, garden)).To(Or(Succeed(), BeNotFoundError()))
		})

		controller1 = &testController{}
		controller2 = &testController{}

		Expect((&controllerregistrar.Reconciler{
			OperatorCancel: operatorCancel,
			Controllers: []controllerregistrar.Controller{
				{
					Name: testControllerName + "-1",
					AddToManagerFunc: func(_ context.Context, manager manager.Manager, _ *operatorv1alpha1.Garden) (bool, error) {
						controller1Added = true
						return true, controller1.AddToManager(manager, testControllerName+"-1")
					},
				},
				{
					Name: testControllerName + "-2",
					AddToManagerFunc: func(_ context.Context, manager manager.Manager, garden *operatorv1alpha1.Garden) (bool, error) {
						if !metav1.HasAnnotation(garden.ObjectMeta, "test.gardener.cloud/continue") {
							return false, nil
						}

						controller2Added = true
						return true, controller2.AddToManager(manager, testControllerName+"-2")
					},
				},
			},
		}).AddToManager(mgr)).To(Succeed())
	})

	Describe("Test controller registration", Ordered, func() {
		It("should have the controllers not being registered", func() {
			Consistently(func() bool { return controller1.reconciled }).Should(BeFalse())
			Consistently(func() bool { return controller2.reconciled }).Should(BeFalse())
		})

		It("should register the first controller once the garden is created", func() {
			Expect(testClient.Create(ctx, garden)).To(Succeed())
			log.Info("Created Garden for test", "garden", garden.Name)

			Eventually(func() bool { return controller1Added }).Should(BeTrue())
			Expect(controller2Added).To(BeFalse())

			Expect(testClient.Update(ctx, garden)).To(Succeed())
			Eventually(func() bool { return controller1.reconciled }).Should(BeTrue())
			Expect(controller2.reconciled).To(BeFalse())
		})

		It("should register the second controller once the garden is prepared", func() {
			metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "test.gardener.cloud/continue", "true")
			Expect(testClient.Update(ctx, garden)).To(Succeed())

			Eventually(func() bool { return controller2Added }).Should(BeTrue())
			Eventually(func() bool { return controller2.reconciled }).Should(BeTrue())
		})

		It("should cancel the operator when garden is deleted", func() {
			Expect(testClient.Delete(ctx, garden)).To(Succeed())

			Eventually(operatorCtx.Done()).Should(BeClosed())
		})
	})
})

const testControllerName = "test-controller"

type testController struct {
	reconciled bool
}

func (t *testController) AddToManager(manager manager.Manager, controllerName string) error {
	return builder.
		ControllerManagedBy(manager).
		Named(controllerName).
		For(&operatorv1alpha1.Garden{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(t)
}

func (t *testController) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)
	log.V(1).Info("Reconcile start")

	t.reconciled = true
	return reconcile.Result{}, nil
}
