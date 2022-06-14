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
	extcoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	coreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	"github.com/gardener/gardener/pkg/features"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/plugin/pkg/shoot/binding"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/utils/pointer"
)

var _ = Describe("validator", func() {
	Describe("#Validate", func() {
		var (
			admissionHandler       *Binding
			coreInformerFactory    coreinformers.SharedInformerFactory
			extCoreInformerFactory extcoreinformers.SharedInformerFactory
			cloudProfile           core.CloudProfile
			seed                   core.Seed
			// secretBinding          core.SecretBinding
			project    core.Project
			shoot      core.Shoot
			shootState gardencorev1alpha1.ShootState
			binding    core.Binding

			podsCIDR     = "100.96.0.0/11"
			servicesCIDR = "100.64.0.0/13"
			nodesCIDR    = "10.250.0.0/16"

			seedName       = "seed"
			targetSeedName = "seed1"
			namespaceName  = "garden-my-project"

			unmanagedDNSProvider = core.DNSUnmanaged
			baseDomain           = "example.com"

			validMachineImageName = "some-machineimage"
			volumeType            = "volume-type-1"

			seedBase = core.Seed{
				TypeMeta: metav1.TypeMeta{
					Kind: "Seed",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: targetSeedName,
				},
				Spec: core.SeedSpec{
					Backup: &core.SeedBackup{},
				},
			}

			shootBase = core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: namespaceName,
				},
				Spec: core.ShootSpec{
					CloudProfileName:  "profile",
					Region:            "europe",
					SecretBindingName: "my-secret",
					SeedName:          &seedName,
					DNS: &core.DNS{
						Domain: pointer.String(fmt.Sprintf("shoot.%s", baseDomain)),
						Providers: []core.DNSProvider{
							{
								Type: &unmanagedDNSProvider,
							},
						},
					},
					Kubernetes: core.Kubernetes{
						Version: "1.6.4",
					},
					Networking: core.Networking{
						Nodes:    &nodesCIDR,
						Pods:     &podsCIDR,
						Services: &servicesCIDR,
					},
					Provider: core.Provider{
						Type: "unknown",
						Workers: []core.Worker{
							{
								Name: "worker-name",
								Machine: core.Machine{
									Type: "machine-type-1",
									Image: &core.ShootMachineImage{
										Name: validMachineImageName,
									},
								},
								Minimum: 1,
								Maximum: 1,
								Volume: &core.Volume{
									VolumeSize: "40Gi",
									Type:       &volumeType,
								},
								Zones: []string{"europe-a"},
							},
						},
						InfrastructureConfig: &runtime.RawExtension{Raw: []byte(`{
"kind": "InfrastructureConfig",
"apiVersion": "some.random.config/v1beta1"}`)},
					},
				},
			}

			shootStateBase = gardencorev1alpha1.ShootState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootBase.Name,
					Namespace: shootBase.Namespace,
				},
			}

			bindingBase = core.Binding{
				TypeMeta: metav1.TypeMeta{
					Kind: "Binding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootBase.Name,
					Namespace: shootBase.Namespace,
				},
				Target: corev1.ObjectReference{
					APIVersion: seedBase.APIVersion,
					Kind:       seedBase.Kind,
					Name:       seedBase.Name,
				},
			}

			cleanup func()
		)

		BeforeEach(func() {
			seed = seedBase
			shoot = *shootBase.DeepCopy()
			shootState = *shootStateBase.DeepCopy()
			binding = bindingBase
			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })
			coreInformerFactory = coreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalCoreInformerFactory(coreInformerFactory)

			extCoreInformerFactory = extcoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetExternalCoreInformerFactory(extCoreInformerFactory)

			cleanup = test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SecretBindingProviderValidation, false)
		})

		AfterEach(func() {
			cleanup()
		})

		It("should reject creating binding if target Kind is empty", func() {
			bindingCopy := binding.DeepCopy()
			bindingCopy.Target.Kind = ""

			attrs := admission.NewAttributesRecord(bindingCopy, bindingCopy, core.Kind("Binding").WithVersion("version"), bindingCopy.Namespace, bindingCopy.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
		})

		It("should reject creating binding if target Kind is not Seed", func() {
			bindingCopy := binding.DeepCopy()
			bindingCopy.Target.Kind = "other"

			attrs := admission.NewAttributesRecord(bindingCopy, bindingCopy, core.Kind("Binding").WithVersion("version"), bindingCopy.Namespace, bindingCopy.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
		})

		It("should reject creating binding if target name is empty", func() {
			bindingCopy := binding.DeepCopy()
			bindingCopy.Target.Name = ""

			attrs := admission.NewAttributesRecord(bindingCopy, bindingCopy, core.Kind("Binding").WithVersion("version"), bindingCopy.Namespace, bindingCopy.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
		})

		Context("checks for shoots referencing a deleted seed", func() {
			var oldShoot *core.Shoot

			BeforeEach(func() {
				oldShoot = shootBase.DeepCopy()

				binding = *bindingBase.DeepCopy()
				seed = *seedBase.DeepCopy()
				now := metav1.Now()
				seed.DeletionTimestamp = &now

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shoot)).To(Succeed())
			})

			It("should reject updating a shoot on a seed which is marked for deletion", func() {
				defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

				attrs := admission.NewAttributesRecord(&binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("cannot schedule shoot '%s' on seed '%s' that is already marked for deletion", shoot.Name, seed.Name)))
			})

			It("should allow no-op updates", func() {
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should allow adding the deletion confirmation", func() {
				shoot.Annotations = make(map[string]string)
				shoot.Annotations[gutil.ConfirmationDeletion] = "true"

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("scheduling checks", func() {
			var (
				oldSeed *core.Seed
			)

			BeforeEach(func() {
				oldSeed = seed.DeepCopy()
				oldSeed.Name = seedName
				shootState.Spec.Gardener = append(shootState.Spec.Gardener, gardencorev1alpha1.GardenerResourceData{
					Labels: map[string]string{
						"name":       "kube-apiserver-etcd-encryption-key",
						"managed-by": "secrets-manager",
					},
				})

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(oldSeed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shoot)).To(Succeed())
				Expect(extCoreInformerFactory.Core().V1alpha1().ShootStates().Informer().GetStore().Add(&shootState)).To(Succeed())
			})

			Context("taints and tolerations", func() {
				It("update should pass because the Seed specified in shoot manifest does not have any taints", func() {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

					attrs := admission.NewAttributesRecord(&binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					err := admissionHandler.Validate(context.TODO(), attrs, nil)
					Expect(err).ToNot(HaveOccurred())
				})

				It("update should pass because the Seed has new non-tolerated taints that were added after the shoot was scheduled to it", func() {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

					seed.Spec.Taints = []core.SeedTaint{{Key: core.SeedTaintProtected}}
					shoot.Spec.SeedName = &targetSeedName
					shoot.Spec.Provider.Workers[0].Maximum++

					attrs := admission.NewAttributesRecord(&binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					err := admissionHandler.Validate(context.TODO(), attrs, nil)
					Expect(err).ToNot(HaveOccurred())
				})

				It("update should fail because the new Seed specified in shoot manifest has non-tolerated taints", func() {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

					seed.Spec.Taints = []core.SeedTaint{{Key: core.SeedTaintProtected}}

					attrs := admission.NewAttributesRecord(&binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					err := admissionHandler.Validate(context.TODO(), attrs, nil)
					Expect(err).To(HaveOccurred())
				})

				It("update should pass because the Seed stays the same, even if it has non-tolerated taints", func() {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

					shoot.Spec.SeedName = &targetSeedName
					seed.Spec.Taints = []core.SeedTaint{{Key: core.SeedTaintProtected}}

					attrs := admission.NewAttributesRecord(&binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					err := admissionHandler.Validate(context.TODO(), attrs, nil)
					Expect(err).ToNot(HaveOccurred())
				})

				It("update should pass because shoot tolerates all taints of the seed", func() {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

					seed.Spec.Taints = []core.SeedTaint{{Key: "foo"}}
					shoot.Spec.Tolerations = []core.Toleration{{Key: "foo", Value: pointer.String("bar")}}

					attrs := admission.NewAttributesRecord(&binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					err := admissionHandler.Validate(context.TODO(), attrs, nil)
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("seed capacity", func() {
				var (
					allocatableShoots resource.Quantity
				)

				BeforeEach(func() {
					allocatableShoots = *resource.NewQuantity(1, resource.DecimalSI)
				})

				It("should pass because seed allocatable capacity is not set", func() {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

					attrs := admission.NewAttributesRecord(&binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

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

					attrs := admission.NewAttributesRecord(&binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					err := admissionHandler.Validate(context.TODO(), attrs, nil)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should reject because seed allocatable capacity is exhausted", func() {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

					seed.Status.Allocatable = corev1.ResourceList{"shoots": allocatableShoots}

					otherShoot := shoot.DeepCopy()
					otherShoot.Name = "other-shoot-1"
					otherShoot.Spec.SeedName = &targetSeedName
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

					otherShoot = shoot.DeepCopy()
					otherShoot.Name = "other-shoot-2"
					otherShoot.Spec.SeedName = nil
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					err := admissionHandler.Validate(context.TODO(), attrs, nil)
					Expect(err).To(MatchError(ContainSubstring("already has the maximum number of shoots scheduled on it")))
				})

				It("should reject because seed allocatable capacity is over-exhausted", func() {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

					seed.Status.Allocatable = corev1.ResourceList{"shoots": allocatableShoots}

					otherShoot := shoot.DeepCopy()
					otherShoot.Name = "other-shoot-1"
					otherShoot.Spec.SeedName = &targetSeedName
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

					otherShoot = shoot.DeepCopy()
					otherShoot.Name = "other-shoot-2"
					otherShoot.Spec.SeedName = &targetSeedName
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					err := admissionHandler.Validate(context.TODO(), attrs, nil)
					Expect(err).To(MatchError(ContainSubstring("already has the maximum number of shoots scheduled on it")))
				})
			})
		})

		Context("backup configuration on seed", func() {
			It("should allow new Shoot creation when Seed doesn't have configuration for backup", func() {
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.SeedName = nil
				seed.Spec.Backup = nil

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("control plane migration", func() {
			var (
				oldSeed *core.Seed
			)
			BeforeEach(func() {
				oldSeed = seed.DeepCopy()
				oldSeed.Name = seedName

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(oldSeed)).To(Succeed())
				Expect(extCoreInformerFactory.Core().V1alpha1().ShootStates().Informer().GetStore().Add(&shootState)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shoot)).To(Succeed())
			})

			It("should fail to change Seed name, because Seed doesn't have configuration for backup", func() {
				defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

				seed.Spec.Backup = nil
				attrs := admission.NewAttributesRecord(&binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("backup is not configured for seed %q", targetSeedName))))
			})

			It("should fail to change Seed name, because old Seed doesn't have configuration for backup", func() {
				defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

				oldSeed.Spec.Backup = nil
				attrs := admission.NewAttributesRecord(&binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("backup is not configured for old seed %q", seedName))))
			})

			It("should fail to change Seed name, because cloud provider for new Seed is not equal to cloud provider for old Seed", func() {
				defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

				oldSeed.Spec.Provider.Type = "gcp"
				seed.Spec.Provider.Type = "aws"

				attrs := admission.NewAttributesRecord(&binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(BeForbiddenError())
			})

			It("should forbid changes to Seed name when etcd encryption key is missing", func() {
				defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

				attrs := admission.NewAttributesRecord(&binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(MatchError(ContainSubstring("because etcd encryption key not found in shoot state")))
			})

			It("should allow changes to Seed name when etcd encryption key is present and nothing else has changed", func() {
				defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

				shootState.Spec.Gardener = append(shootState.Spec.Gardener, gardencorev1alpha1.GardenerResourceData{
					Labels: map[string]string{
						"name":       "kube-apiserver-etcd-encryption-key",
						"managed-by": "secrets-manager",
					},
				})
				Expect(extCoreInformerFactory.Core().V1alpha1().ShootStates().Informer().GetStore().Update(&shootState)).To(Succeed())

				attrs := admission.NewAttributesRecord(&binding, nil, core.Kind("Binding").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("bindings").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
