// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package required_test

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/required"
)

var _ = Describe("Add", func() {
	var (
		reconciler     *Reconciler
		infrastructure *extensionsv1alpha1.Infrastructure
	)

	BeforeEach(func() {
		reconciler = &Reconciler{}
		infrastructure = &extensionsv1alpha1.Infrastructure{
			Spec: extensionsv1alpha1.InfrastructureSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: "type",
				},
			},
		}
	})

	Describe("#MapObjectKindToControllerInstallations", func() {
		var (
			ctx              = context.TODO()
			log              = logr.Discard()
			fakeGardenClient client.Client
			fakeSeedClient   client.Client
			mapFn            handler.MapFunc

			infrastructure2                                                           *extensionsv1alpha1.Infrastructure
			controllerRegistration1, controllerRegistration2, controllerRegistration3 *gardencorev1beta1.ControllerRegistration
			controllerInstallation1, controllerInstallation2, controllerInstallation3 *gardencorev1beta1.ControllerInstallation

			type1, type2 = "foo", "bar"
			seedName     = "seed"
		)

		BeforeEach(func() {
			fakeGardenClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithIndex(&gardencorev1beta1.ControllerInstallation{}, core.SeedRefName, indexer.ControllerInstallationSeedRefNameIndexerFunc).
				Build()
			fakeSeedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			mapFn = reconciler.MapObjectKindToControllerInstallations(log, extensionsv1alpha1.InfrastructureResource, func() client.ObjectList { return &extensionsv1alpha1.InfrastructureList{} })

			reconciler.GardenClient = fakeGardenClient
			reconciler.SeedClient = fakeSeedClient
			reconciler.SeedName = seedName
			reconciler.Lock = &sync.RWMutex{}
			reconciler.KindToRequiredTypes = make(map[string]sets.Set[string])

			controllerRegistration1 = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "reg1"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: extensionsv1alpha1.InfrastructureResource, Type: type1},
					},
				},
			}
			controllerRegistration2 = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "reg2"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: extensionsv1alpha1.InfrastructureResource, Type: type2},
					},
				},
			}
			controllerRegistration3 = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "reg3"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: extensionsv1alpha1.ControlPlaneResource, Type: "foo"},
					},
				},
			}

			controllerInstallation1 = &gardencorev1beta1.ControllerInstallation{
				ObjectMeta: metav1.ObjectMeta{Name: "inst1"},
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					RegistrationRef: corev1.ObjectReference{Name: controllerRegistration1.Name},
					SeedRef:         corev1.ObjectReference{Name: seedName},
				},
			}
			controllerInstallation2 = &gardencorev1beta1.ControllerInstallation{
				ObjectMeta: metav1.ObjectMeta{Name: "inst2"},
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					RegistrationRef: corev1.ObjectReference{Name: controllerRegistration2.Name},
					SeedRef:         corev1.ObjectReference{Name: seedName},
				},
			}
			controllerInstallation3 = &gardencorev1beta1.ControllerInstallation{
				ObjectMeta: metav1.ObjectMeta{Name: "inst3"},
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					RegistrationRef: corev1.ObjectReference{Name: controllerRegistration3.Name},
					SeedRef:         corev1.ObjectReference{Name: seedName},
				},
			}

			infrastructure2 = infrastructure.DeepCopy()
			infrastructure.Name = "infra1"
			infrastructure.Spec.Type = type1
			infrastructure2.Name = "infra2"
			infrastructure2.Spec.Type = type2
		})

		It("should do nothing when there are no infrastructure resources", func() {
			Expect(mapFn(ctx, nil)).To(BeEmpty())
			Expect(reconciler.KindToRequiredTypes).To(HaveKeyWithValue(extensionsv1alpha1.InfrastructureResource, sets.New[string]()))
		})

		It("should do nothing when there are no controllerregistration resources", func() {
			Expect(fakeGardenClient.Create(ctx, controllerInstallation1)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, controllerInstallation2)).To(Succeed())

			Expect(fakeSeedClient.Create(ctx, infrastructure)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, infrastructure2)).To(Succeed())

			Expect(mapFn(ctx, nil)).To(BeEmpty())
			Expect(reconciler.KindToRequiredTypes).To(HaveKeyWithValue(extensionsv1alpha1.InfrastructureResource, sets.New(type1, type2)))
		})

		It("should do nothing when there are no controllerinstallation resources", func() {
			Expect(fakeGardenClient.Create(ctx, controllerRegistration1)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, controllerRegistration2)).To(Succeed())

			Expect(fakeSeedClient.Create(ctx, infrastructure)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, infrastructure2)).To(Succeed())

			Expect(mapFn(ctx, nil)).To(BeEmpty())
			Expect(reconciler.KindToRequiredTypes).To(HaveKeyWithValue(extensionsv1alpha1.InfrastructureResource, sets.New(type1, type2)))
		})

		It("should return the expected names of controllerinstallations", func() {
			Expect(fakeGardenClient.Create(ctx, controllerRegistration1)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, controllerRegistration2)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, controllerRegistration3)).To(Succeed())

			Expect(fakeGardenClient.Create(ctx, controllerInstallation1)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, controllerInstallation2)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, controllerInstallation3)).To(Succeed())

			Expect(fakeSeedClient.Create(ctx, infrastructure)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, infrastructure2)).To(Succeed())

			Expect(mapFn(ctx, nil)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerInstallation1.Name}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerInstallation2.Name}},
			))
			Expect(reconciler.KindToRequiredTypes).To(HaveKeyWithValue(extensionsv1alpha1.InfrastructureResource, sets.New(type1, type2)))
		})

		It("should correctly calculate the kind-to-types map and return the expected extension in the requests", func() {
			Expect(fakeGardenClient.Create(ctx, controllerRegistration1)).To(Succeed())
			Expect(fakeGardenClient.Create(ctx, controllerInstallation1)).To(Succeed())

			By("Invoke mapper the first time and expect requests")
			Expect(fakeSeedClient.Create(ctx, infrastructure)).To(Succeed())

			Expect(mapFn(ctx, nil)).To(ConsistOf(Equal(reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerInstallation1.Name}})))
			Expect(reconciler.KindToRequiredTypes).To(HaveKeyWithValue(extensionsv1alpha1.InfrastructureResource, sets.New(type1)))

			By("Invoke mapper again w/o changes and expect no requests")
			Expect(mapFn(ctx, nil)).To(BeEmpty())
			Expect(reconciler.KindToRequiredTypes).To(HaveKeyWithValue(extensionsv1alpha1.InfrastructureResource, sets.New(type1)))

			By("Delete infrastructure and expect the extension in the requests")
			Expect(fakeSeedClient.Delete(ctx, infrastructure)).To(Succeed())
			Expect(mapFn(ctx, nil)).To(ConsistOf(Equal(reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerInstallation1.Name}})))
			Expect(reconciler.KindToRequiredTypes).To(HaveKeyWithValue(extensionsv1alpha1.InfrastructureResource, sets.New[string]()))

			By("Create a infrastructure with class garden and expect no requests")
			infrastructureGarden := infrastructure.DeepCopy()
			infrastructureGarden.ResourceVersion = ""
			infrastructureGarden.Spec.Class = ptr.To(extensionsv1alpha1.ExtensionClassGarden)
			Expect(fakeSeedClient.Create(ctx, infrastructureGarden)).To(Succeed())
			Expect(mapFn(ctx, nil)).To(BeEmpty())
			Expect(reconciler.KindToRequiredTypes).To(HaveKeyWithValue(extensionsv1alpha1.InfrastructureResource, sets.New[string]()))

			By("Create a infrastructure with class seed and expect requests")
			infrastructureSeed := infrastructure.DeepCopy()
			infrastructureSeed.ResourceVersion = ""
			infrastructureSeed.Name = "infra-seed"
			infrastructureSeed.Spec.Class = ptr.To(extensionsv1alpha1.ExtensionClassSeed)
			Expect(fakeSeedClient.Create(ctx, infrastructureSeed)).To(Succeed())
			Expect(mapFn(ctx, nil)).To(ConsistOf(Equal(reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerInstallation1.Name}})))
			Expect(reconciler.KindToRequiredTypes).To(HaveKeyWithValue(extensionsv1alpha1.InfrastructureResource, sets.New(type1)))
		})
	})
})
