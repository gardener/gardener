// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/provider-local/admission/mutator"
	"github.com/gardener/gardener/pkg/provider-local/apis/local/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
)

var _ = Describe("Shoot mutator", func() {
	Describe("#Mutate", func() {
		const namespace = "garden-dev"

		var (
			shootMutator extensionswebhook.Mutator
			shoot        *gardencorev1beta1.Shoot
			ctx          = context.TODO()
			now          = metav1.Now()
			ctrl         *gomock.Controller
			mgr          *mockmanager.MockManager
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())

			scheme := runtime.NewScheme()
			Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())

			mgr = mockmanager.NewMockManager(ctrl)
			mgr.EXPECT().GetScheme().Return(scheme)
			shootMutator = mutator.NewShootMutator(mgr)

			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: namespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.28.2",
					},
				},
			}
		})

		Context("Mutate shoot", func() {
			It("should set test value", func() {
				shootExpected := shoot.DeepCopy()
				shootExpected.Spec.Provider.ControlPlaneConfig = &runtime.RawExtension{
					Object: &v1alpha1.ControlPlaneConfig{
						TypeMeta: metav1.TypeMeta{
							APIVersion: v1alpha1.SchemeGroupVersion.String(),
							Kind:       "ControlPlaneConfig",
						},
						TestValue: ptr.To("filled-by-admission-webhook"),
					},
				}
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot).To(DeepEqual(shootExpected))
			})

			It("should return without mutation when shoot is in deletion phase", func() {
				shoot.DeletionTimestamp = &now
				shootExpected := shoot.DeepCopy()
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot).To(DeepEqual(shootExpected))
			})

			It("should return without mutation when shoot specs have not changed", func() {
				shootWithAnnotations := shoot.DeepCopy()
				shootWithAnnotations.Annotations = map[string]string{"foo": "bar"}
				shootExpected := shootWithAnnotations.DeepCopy()

				err := shootMutator.Mutate(ctx, shootWithAnnotations, shoot)
				Expect(err).NotTo(HaveOccurred())
				Expect(shootWithAnnotations).To(DeepEqual(shootExpected))
			})
		})
	})
})
