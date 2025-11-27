// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clusterfinalizer_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/clusterfinalizer"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Reconciler", func() {
	var (
		ctx = context.Background()

		ctrl *gomock.Controller
		c    client.Client

		reconciler *Reconciler
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.GardenScheme).
			WithIndex(&gardencorev1beta1.ControllerInstallation{}, core.SeedRefName, indexer.ControllerInstallationSeedRefNameIndexerFunc).
			WithIndex(&gardencorev1beta1.ControllerInstallation{}, core.ShootRefName, indexer.ControllerInstallationShootRefNameIndexerFunc).
			WithIndex(&gardencorev1beta1.ControllerInstallation{}, core.ShootRefNamespace, indexer.ControllerInstallationShootRefNamespaceIndexerFunc).
			Build()

		reconciler = &Reconciler{
			Client: c,
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("for Seeds", func() {
		var (
			seedName = "seed"
			seed     *gardencorev1beta1.Seed
		)

		BeforeEach(func() {
			reconciler.NewTargetObjectFunc = func() client.Object {
				return &gardencorev1beta1.Seed{}
			}
			reconciler.NewControllerInstallationSelector = func(obj client.Object) client.MatchingFields {
				return client.MatchingFields{core.SeedRefName: obj.GetName()}
			}
			seed = &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
			}
		})

		Describe("#Reconcile", func() {
			It("should return nil because object not found", func() {
				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
			})

			Context("deletion timestamp not set", func() {
				BeforeEach(func() {
					Expect(c.Create(ctx, seed)).To(Succeed())
				})

				It("should ensure the finalizer", func() {
					result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}})
					Expect(result).To(Equal(reconcile.Result{}))
					Expect(err).NotTo(HaveOccurred())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
					Expect(seed.Finalizers).To(ConsistOf("core.gardener.cloud/controllerregistration"))
				})
			})

			Context("deletion timestamp set", func() {
				BeforeEach(func() {
					Expect(c.Create(ctx, seed)).To(Succeed())
					result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}})
					Expect(result).To(Equal(reconcile.Result{}))
					Expect(err).NotTo(HaveOccurred())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
					Expect(seed.Finalizers).To(ConsistOf("core.gardener.cloud/controllerregistration"))

					Expect(c.Delete(ctx, seed)).To(Succeed())
				})

				It("should not remove finalizer while installation referencing seed exists", func() {
					controllerInstallation := &gardencorev1beta1.ControllerInstallation{
						ObjectMeta: metav1.ObjectMeta{
							Name: "controllerInstallation",
						},
						Spec: gardencorev1beta1.ControllerInstallationSpec{
							SeedRef: &corev1.ObjectReference{
								Name: seedName,
							},
						},
					}

					Expect(c.Create(ctx, controllerInstallation)).To(Succeed())

					result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}})
					Expect(result).To(Equal(reconcile.Result{}))
					Expect(err).NotTo(HaveOccurred())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
					Expect(seed.Finalizers).To(ConsistOf("core.gardener.cloud/controllerregistration"))
				})

				It("should remove the finalizer", func() {
					result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}})
					Expect(result).To(Equal(reconcile.Result{}))
					Expect(err).NotTo(HaveOccurred())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(BeNotFoundError())
				})
			})
		})
	})

	Context("for Shoots", func() {
		var (
			shootName      = "shoot"
			shootNamespace = "shoot-namespace"
			shoot          *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			reconciler.NewTargetObjectFunc = func() client.Object {
				return &gardencorev1beta1.Shoot{}
			}
			reconciler.NewControllerInstallationSelector = func(obj client.Object) client.MatchingFields {
				return client.MatchingFields{core.ShootRefName: obj.GetName(), core.ShootRefNamespace: obj.GetNamespace()}
			}
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: shootNamespace,
				},
			}
		})

		Describe("#Reconcile", func() {
			It("should return nil because object not found", func() {
				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: shootName, Namespace: shootNamespace}})
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())
			})

			Context("deletion timestamp not set", func() {
				BeforeEach(func() {
					Expect(c.Create(ctx, shoot)).To(Succeed())
				})

				It("should ensure the finalizer", func() {
					result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: shootName, Namespace: shootNamespace}})
					Expect(result).To(Equal(reconcile.Result{}))
					Expect(err).NotTo(HaveOccurred())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					Expect(shoot.Finalizers).To(ConsistOf("core.gardener.cloud/controllerregistration"))
				})
			})

			Context("deletion timestamp set", func() {
				BeforeEach(func() {
					Expect(c.Create(ctx, shoot)).To(Succeed())
					result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: shootName, Namespace: shootNamespace}})
					Expect(result).To(Equal(reconcile.Result{}))
					Expect(err).NotTo(HaveOccurred())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					Expect(shoot.Finalizers).To(ConsistOf("core.gardener.cloud/controllerregistration"))

					Expect(c.Delete(ctx, shoot)).To(Succeed())
				})

				It("should not remove finalizer while installation referencing shoot exists", func() {
					controllerInstallation := &gardencorev1beta1.ControllerInstallation{
						ObjectMeta: metav1.ObjectMeta{
							Name: "controllerInstallation",
						},
						Spec: gardencorev1beta1.ControllerInstallationSpec{
							ShootRef: &corev1.ObjectReference{
								Name:      shootName,
								Namespace: shootNamespace,
							},
						},
					}

					Expect(c.Create(ctx, controllerInstallation)).To(Succeed())

					result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: shootName, Namespace: shootNamespace}})
					Expect(result).To(Equal(reconcile.Result{}))
					Expect(err).NotTo(HaveOccurred())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					Expect(shoot.Finalizers).To(ConsistOf("core.gardener.cloud/controllerregistration"))
				})

				It("should remove the finalizer", func() {
					result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: shootName, Namespace: shootNamespace}})
					Expect(result).To(Equal(reconcile.Result{}))
					Expect(err).NotTo(HaveOccurred())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(BeNotFoundError())
				})
			})
		})
	})
})
