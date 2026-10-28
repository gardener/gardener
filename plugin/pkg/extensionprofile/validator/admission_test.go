// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/plugin/pkg/extensionprofile/validator"
)

var _ = Describe("ExtensionProfile Admission", func() {
	Describe("#Validate", func() {
		var (
			ctx                 = context.Background()
			admissionHandler    *ExtensionProfileValidator
			coreInformerFactory gardencoreinformers.SharedInformerFactory

			extensionProfile *gardencorev1beta1.ExtensionProfile
			shoot            *gardencore.Shoot

			shootKind     = gardencorev1beta1.Kind("Shoot").WithVersion("version")
			shootResource = gardencorev1beta1.Resource("shoots").WithVersion("version")
		)

		pastTime := metav1.NewTime(time.Now().Add(-time.Hour))

		BeforeEach(func() {
			var err error
			admissionHandler, err = New()
			Expect(err).NotTo(HaveOccurred())
			admissionHandler.AssignReadyFunc(func() bool { return true })

			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetCoreInformerFactory(coreInformerFactory)

			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.ExtensionComponentVersions, true))

			extensionProfile = &gardencorev1beta1.ExtensionProfile{
				ObjectMeta: metav1.ObjectMeta{Name: "shoot-traefik"},
				Spec: gardencorev1beta1.ExtensionProfileSpec{
					Components: []gardencorev1beta1.ExtensionProfileComponent{
						{
							Name: "traefik",
							Versions: []gardencorev1beta1.ExtensionVersion{
								{
									Version:   "3.1.4",
									Lifecycle: []gardencorev1beta1.LifecycleStage{{Classification: gardencorev1beta1.ClassificationSupported, StartTime: &pastTime}},
								},
								{
									Version:   "3.0.0",
									Lifecycle: []gardencorev1beta1.LifecycleStage{{Classification: gardencorev1beta1.ClassificationExpired, StartTime: &pastTime}},
								},
							},
						},
					},
				},
			}

			shoot = &gardencore.Shoot{
				ObjectMeta: metav1.ObjectMeta{Name: "shoot", Namespace: "garden-dev"},
				Spec: gardencore.ShootSpec{
					Extensions: []gardencore.Extension{
						{
							Type: "shoot-traefik",
							Components: []gardencore.ShootExtensionComponent{
								{Name: "traefik", Version: new("3.1.4")},
							},
						},
					},
				},
			}
		})

		newShootAttrs := func(operation admission.Operation, oldShoot *gardencore.Shoot) admission.Attributes {
			return admission.NewAttributesRecord(shoot, oldShoot, shootKind, shoot.Namespace, shoot.Name, shootResource, "", operation, &metav1.CreateOptions{}, false, nil)
		}

		It("should allow pinning a supported version", func() {
			Expect(coreInformerFactory.Core().V1beta1().ExtensionProfiles().Informer().GetStore().Add(extensionProfile)).To(Succeed())

			Expect(admissionHandler.Validate(ctx, newShootAttrs(admission.Create, nil), nil)).To(Succeed())
		})

		It("should reject pinning an expired version", func() {
			shoot.Spec.Extensions[0].Components[0].Version = new("3.0.0")
			Expect(coreInformerFactory.Core().V1beta1().ExtensionProfiles().Informer().GetStore().Add(extensionProfile)).To(Succeed())

			Expect(admissionHandler.Validate(ctx, newShootAttrs(admission.Create, nil), nil)).To(MatchError(ContainSubstring("classified \"expired\"")))
		})

		It("should reject pinning a version not offered by the profile", func() {
			shoot.Spec.Extensions[0].Components[0].Version = new("9.9.9")
			Expect(coreInformerFactory.Core().V1beta1().ExtensionProfiles().Informer().GetStore().Add(extensionProfile)).To(Succeed())

			Expect(admissionHandler.Validate(ctx, newShootAttrs(admission.Create, nil), nil)).To(MatchError(ContainSubstring("is not offered")))
		})

		It("should reject a pin when no matching ExtensionProfile exists", func() {
			Expect(admissionHandler.Validate(ctx, newShootAttrs(admission.Create, nil), nil)).To(MatchError(ContainSubstring("no ExtensionProfile found")))
		})

		It("should be a no-op when the feature gate is disabled", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.ExtensionComponentVersions, false))
			shoot.Spec.Extensions[0].Components[0].Version = new("9.9.9")

			Expect(admissionHandler.Validate(ctx, newShootAttrs(admission.Create, nil), nil)).To(Succeed())
		})

		It("should not re-check an unchanged expired pin on update", func() {
			shoot.Spec.Extensions[0].Components[0].Version = new("3.0.0")
			oldShoot := shoot.DeepCopy()
			Expect(coreInformerFactory.Core().V1beta1().ExtensionProfiles().Informer().GetStore().Add(extensionProfile)).To(Succeed())

			Expect(admissionHandler.Validate(ctx, newShootAttrs(admission.Update, oldShoot), nil)).To(Succeed())
		})
	})
})
