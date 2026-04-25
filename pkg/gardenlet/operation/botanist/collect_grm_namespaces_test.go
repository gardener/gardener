// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("collectAdditionalGRMTargetNamespaces", func() {
	var (
		ctx        context.Context
		b          *Botanist
		fakeClient client.Client
		s          *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		b = &Botanist{Operation: &operation.Operation{}}
		b.Shoot = &shootpkg.Shoot{}

		s = runtime.NewScheme()
		Expect(gardencorev1beta1.AddToScheme(s)).To(Succeed())
	})

	It("should return empty when no ControllerRegistrations exist", func() {
		fakeClient = fake.NewClientBuilder().WithScheme(s).Build()
		b.GardenClient = fakeClient

		Expect(b.collectAdditionalGRMTargetNamespaces(ctx)).To(Succeed())
		Expect(b.Shoot.AdditionalGRMTargetNamespaces).To(BeEmpty())
	})

	It("should collect namespaces from Extension resources only", func() {
		fakeClient = fake.NewClientBuilder().
			WithScheme(s).
			WithObjects(
				&gardencorev1beta1.ControllerRegistration{
					ObjectMeta: metav1.ObjectMeta{Name: "ext-with-namespaces"},
					Spec: gardencorev1beta1.ControllerRegistrationSpec{
						Resources: []gardencorev1beta1.ControllerResource{
							{
								Kind:                            extensionsv1alpha1.ExtensionResource,
								Type:                            "my-extension",
								AdditionalShootTargetNamespaces: []string{"custom-namespace", "another-ns"},
							},
						},
					},
				},
				&gardencorev1beta1.ControllerRegistration{
					ObjectMeta: metav1.ObjectMeta{Name: "infra-with-namespaces"},
					Spec: gardencorev1beta1.ControllerRegistrationSpec{
						Resources: []gardencorev1beta1.ControllerResource{
							{
								Kind:                            extensionsv1alpha1.InfrastructureResource,
								Type:                            "my-infra",
								AdditionalShootTargetNamespaces: []string{"should-be-ignored"},
							},
						},
					},
				},
			).
			Build()
		b.GardenClient = fakeClient

		Expect(b.collectAdditionalGRMTargetNamespaces(ctx)).To(Succeed())
		Expect(b.Shoot.AdditionalGRMTargetNamespaces).To(Equal([]string{"another-ns", "custom-namespace"}))
	})

	It("should merge and deduplicate namespaces from multiple ControllerRegistrations", func() {
		fakeClient = fake.NewClientBuilder().
			WithScheme(s).
			WithObjects(
				&gardencorev1beta1.ControllerRegistration{
					ObjectMeta: metav1.ObjectMeta{Name: "ext-a"},
					Spec: gardencorev1beta1.ControllerRegistrationSpec{
						Resources: []gardencorev1beta1.ControllerResource{
							{
								Kind:                            extensionsv1alpha1.ExtensionResource,
								Type:                            "ext-a",
								AdditionalShootTargetNamespaces: []string{"shared-ns", "ns-a"},
							},
						},
					},
				},
				&gardencorev1beta1.ControllerRegistration{
					ObjectMeta: metav1.ObjectMeta{Name: "ext-b"},
					Spec: gardencorev1beta1.ControllerRegistrationSpec{
						Resources: []gardencorev1beta1.ControllerResource{
							{
								Kind:                            extensionsv1alpha1.ExtensionResource,
								Type:                            "ext-b",
								AdditionalShootTargetNamespaces: []string{"shared-ns", "ns-b"},
							},
						},
					},
				},
			).
			Build()
		b.GardenClient = fakeClient

		Expect(b.collectAdditionalGRMTargetNamespaces(ctx)).To(Succeed())
		Expect(b.Shoot.AdditionalGRMTargetNamespaces).To(Equal([]string{"ns-a", "ns-b", "shared-ns"}))
	})

	It("should skip resources without AdditionalShootTargetNamespaces", func() {
		fakeClient = fake.NewClientBuilder().
			WithScheme(s).
			WithObjects(
				&gardencorev1beta1.ControllerRegistration{
					ObjectMeta: metav1.ObjectMeta{Name: "ext-no-ns"},
					Spec: gardencorev1beta1.ControllerRegistrationSpec{
						Resources: []gardencorev1beta1.ControllerResource{
							{
								Kind: extensionsv1alpha1.ExtensionResource,
								Type: "plain-ext",
							},
						},
					},
				},
			).
			Build()
		b.GardenClient = fakeClient

		Expect(b.collectAdditionalGRMTargetNamespaces(ctx)).To(Succeed())
		Expect(b.Shoot.AdditionalGRMTargetNamespaces).To(BeEmpty())
	})
})
