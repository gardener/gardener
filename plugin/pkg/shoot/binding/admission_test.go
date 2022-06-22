// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package binding_test

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extcoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	coreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/plugin/pkg/shoot/binding"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/utils/pointer"
)

var _ = Describe("Shoot Binding Validator", func() {
	Describe("#Validate", func() {
		var (
			admissionHandler       *Binding
			coreInformerFactory    coreinformers.SharedInformerFactory
			extCoreInformerFactory extcoreinformers.SharedInformerFactory

			seed       *core.Seed
			newSeed    *core.Seed
			shoot      *core.Shoot
			binding    *core.Binding
			shootState *gardencorev1alpha1.ShootState

			seedName      = "seed"
			newSeedName   = "new-seed"
			namespaceName = "test-namespace"
		)

		BeforeEach(func() {
			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })

			coreInformerFactory = coreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalCoreInformerFactory(coreInformerFactory)

			extCoreInformerFactory = extcoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetExternalCoreInformerFactory(extCoreInformerFactory)

			seed = &core.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
				Spec: core.SeedSpec{
					Backup: &core.SeedBackup{},
				},
			}

			newSeed = &core.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: newSeedName,
				},
				Spec: core.SeedSpec{
					Backup: &core.SeedBackup{},
				},
			}

			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: namespaceName,
				},
				Spec: core.ShootSpec{
					SeedName: pointer.String(seedName),
				},
			}

			shootState = &gardencorev1alpha1.ShootState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shoot.Name,
					Namespace: shoot.Namespace,
				},
			}

			binding = &core.Binding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shoot.Name,
					Namespace: shoot.Namespace,
				},
				Target: corev1.ObjectReference{
					Kind:       "Seed",
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Name:       seed.Name,
				},
			}

			Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(seed)).To(Succeed())
			Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
			Expect(extCoreInformerFactory.Core().V1alpha1().ShootStates().Informer().GetStore().Add(shootState)).To(Succeed())
		})

		Context("#CreateBinding", func() {
			It("should allow creation of binding when shoot.spec.seedName is nil", func() {
				shoot.Spec.SeedName = nil
				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), binding.Namespace, binding.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject creation of binding when shoot.spec.seedName is not nil and the binding target has the same seedName", func() {
				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), binding.Namespace, binding.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("creation of binding rejected, shoot is already assigned to the seed in the binding"))
			})

			It("should allow creation of binding when shoot.spec.seedName is not nil and SeedChange feature gate is enabled", func() {
				defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()
				binding.Target.Name = newSeed.Name
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(newSeed)).To(Succeed())

				shootState.Spec.Gardener = append(shootState.Spec.Gardener, gardencorev1alpha1.GardenerResourceData{
					Labels: map[string]string{
						"name":       "kube-apiserver-etcd-encryption-key",
						"managed-by": "secrets-manager",
					},
				})

				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), binding.Namespace, binding.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject creation of binding when shoot.spec.seedName is not nil and SeedChange feature gate is disabled", func() {
				binding.Target.Name = newSeed.Name

				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), binding.Namespace, binding.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("target.name: Invalid value: %q: field is immutable", newSeedName))
			})

			It("should reject creation of binding if target Kind is empty", func() {
				binding.Target.Kind = ""

				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), binding.Namespace, binding.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("target.kind: Unsupported value"))
			})

			It("should reject creation of binding if target Kind is not Seed", func() {
				binding.Target.Kind = "other"

				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), binding.Namespace, binding.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("target.kind: Unsupported value"))
			})

			It("should reject creation of binding if target name is empty", func() {
				binding.Target.Name = ""

				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), binding.Namespace, binding.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("target.name: Required value: target name for Binding object cannot be an empty string"))
			})

			It("should reject creation of binding if target seed does not exist", func() {
				defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()
				binding.Target.Name = newSeedName + " other"

				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), binding.Namespace, binding.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Internal error occurred: could not find referenced seed"))
			})
		})

		Context("shootIsBeingScheduled", func() {
			It("should reject creation of binding if target seed is marked for deletion", func() {
				shoot.Spec.SeedName = nil
				now := metav1.Now()
				seed.DeletionTimestamp = &now

				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), binding.Namespace, binding.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot schedule shoot 'shoot' on seed 'seed' that is already marked for deletion"))
			})
		})

		Context("shootIsBeingRescheduled or Control-Plane migration", func() {
			BeforeEach(func() {
				binding.Target.Name = newSeed.Name
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(newSeed)).To(Succeed())

				shootState.Spec.Gardener = append(shootState.Spec.Gardener, gardencorev1alpha1.GardenerResourceData{
					Labels: map[string]string{
						"name":       "kube-apiserver-etcd-encryption-key",
						"managed-by": "secrets-manager",
					},
				})
			})

			It("should reject creation of binding if target seed is marked for deletion", func() {
				defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()
				now := metav1.Now()
				newSeed.DeletionTimestamp = &now

				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), binding.Namespace, binding.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot schedule shoot '%s' on seed '%s' that is already marked for deletion", shoot.Name, newSeedName))
			})

			It("should reject creation of binding, because target Seed doesn't have configuration for backup", func() {
				defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

				newSeed.Spec.Backup = nil

				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("backup is not configured for seed %q", newSeedName)))
			})

			It("should reject creation of binding, because old Seed doesn't have configuration for backup", func() {
				defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

				seed.Spec.Backup = nil

				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("backup is not configured for old seed %q", seedName)))
			})

			It("should reject creation of binding, ecause cloud provider for new Seed is not equal to cloud provider for old Seed", func() {
				defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

				seed.Spec.Provider.Type = "gcp"
				newSeed.Spec.Provider.Type = "aws"

				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("cannot change seed because cloud provider for new seed (%s) is not equal to cloud provider for old seed (%s)", newSeed.Spec.Provider.Type, seed.Spec.Provider.Type))
			})

			It("should forbid changes to Seed name when etcd encryption key is missing", func() {
				defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

				shootState.Spec.Gardener = nil

				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("cannot change seed because etcd encryption key not found in shoot state"))
			})
		})

		Context("taints and tolerations", func() {
			BeforeEach(func() {
				shoot.Spec.SeedName = nil
			})

			It("creation of binding should succeed because the Seed specified in shoot manifest does not have any taints", func() {
				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})

			It("creation of binding should fail because the seed specified in the binding has non-tolerated taints", func() {
				defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

				seed.Spec.Taints = []core.SeedTaint{{Key: core.SeedTaintProtected}}

				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("forbidden to use a seed whose taints are not tolerated by the shoot"))
			})

			It("creation of binding should fail because the new Seed specified in the binding has non-tolerated taints", func() {
				defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

				binding.Target.Name = newSeedName
				newSeed.Spec.Taints = []core.SeedTaint{{Key: core.SeedTaintProtected}}

				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(newSeed)).To(Succeed())

				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("forbidden to use a seed whose taints are not tolerated by the shoot"))
			})

			It("creation of binding should pass because shoot tolerates all taints of the seed", func() {
				seed.Spec.Taints = []core.SeedTaint{{Key: "foo"}}
				shoot.Spec.Tolerations = []core.Toleration{{Key: "foo", Value: pointer.String("bar")}}

				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("seed capacity", func() {
			var (
				allocatableShoots resource.Quantity
			)

			BeforeEach(func() {
				shoot.Spec.SeedName = nil
				allocatableShoots = *resource.NewQuantity(1, resource.DecimalSI)
			})

			It("should pass because seed allocatable capacity is not set", func() {
				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should pass because seed allocatable capacity is not exhausted", func() {
				defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

				seed.Status.Allocatable = corev1.ResourceList{"shoots": allocatableShoots}

				otherShoot := shoot.DeepCopy()
				otherShoot.Name = "other-shoot-1"
				otherShoot.Spec.SeedName = pointer.String("other-seed")
				Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

				otherShoot = shoot.DeepCopy()
				otherShoot.Name = "other-shoot-2"
				otherShoot.Spec.SeedName = nil
				Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because seed allocatable capacity is exhausted", func() {
				defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

				seed.Status.Allocatable = corev1.ResourceList{"shoots": allocatableShoots}

				otherShoot := shoot.DeepCopy()
				otherShoot.Name = "other-shoot-1"
				otherShoot.Spec.SeedName = pointer.String(seedName)
				Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

				otherShoot = shoot.DeepCopy()
				otherShoot.Name = "other-shoot-2"
				otherShoot.Spec.SeedName = nil
				Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(MatchError(ContainSubstring("already has the maximum number of shoots scheduled on it")))
			})

			It("should reject because seed allocatable capacity is over-exhausted", func() {
				defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

				seed.Status.Allocatable = corev1.ResourceList{"shoots": allocatableShoots}

				otherShoot := shoot.DeepCopy()
				otherShoot.Name = "other-shoot-1"
				otherShoot.Spec.SeedName = pointer.String(seedName)
				Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

				otherShoot = shoot.DeepCopy()
				otherShoot.Name = "other-shoot-2"
				otherShoot.Spec.SeedName = pointer.String(seedName)
				Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

				attrs := admission.NewAttributesRecord(binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(MatchError(ContainSubstring("already has the maximum number of shoots scheduled on it")))
			})
		})
	})
})
