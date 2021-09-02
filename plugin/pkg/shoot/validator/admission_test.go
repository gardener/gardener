// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validator_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	coreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/plugin/pkg/shoot/validator"
)

var _ = Describe("validator", func() {
	Describe("#Admit", func() {
		var (
			admissionHandler    *ValidateShoot
			coreInformerFactory coreinformers.SharedInformerFactory
			cloudProfile        core.CloudProfile
			seed                core.Seed
			project             core.Project
			shoot               core.Shoot

			podsCIDR     = "100.96.0.0/11"
			servicesCIDR = "100.64.0.0/13"
			nodesCIDR    = "10.250.0.0/16"

			falseVar = false

			seedName      = "seed"
			namespaceName = "garden-my-project"
			projectName   = "my-project"

			unmanagedDNSProvider = core.DNSUnmanaged
			baseDomain           = "example.com"

			validMachineImageName     = "some-machineimage"
			validMachineImageVersions = []core.MachineImageVersion{
				{
					ExpirableVersion: core.ExpirableVersion{
						Version: "0.0.1",
					},
				},
			}
			volumeType        = "volume-type-1"
			volumeType2       = "volume-type-2"
			minVolSize        = resource.MustParse("100Gi")
			minVolSizeMachine = resource.MustParse("50Gi")

			seedPodsCIDR     = "10.241.128.0/17"
			seedServicesCIDR = "10.241.0.0/17"
			seedNodesCIDR    = "10.240.0.0/16"
			seedSecret       = corev1.Secret{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      seedName,
					Namespace: "garden",
				},
				Data: map[string][]byte{
					kubernetes.KubeConfig: []byte(""),
				},
				Type: corev1.SecretTypeOpaque,
			}

			projectBase = core.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
				Spec: core.ProjectSpec{
					Namespace: &namespaceName,
				},
			}
			cloudProfileBase = core.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "profile",
				},
				Spec: core.CloudProfileSpec{
					Type: "unknown",
					Kubernetes: core.KubernetesSettings{
						Versions: []core.ExpirableVersion{{Version: "1.6.4"}},
					},
					MachineImages: []core.MachineImage{
						{
							Name:     validMachineImageName,
							Versions: validMachineImageVersions,
						},
					},
					MachineTypes: []core.MachineType{
						{
							Name:   "machine-type-1",
							CPU:    resource.MustParse("2"),
							GPU:    resource.MustParse("0"),
							Memory: resource.MustParse("100Gi"),
						},
						{
							Name:   "machine-type-old",
							CPU:    resource.MustParse("2"),
							GPU:    resource.MustParse("0"),
							Memory: resource.MustParse("100Gi"),
							Usable: &falseVar,
						},
						{
							Name:   "machine-type-2",
							CPU:    resource.MustParse("2"),
							GPU:    resource.MustParse("0"),
							Memory: resource.MustParse("100Gi"),
							Storage: &core.MachineTypeStorage{
								Type:    volumeType,
								MinSize: &minVolSizeMachine,
							},
						},
					},
					VolumeTypes: []core.VolumeType{
						{
							Name:  volumeType,
							Class: "super-premium",
						},
						{
							Name:    volumeType2,
							Class:   "super-premium",
							MinSize: &minVolSize,
						},
					},
					Regions: []core.Region{
						{
							Name:  "europe",
							Zones: []core.AvailabilityZone{{Name: "europe-a"}},
						},
						{
							Name:  "asia",
							Zones: []core.AvailabilityZone{{Name: "asia-a"}},
						},
					},
				},
			}
			seedBase = core.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
				Spec: core.SeedSpec{
					Backup: &core.SeedBackup{},
					Networks: core.SeedNetworks{
						Pods:     seedPodsCIDR,
						Services: seedServicesCIDR,
						Nodes:    &seedNodesCIDR,
					},
					SecretRef: &corev1.SecretReference{
						Name:      seedSecret.Name,
						Namespace: seedSecret.Namespace,
					},
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
					},
				},
			}
		)

		BeforeEach(func() {
			project = projectBase
			cloudProfile = *cloudProfileBase.DeepCopy()
			seed = seedBase
			shoot = *shootBase.DeepCopy()

			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })
			coreInformerFactory = coreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalCoreInformerFactory(coreInformerFactory)
		})

		Context("name/project length checks", func() {
			It("should reject create operations on Shoot resources in projects which shall be deleted", func() {
				deletionTimestamp := metav1.NewTime(time.Now())
				project.ObjectMeta.DeletionTimestamp = &deletionTimestamp

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("already marked for deletion"))
			})

			It("should reject Shoot resources with not fulfilling the length constraints", func() {
				tooLongName := "too-long-namespace"
				project.ObjectMeta = metav1.ObjectMeta{
					Name: tooLongName,
				}
				shoot.ObjectMeta = metav1.ObjectMeta{
					Name:      "too-long-name",
					Namespace: namespaceName,
				}

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(BeBadRequestError())
				Expect(err.Error()).To(ContainSubstring("name must not exceed"))
			})

			It("should not test length constraints for operations other than CREATE", func() {
				shortName := "short"
				projectName := "too-long-long-long-label"
				project.ObjectMeta = metav1.ObjectMeta{
					Name: projectName,
				}
				shoot.ObjectMeta = metav1.ObjectMeta{
					Name:      shortName,
					Namespace: shortName,
				}

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).NotTo(ContainSubstring("name must not exceed"))

				attrs = admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
				err = admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).NotTo(ContainSubstring("name must not exceed"))
			})
		})

		Context("shoot with generate name", func() {
			BeforeEach(func() {
				shoot.ObjectMeta = metav1.ObjectMeta{
					GenerateName: "demo-",
					Namespace:    namespaceName,
				}
			})

			It("should admit Shoot resources", func() {
				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject Shoot resources with not fulfilling the length constraints", func() {
				tooLongName := "too-long-namespace"
				project.ObjectMeta = metav1.ObjectMeta{
					Name: tooLongName,
				}
				shoot.ObjectMeta = metav1.ObjectMeta{
					GenerateName: "too-long-name",
					Namespace:    namespaceName,
				}

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(BeBadRequestError())
				Expect(err.Error()).To(ContainSubstring("name must not exceed"))
			})
		})

		Context("finalizer removal checks", func() {
			var (
				oldShoot *core.Shoot
			)

			BeforeEach(func() {
				shoot = *shootBase.DeepCopy()

				shoot.Status.TechnicalID = "some-id"
				shoot.Status.LastOperation = &core.LastOperation{
					Type:     core.LastOperationTypeReconcile,
					State:    core.LastOperationStateSucceeded,
					Progress: 100,
				}

				// set old shoot for update and add gardener finalizer to it
				oldShoot = shoot.DeepCopy()
				finalizers := sets.NewString(oldShoot.GetFinalizers()...)
				finalizers.Insert(core.GardenerName)
				oldShoot.SetFinalizers(finalizers.UnsortedList())
			})

			It("should reject removing the gardener finalizer if the shoot has not yet been deleted successfully", func() {
				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("shoot deletion has not completed successfully yet"))
			})

			It("should admit removing the gardener finalizer if the shoot deletion succeeded ", func() {
				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

				shoot.Status.LastOperation = &core.LastOperation{
					Type:     core.LastOperationTypeDelete,
					State:    core.LastOperationStateSucceeded,
					Progress: 100,
				}

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("hibernation checks", func() {
			var (
				oldShoot *core.Shoot
			)

			BeforeEach(func() {
				shoot = *shootBase.DeepCopy()
				oldShoot = shoot.DeepCopy()
				oldShoot.Spec.Hibernation = &core.Hibernation{Enabled: pointer.Bool(false)}

				shoot.Spec.Hibernation = &core.Hibernation{Enabled: pointer.Bool(true)}
			})

			DescribeTable("should allow/deny hibernating the Shoot according to HibernationPossible constraint",
				func(constraints []core.Condition, match types.GomegaMatcher) {
					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

					shoot.Status.Constraints = constraints

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)
					Expect(err).To(match)
				},
				Entry("should allow if set to True", []core.Condition{
					{
						Type:   core.ShootHibernationPossible,
						Status: core.ConditionTrue,
					},
				}, Not(HaveOccurred())),
				Entry("should deny if set to False", []core.Condition{
					{
						Type:    core.ShootHibernationPossible,
						Status:  core.ConditionFalse,
						Message: "foo",
					},
				}, And(HaveOccurred(), MatchError(ContainSubstring("foo")))),
				Entry("should deny if set to Unknown", []core.Condition{
					{
						Type:    core.ShootHibernationPossible,
						Status:  core.ConditionUnknown,
						Message: "foo",
					},
				}, And(HaveOccurred(), MatchError(ContainSubstring("foo")))),
				Entry("should allow if unset", []core.Condition{}, Not(HaveOccurred())),
			)
		})

		Context("shoot maintenance checks", func() {
			var (
				oldShoot           *core.Shoot
				confineEnabled     = true
				specUpdate         = true
				operationFaild     = &core.LastOperation{State: core.LastOperationStateFailed}
				operationSucceeded = &core.LastOperation{State: core.LastOperationStateSucceeded}
			)
			BeforeEach(func() {
				shoot = *shootBase.DeepCopy()
				shoot.Spec.Maintenance = &core.Maintenance{}
				oldShoot = shoot.DeepCopy()

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
			})

			DescribeTable("confine spec roll-out checks",
				func(specChange, oldConfine, confine bool, oldOperation, operation *core.LastOperation, matcher types.GomegaMatcher) {
					oldShoot.Spec.Maintenance.ConfineSpecUpdateRollout = pointer.Bool(oldConfine)
					oldShoot.Status.LastOperation = oldOperation
					shoot.Spec.Maintenance.ConfineSpecUpdateRollout = pointer.Bool(confine)
					shoot.Status.LastOperation = operation
					if specChange {
						shoot.Spec.Kubernetes.AllowPrivilegedContainers = pointer.Bool(
							oldShoot.Spec.Kubernetes.AllowPrivilegedContainers == nil ||
								!(*oldShoot.Spec.Kubernetes.AllowPrivilegedContainers))
					}

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					Expect(admissionHandler.Admit(context.TODO(), attrs, nil)).To(Succeed())

					Expect(shoot.Annotations).To(matcher)
				},
				Entry(
					"should add annotation for failed shoot",
					specUpdate, confineEnabled, confineEnabled, operationFaild, operationFaild,
					HaveKeyWithValue(v1beta1constants.FailedShootNeedsRetryOperation, "true"),
				),
				Entry(
					"should not add annotation for failed shoot because of missing spec change",
					!specUpdate, confineEnabled, confineEnabled, operationFaild, operationFaild,
					Not(HaveKey(v1beta1constants.FailedShootNeedsRetryOperation)),
				),
				Entry(
					"should not add annotation for succeeded shoot",
					specUpdate, confineEnabled, confineEnabled, operationFaild, operationSucceeded,
					Not(HaveKey(v1beta1constants.FailedShootNeedsRetryOperation)),
				),
				Entry(
					"should not add annotation for shoot w/o confine spec roll-out enabled",
					specUpdate, confineEnabled, !confineEnabled, operationFaild, operationFaild,
					Not(HaveKey(v1beta1constants.FailedShootNeedsRetryOperation)),
				),
				Entry(
					"should not add annotation for shoot w/o last operation",
					specUpdate, confineEnabled, confineEnabled, nil, nil,
					Not(HaveKey(v1beta1constants.FailedShootNeedsRetryOperation)),
				),
			)
		})

		Context("checks for shoots referencing a deleted seed", func() {
			var oldShoot *core.Shoot

			BeforeEach(func() {
				oldShoot = shootBase.DeepCopy()

				seed = *seedBase.DeepCopy()
				now := metav1.Now()
				seed.DeletionTimestamp = &now

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
			})

			It("should reject creating a shoot on a seed which is marked for deletion", func() {
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("cannot schedule shoot '%s' on seed '%s' that is already marked for deletion", shoot.Name, seed.Name)))
			})

			It("should allow no-op updates", func() {
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should allow modifying the finalizers array", func() {
				oldShoot.Finalizers = []string{core.GardenerName}
				shoot.Finalizers = []string{}

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should allow adding the deletion confirmation", func() {
				shoot.Annotations = make(map[string]string)
				shoot.Annotations[gutil.ConfirmationDeletion] = "true"

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should reject modifying the shoot spec when seed is marked for deletion", func() {
				shoot.Spec.Region = "other-region"

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("cannot update spec of shoot '%s' on seed '%s' already marked for deletion", shoot.Name, seed.Name)))
			})

			It("should reject modifying other annotations than the deletion confirmation when seed is marked for deletion", func() {
				shoot.Annotations = make(map[string]string)
				shoot.Annotations["foo"] = "bar"

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("cannot update annotations of shoot '%s' on seed '%s' already marked for deletion", shoot.Name, seed.Name)))
			})
		})

		Context("reference checks", func() {
			It("should reject because the referenced cloud profile was not found", func() {
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(BeBadRequestError())
			})

			It("should reject because the referenced seed was not found", func() {
				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(BeBadRequestError())
			})

			It("should reject because the referenced project was not found", func() {
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(BeBadRequestError())
			})

			It("should reject because the cloud provider in shoot and profile differ", func() {
				cloudProfile.Spec.Type = "gcp"
				shoot.Spec.Provider.Type = "aws"

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(BeForbiddenError())
			})

			It("should pass because no seed has to be specified (however can be). The scheduler sets the seed instead.", func() {
				shoot.Spec.SeedName = nil
				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("tests infrastructure deploy task", func() {
			var (
				oldShoot *core.Shoot
			)

			BeforeEach(func() {
				oldShoot = shootBase.DeepCopy()
				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
			})

			It("should add deploy infrastructure task because shoot is being created", func() {
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, v1beta1constants.ShootTaskDeployInfrastructure)).To(BeTrue())
			})

			It("should add deploy infrastructure task because shoot is waking up from hibernation", func() {
				oldShoot.Spec.Hibernation = &core.Hibernation{
					Enabled: pointer.Bool(true),
				}
				shoot.Spec.Hibernation = &core.Hibernation{
					Enabled: pointer.Bool(false),
				}

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, v1beta1constants.ShootTaskDeployInfrastructure)).To(BeTrue())
			})

			It("should add deploy infrastructure task because spec has changed", func() {
				shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{
					Raw: []byte("infrastructure"),
				}

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, v1beta1constants.ShootTaskDeployInfrastructure)).To(BeTrue())
			})

			It("should add deploy infrastructure task because shoot operation annotation to rotate ssh keypair was set", func() {
				shoot.Annotations = make(map[string]string)
				shoot.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.ShootOperationRotateSSHKeypair

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, v1beta1constants.ShootTaskDeployInfrastructure)).To(BeTrue())
			})

			It("should not add deploy infrastructure task because spec has not changed", func() {
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, v1beta1constants.ShootTaskDeployInfrastructure)).ToNot(BeTrue())
			})
		})

		Context("tests for region/zone updates", func() {
			var (
				oldShoot *core.Shoot
			)

			BeforeEach(func() {
				oldShoot = shootBase.DeepCopy()
				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
			})

			It("should pass update for non existing region in cloud profile because shoot region is unchanged", func() {
				cloudProfile.Spec.Regions = nil

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
			})

			It("should reject update because shoot changed to unknown region", func() {
				shoot.Spec.Region = "does-not-exist"

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("Unsupported value: \"does-not-exist\": supported values: \"europe\", \"asia\""))
			})

			It("should pass update for non existing zone in cloud profile because shoot worker zone is unchanged", func() {
				cloudProfile.Spec.Regions[0].Zones = []core.AvailabilityZone{{Name: "not-available"}}

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
			})

			It("should reject update because shoot changed to region with unavailable zone", func() {
				shoot.Spec.Region = "asia"

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("Unsupported value: \"europe-a\": supported values: \"asia-a\""))
			})

			It("should reject update because shoot and cloud profile changed zones", func() {
				cloudProfile.Spec.Regions[0].Zones = []core.AvailabilityZone{{Name: "zone-1"}, {Name: "zone-2"}}
				shoot.Spec.Provider.Workers[0].Zones = append(shoot.Spec.Provider.Workers[0].Zones, "zone-1")

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("Unsupported value: \"europe-a\": supported values: \"zone-1\", \"zone-2\""))
			})

			It("should reject due to an invalid zone", func() {
				shoot.Spec.Provider.Workers[0].Zones = []string{"invalid-zone"}

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(BeForbiddenError())
			})

			It("should reject due to an invalid zone update", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Provider.Workers[0].Zones = append(shoot.Spec.Provider.Workers[0].Zones, oldShoot.Spec.Provider.Workers[0].Zones...)
				shoot.Spec.Provider.Workers[0].Zones = append(shoot.Spec.Provider.Workers[0].Zones, "invalid-zone")

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(BeForbiddenError())
			})

			It("should allow update when zone has removed from CloudProfile", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Provider.Workers[0].Zones = []string{}
				cloudProfile.Spec.Regions = cloudProfile.Spec.Regions[1:]

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("tests for unknown provider", func() {
			Context("scheduling checks", func() {
				var (
					oldShoot *core.Shoot
				)

				BeforeEach(func() {
					oldShoot = shoot.DeepCopy()
					oldShoot.Spec.SeedName = nil
				})

				Context("taints and tolerations", func() {
					It("create should pass because the Seed specified in shoot manifest does not have any taints", func() {
						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)
						Expect(err).ToNot(HaveOccurred())
					})

					It("update should pass because the Seed specified in shoot manifest does not have any taints", func() {
						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)
						Expect(err).ToNot(HaveOccurred())
					})

					It("update should pass because the Seed has new non-tolerated taints that were added after the shoot was scheduled to it", func() {
						seed.Spec.Taints = []core.SeedTaint{{Key: core.SeedTaintProtected}}
						oldShoot.Spec.SeedName = shoot.Spec.SeedName
						shoot.Spec.Provider.Workers[0].Maximum++

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)
						Expect(err).ToNot(HaveOccurred())
					})

					It("create should fail because the Seed specified in shoot manifest has non-tolerated taints", func() {
						seed.Spec.Taints = []core.SeedTaint{{Key: core.SeedTaintProtected}}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)
						Expect(err).To(BeForbiddenError())
					})

					It("update should fail because the new Seed specified in shoot manifest has non-tolerated taints", func() {
						seed.Spec.Taints = []core.SeedTaint{{Key: core.SeedTaintProtected}}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)
						Expect(err).To(HaveOccurred())
					})

					It("create should pass because shoot tolerates all taints of the seed", func() {
						seed.Spec.Taints = []core.SeedTaint{{Key: core.SeedTaintProtected}}
						shoot.Spec.Tolerations = []core.Toleration{{Key: core.SeedTaintProtected}}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)
						Expect(err).ToNot(HaveOccurred())
					})

					It("update should pass because shoot tolerates all taints of the seed", func() {
						seed.Spec.Taints = []core.SeedTaint{{Key: "foo"}}
						shoot.Spec.Tolerations = []core.Toleration{{Key: "foo", Value: pointer.String("bar")}}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)
						Expect(err).ToNot(HaveOccurred())
					})
				})

				Context("seed capacity", func() {
					var (
						allocatableShoots resource.Quantity
					)

					BeforeEach(func() {
						allocatableShoots = *resource.NewQuantity(1, resource.DecimalSI)

						shoot.Spec.DNS = nil
					})

					It("should pass because seed allocatable capacity is not set", func() {
						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)
						Expect(err).NotTo(HaveOccurred())
					})

					It("should pass because seed allocatable capacity is not exhausted", func() {
						seed.Status.Allocatable = corev1.ResourceList{"shoots": allocatableShoots}

						otherShoot := shoot.DeepCopy()
						otherShoot.Name = "other-shoot-1"
						otherShoot.Spec.SeedName = pointer.String("other-seed")
						Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

						otherShoot = shoot.DeepCopy()
						otherShoot.Name = "other-shoot-2"
						otherShoot.Spec.SeedName = nil
						Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)
						Expect(err).NotTo(HaveOccurred())
					})

					It("should reject because seed allocatable capacity is exhausted", func() {
						seed.Status.Allocatable = corev1.ResourceList{"shoots": allocatableShoots}

						otherShoot := shoot.DeepCopy()
						otherShoot.Name = "other-shoot-1"
						otherShoot.Spec.SeedName = &seedName
						Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

						otherShoot = shoot.DeepCopy()
						otherShoot.Name = "other-shoot-2"
						otherShoot.Spec.SeedName = nil
						Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)
						Expect(err).To(MatchError(ContainSubstring("already has the maximum number of shoots scheduled on it")))
					})

					It("should reject because seed allocatable capacity is over-exhausted", func() {
						seed.Status.Allocatable = corev1.ResourceList{"shoots": allocatableShoots}

						otherShoot := shoot.DeepCopy()
						otherShoot.Name = "other-shoot-1"
						otherShoot.Spec.SeedName = &seedName
						Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

						otherShoot = shoot.DeepCopy()
						otherShoot.Name = "other-shoot-2"
						otherShoot.Spec.SeedName = &seedName
						Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)
						Expect(err).To(MatchError(ContainSubstring("already has the maximum number of shoots scheduled on it")))
					})
				})
			})

			Context("networking settings checks", func() {
				var (
					oldShoot *core.Shoot
				)

				BeforeEach(func() {
					oldShoot = shoot.DeepCopy()
					oldShoot.Spec.SeedName = nil
				})

				It("update should pass because validation of network disjointedness should not be executed", func() {
					// set shoot pod cidr to overlap with vpn pod cidr
					shoot.Spec.Networking.Pods = pointer.String(v1beta1constants.DefaultVpnRange)
					oldShoot.Spec.SeedName = shoot.Spec.SeedName
					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)
					Expect(err).ToNot(HaveOccurred())
				})

				It("update should fail because validation of network disjointedness is executed", func() {
					// set shoot pod cidr to overlap with vpn pod cidr
					shoot.Spec.Networking.Pods = pointer.String(v1beta1constants.DefaultVpnRange)
					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)
					Expect(err).To(BeForbiddenError())
				})

				It("should reject because shoot pods network is missing", func() {
					shoot.Spec.Networking.Pods = nil

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because shoot services network is missing", func() {
					shoot.Spec.Networking.Services = nil

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should default shoot networks if seed provides ShootDefaults", func() {
					seed.Spec.Networks.ShootDefaults = &core.ShootNetworks{
						Pods:     &podsCIDR,
						Services: &servicesCIDR,
					}
					shoot.Spec.Networking.Pods = nil
					shoot.Spec.Networking.Services = nil

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.Networking.Pods).To(Equal(&podsCIDR))
					Expect(shoot.Spec.Networking.Services).To(Equal(&servicesCIDR))
				})

				It("should reject because the shoot node and the seed node networks intersect", func() {
					shoot.Spec.Networking.Nodes = &seedNodesCIDR

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the shoot pod and the seed pod networks intersect", func() {
					shoot.Spec.Networking.Pods = &seedPodsCIDR

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the shoot service and the seed service networks intersect", func() {
					shoot.Spec.Networking.Services = &seedServicesCIDR

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})
			})

			Context("dns settings checks", func() {
				It("should reject because the specified domain is already used by another shoot", func() {
					anotherShoot := shoot.DeepCopy()
					anotherShoot.Name = "another-shoot"

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the specified domain is a subdomain of a domain already used by another shoot", func() {
					anotherShoot := shoot.DeepCopy()
					anotherShoot.Name = "another-shoot"

					subdomain := fmt.Sprintf("subdomain.%s", *anotherShoot.Spec.DNS.Domain)
					shoot.Spec.DNS.Domain = &subdomain

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the specified domain is a subdomain of a domain already used by another shoot (case one)", func() {
					anotherShoot := shoot.DeepCopy()
					anotherShoot.Name = "another-shoot"

					subdomain := fmt.Sprintf("subdomain.%s", *anotherShoot.Spec.DNS.Domain)
					shoot.Spec.DNS.Domain = &subdomain

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the specified domain is a subdomain of a domain already used by another shoot (case two)", func() {
					anotherShoot := shoot.DeepCopy()
					anotherShoot.Name = "another-shoot"

					shoot.Spec.DNS.Domain = &baseDomain

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should allow because the specified domain is not a subdomain of a domain already used by another shoot", func() {
					anotherShoot := shoot.DeepCopy()
					anotherShoot.Name = "another-shoot"

					anotherDomain := fmt.Sprintf("someprefix%s", *anotherShoot.Spec.DNS.Domain)
					shoot.Spec.DNS.Domain = &anotherDomain

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(BeNil())
				})
			})

			Context("kubernetes version checks", func() {
				It("should reject due to an invalid kubernetes version", func() {
					shoot.Spec.Kubernetes.Version = "1.2.3"

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should default a major.minor kubernetes version to latest patch version", func() {
					shoot.Spec.Kubernetes.Version = "1.6"
					highestPatchVersion := core.ExpirableVersion{Version: "1.6.6"}
					cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, highestPatchVersion, core.ExpirableVersion{Version: "1.7.1"}, core.ExpirableVersion{Version: "1.7.2"})

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(Not(HaveOccurred()))
					Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestPatchVersion.Version))
				})

				It("should default a major.minor kubernetes version only to non-preview versions", func() {
					shoot.Spec.Kubernetes.Version = "1.6"
					preview := core.ClassificationPreview
					previewVersion := core.ExpirableVersion{Version: "1.6.6", Classification: &preview}
					highestNonPreviewPatchVersion := core.ExpirableVersion{Version: "1.6.5"}
					cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, previewVersion, highestNonPreviewPatchVersion, core.ExpirableVersion{Version: "1.7.1"}, core.ExpirableVersion{Version: "1.7.2"})

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(Not(HaveOccurred()))
					Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestNonPreviewPatchVersion.Version))
				})

				It("should reject defaulting a major.minor kubernetes version if there is no higher non-preview version available for defaulting", func() {
					shoot.Spec.Kubernetes.Version = "1.6"
					preview := core.ClassificationPreview
					previewVersion := core.ExpirableVersion{Version: "1.6.6", Classification: &preview}
					highestNonPreviewPatchVersion := core.ExpirableVersion{Version: "1.6.5", Classification: &preview}
					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{previewVersion, highestNonPreviewPatchVersion, {Version: "1.7.1"}, {Version: "1.7.2"}}

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should be able to explicitly pick preview versions", func() {
					shoot.Spec.Kubernetes.Version = "1.6.6"
					preview := core.ClassificationPreview
					previewVersion := core.ExpirableVersion{Version: "1.6.6", Classification: &preview}
					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{previewVersion}

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(Not(HaveOccurred()))
				})

				It("should reject: default only exactly matching minor kubernetes version", func() {
					shoot.Spec.Kubernetes.Version = "1.8"
					highestPatchVersion := core.ExpirableVersion{Version: "1.81.5"}
					cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, core.ExpirableVersion{Version: "1.81.0"}, highestPatchVersion)

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})
			})

			Context("machine image checks", func() {
				var (
					classificationPreview = core.ClassificationPreview

					imageName1 = "some-image"
					imageName2 = "other-image"

					expiredVersion          = "1.1.1"
					expiringVersion         = "1.2.1"
					nonExpiredVersion1      = "2.0.0"
					nonExpiredVersion2      = "2.0.1"
					latestNonExpiredVersion = "2.1.0"
					previewVersion          = "3.0.0"

					cloudProfileMachineImages = []core.MachineImage{
						{
							Name: imageName1,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        previewVersion,
										Classification: &classificationPreview,
									},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: latestNonExpiredVersion,
									},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: nonExpiredVersion1,
									},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: nonExpiredVersion2,
									},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        expiringVersion,
										ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * 1000)},
									},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        expiredVersion,
										ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
									},
								},
							},
						}, {
							Name: imageName2,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        previewVersion,
										Classification: &classificationPreview,
									},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: latestNonExpiredVersion,
									},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: nonExpiredVersion1,
									},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: nonExpiredVersion2,
									},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        expiringVersion,
										ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * 1000)},
									},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        expiredVersion,
										ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
									},
								},
							},
						},
					}
				)

				BeforeEach(func() {
					cloudProfile.Spec.MachineImages = cloudProfileMachineImages
				})

				Context("create Shoot", func() {
					It("should reject due to an invalid machine image", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    "not-supported",
							Version: "not-supported",
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)

						Expect(err).To(BeForbiddenError())
					})

					It("should reject due to an invalid machine image (version unset)", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name: "not-supported",
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)

						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("image name \"not-supported\" is not supported"))
					})

					It("should reject due to a machine image with expiration date in the past", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    imageName1,
							Version: expiredVersion,
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)

						Expect(err).To(BeForbiddenError())
					})

					It("should default version to latest non-preview version as shoot does not specify one", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = nil

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(&core.ShootMachineImage{
							Name:    imageName1,
							Version: latestNonExpiredVersion,
						}))
					})

					It("should default version to latest non-preview version as shoot only specifies name", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name: imageName1,
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(&core.ShootMachineImage{
							Name:    imageName1,
							Version: latestNonExpiredVersion,
						}))
					})

					It("should allow supported CRI and CRs", func() {
						shoot.Spec.Provider.Workers = []core.Worker{
							{
								CRI: &core.CRI{
									Name: core.CRINameContainerD,
									ContainerRuntimes: []core.ContainerRuntime{
										{Type: "supported-cr-1"},
										{Type: "supported-cr-2"},
									},
								},
								Machine: core.Machine{
									Image: &core.ShootMachineImage{
										Name:    "cr-image-name",
										Version: "1.2.3",
									},
								},
							},
						}

						cloudProfile.Spec.MachineImages = append(
							cloudProfile.Spec.MachineImages,
							core.MachineImage{
								Name: "cr-image-name",
								Versions: []core.MachineImageVersion{
									{
										ExpirableVersion: core.ExpirableVersion{
											Version: "1.2.3",
										},
										CRI: []core.CRI{
											{
												Name: core.CRINameContainerD,
												ContainerRuntimes: []core.ContainerRuntime{
													{
														Type: "supported-cr-1",
													},
													{
														Type: "supported-cr-2",
													},
												},
											},
										},
									},
								},
							})

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})

					It("should reject unsupported CRI", func() {
						shoot.Spec.Provider.Workers = append(
							shoot.Spec.Provider.Workers,
							core.Worker{
								CRI: &core.CRI{
									Name: core.CRINameContainerD,
									ContainerRuntimes: []core.ContainerRuntime{
										{Type: "supported-cr-1"},
										{Type: "supported-cr-2"},
									},
								},
								Machine: core.Machine{
									Image: &core.ShootMachineImage{
										Name:    "cr-image-name",
										Version: "1.2.3",
									},
								},
							})

						cloudProfile.Spec.MachineImages = append(
							cloudProfile.Spec.MachineImages,
							core.MachineImage{
								Name: "cr-image-name",
								Versions: []core.MachineImageVersion{
									{
										ExpirableVersion: core.ExpirableVersion{
											Version: "1.2.3",
										},
										CRI: []core.CRI{
											{
												Name: "unsupported-cri",
												ContainerRuntimes: []core.ContainerRuntime{
													{
														Type: "supported-cr-1",
													},
													{
														Type: "supported-cr-2",
													},
												},
											},
										},
									},
								},
							})

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)

						Expect(err).To(BeForbiddenError())
					})

					It("should reject unsupported CR", func() {
						shoot.Spec.Provider.Workers = append(
							shoot.Spec.Provider.Workers,
							core.Worker{
								CRI: &core.CRI{
									Name: core.CRINameContainerD,
									ContainerRuntimes: []core.ContainerRuntime{
										{Type: "supported-cr-1"},
										{Type: "supported-cr-2"},
										{Type: "unsupported-cr-1"},
									},
								},
								Machine: core.Machine{
									Image: &core.ShootMachineImage{
										Name:    "cr-image-name",
										Version: "1.2.3",
									},
								},
							})

						cloudProfile.Spec.MachineImages = append(
							cloudProfile.Spec.MachineImages,
							core.MachineImage{
								Name: "cr-image-name",
								Versions: []core.MachineImageVersion{
									{
										ExpirableVersion: core.ExpirableVersion{
											Version: "1.2.3",
										},
										CRI: []core.CRI{
											{
												Name: core.CRINameContainerD,
												ContainerRuntimes: []core.ContainerRuntime{
													{Type: "supported-cr-1"},
													{Type: "supported-cr-2"},
												},
											},
										},
									},
								},
							})

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)

						Expect(err).To(BeForbiddenError())
						Expect(err.Error()).To(ContainSubstring("Unsupported value: core.ContainerRuntime{Type:\"unsupported-cr-1\""))
					})
				})

				Context("update Shoot", func() {
					BeforeEach(func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    imageName1,
							Version: nonExpiredVersion1,
						}
					})

					It("should deny updating to an MachineImage which does not support the selected container runtime", func() {
						cloudProfile.Spec.MachineImages = append(
							cloudProfile.Spec.MachineImages,
							core.MachineImage{
								Name: "cr-image-name",
								Versions: []core.MachineImageVersion{
									{
										ExpirableVersion: core.ExpirableVersion{
											Version: "1.2.3",
										},
										CRI: []core.CRI{
											{
												Name: core.CRINameContainerD,
											},
										},
									},
								},
							})

						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    "cr-image-name",
							Version: "1.2.3",
						}
						shoot.Spec.Provider.Workers[0].CRI = &core.CRI{Name: core.CRINameContainerD}
						newShoot := shoot.DeepCopy()
						newShoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    imageName1,
							Version: latestNonExpiredVersion,
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)

						Expect(err).To(HaveOccurred())
					})

					It("should deny updating to an MachineImageVersion which does not support the selected container runtime", func() {
						cloudProfile.Spec.MachineImages = append(
							cloudProfile.Spec.MachineImages,
							core.MachineImage{
								Name: "cr-image-name",
								Versions: []core.MachineImageVersion{
									{
										ExpirableVersion: core.ExpirableVersion{
											Version: "1.2.3",
										},
										CRI: []core.CRI{
											{
												Name: core.CRINameContainerD,
											},
										},
									},
									{
										ExpirableVersion: core.ExpirableVersion{
											Version: "2.3.4",
										},
									},
								},
							})

						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    "cr-image-name",
							Version: "1.2.3",
						}
						shoot.Spec.Provider.Workers[0].CRI = &core.CRI{Name: core.CRINameContainerD}
						newShoot := shoot.DeepCopy()
						newShoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    "cr-image-name",
							Version: "2.3.4",
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)

						Expect(err).To(HaveOccurred())
					})

					It("should keep machine image of the old shoot (unset in new shoot)", func() {
						newShoot := shoot.DeepCopy()
						newShoot.Spec.Provider.Workers[0].Machine.Image = nil

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(*newShoot).To(Equal(shoot))
					})

					It("should keep machine image of the old shoot (version unset in new shoot)", func() {

						newShoot := shoot.DeepCopy()
						newShoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name: imageName1,
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(*newShoot).To(Equal(shoot))
					})

					It("should use updated machine image version as specified", func() {
						newShoot := shoot.DeepCopy()
						newShoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    imageName1,
							Version: nonExpiredVersion2,
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(newShoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(&core.ShootMachineImage{
							Name:    imageName1,
							Version: nonExpiredVersion2,
						}))
					})

					It("should default version of new worker pool to latest non-preview version", func() {
						newShoot := shoot.DeepCopy()
						newWorker := newShoot.Spec.Provider.Workers[0].DeepCopy()
						newWorker.Name = "second-worker"
						newWorker.Machine.Image = nil
						newShoot.Spec.Provider.Workers = append(newShoot.Spec.Provider.Workers, *newWorker)

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), newShoot.Namespace, newShoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.CreateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(newShoot.Spec.Provider.Workers[0]).To(Equal(shoot.Spec.Provider.Workers[0]))
						Expect(newShoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(&core.ShootMachineImage{
							Name:    imageName1,
							Version: latestNonExpiredVersion,
						}))
					})

					It("should default version of new worker pool to latest non-preview version (version unset)", func() {
						newShoot := shoot.DeepCopy()
						newWorker := newShoot.Spec.Provider.Workers[0].DeepCopy()
						newWorker.Name = "second-worker"
						newWorker.Machine.Image = &core.ShootMachineImage{
							Name: imageName2,
						}
						newShoot.Spec.Provider.Workers = append(newShoot.Spec.Provider.Workers, *newWorker)

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), newShoot.Namespace, newShoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.CreateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(newShoot.Spec.Provider.Workers[0]).To(Equal(shoot.Spec.Provider.Workers[0]))
						Expect(newShoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(&core.ShootMachineImage{
							Name:    imageName2,
							Version: latestNonExpiredVersion,
						}))
					})

					It("should use version of new worker pool as specified", func() {
						newShoot := shoot.DeepCopy()
						newWorker := newShoot.Spec.Provider.Workers[0].DeepCopy()
						newWorker.Name = "second-worker"
						newWorker.Machine.Image = &core.ShootMachineImage{
							Name:    imageName2,
							Version: nonExpiredVersion1,
						}
						newShoot.Spec.Provider.Workers = append(newShoot.Spec.Provider.Workers, *newWorker)

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), newShoot.Namespace, newShoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.CreateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(newShoot.Spec.Provider.Workers[0]).To(Equal(shoot.Spec.Provider.Workers[0]))
						Expect(newShoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(&core.ShootMachineImage{
							Name:    imageName2,
							Version: nonExpiredVersion1,
						}))
					})

					It("should default version of new image to latest non-preview version (version unset)", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    imageName1,
							Version: nonExpiredVersion2,
						}

						newShoot := shoot.DeepCopy()
						newShoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name: imageName2,
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), newShoot.Namespace, newShoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.CreateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(newShoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(&core.ShootMachineImage{
							Name:    imageName2,
							Version: latestNonExpiredVersion,
						}))
					})

					It("should use version of new image as specified", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    imageName1,
							Version: nonExpiredVersion2,
						}

						newShoot := shoot.DeepCopy()
						newShoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    imageName2,
							Version: nonExpiredVersion2,
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), newShoot.Namespace, newShoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.CreateOptions{}, false, nil)

						err := admissionHandler.Admit(context.TODO(), attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(newShoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(&core.ShootMachineImage{
							Name:    imageName2,
							Version: nonExpiredVersion2,
						}))
					})
				})
			})

			Context("machine type checks", func() {
				It("should not reject due to an usable machine type", func() {
					shoot.Spec.Provider.Workers = []core.Worker{
						{
							Machine: core.Machine{
								Type: "machine-type-1",
							},
						},
					}

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should reject due to a not usable machine type", func() {
					shoot.Spec.Provider.Workers = []core.Worker{
						{
							Machine: core.Machine{
								Type: "machine-type-old",
							},
						},
					}

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject due to an invalid machine type", func() {
					shoot.Spec.Provider.Workers = []core.Worker{
						{
							Machine: core.Machine{
								Type: "not-allowed",
							},
						},
					}

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})
			})

			Context("volume checks", func() {
				It("should reject due to an invalid volume type", func() {
					notAllowed := "not-allowed"
					shoot.Spec.Provider.Workers = []core.Worker{
						{
							Machine: core.Machine{
								Type: "machine-type-1",
							},
							Volume: &core.Volume{
								Type: &notAllowed,
							},
						},
					}

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should allow volume removal", func() {
					oldShoot := shoot.DeepCopy()
					shoot.Spec.Provider.Workers[0].Volume = nil
					oldShoot.Spec.Provider.Workers[0].Volume.VolumeSize = "20Gi"

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).ToNot(HaveOccurred())
				})

				It("should reject due to wrong volume size (volume type constraint)", func() {
					boundaryVolSize := minVolSize
					boundaryVolSize.Add(resource.MustParse("-1"))

					boundaryVolSizeMachine := minVolSizeMachine
					boundaryVolSizeMachine.Add(resource.MustParse("-1"))

					shoot.Spec.Provider.Workers = []core.Worker{
						{
							Machine: core.Machine{
								Type: "machine-type-1",
							},
							Volume: &core.Volume{
								Type:       &volumeType2,
								VolumeSize: boundaryVolSize.String(),
							},
						},
						{
							Machine: core.Machine{
								Type: "machine-type-2",
							},
							Volume: &core.Volume{
								Type:       &volumeType,
								VolumeSize: boundaryVolSize.String(),
							},
						},
						{
							Machine: core.Machine{
								Type: "machine-type-2",
							},
							Volume: &core.Volume{
								Type:       &volumeType,
								VolumeSize: boundaryVolSizeMachine.String(),
							},
						},
					}

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err.Error()).To(ContainSubstring("spec.provider.workers[0].volume.size"))
					Expect(err.Error()).To(ContainSubstring("spec.provider.workers[2].volume.size"))
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

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("control plane migration", func() {
			It("should fail to change Seed name, because Seed doesn't have configuration for backup", func() {
				oldSeedName := fmt.Sprintf("old-%s", seedName)
				oldSeed := seed.DeepCopy()
				oldSeed.Name = oldSeedName
				seed.Spec.Backup = nil

				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.SeedName = &oldSeedName

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(oldSeed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("backup is not configured for seed %q", seedName))))
			})

			It("should fail to change Seed name, because old Seed doesn't have configuration for backup", func() {
				oldSeedName := fmt.Sprintf("old-%s", seedName)
				oldSeed := seed.DeepCopy()
				oldSeed.Name = oldSeedName
				oldSeed.Spec.Backup = nil

				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.SeedName = &oldSeedName

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(oldSeed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("backup is not configured for old seed %q", oldSeedName))))
			})

			It("should fail to change Seed name, because cloud provider for new Seed is not equal to cloud provider for old Seed", func() {
				oldSeedName := fmt.Sprintf("old-%s", seedName)
				oldSeed := seed.DeepCopy()
				oldSeed.Name = oldSeedName
				oldSeed.Spec.Provider.Type = "gcp"
				seed.Spec.Provider.Type = "aws"

				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.SeedName = &oldSeedName

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(oldSeed)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("shoot deletion", func() {
			var shootStore cache.Store

			BeforeEach(func() {
				shootStore = coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore()
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
			})

			DescribeTable("DeleteShootInMigration",
				func(lastOperationType core.LastOperationType, lastOperationState core.LastOperationState, matcher types.GomegaMatcher) {
					shootBase.Status = core.ShootStatus{
						LastOperation: &core.LastOperation{
							Type:  lastOperationType,
							State: lastOperationState,
						},
					}
					shoot.Annotations = map[string]string{
						gutil.ConfirmationDeletion: "true",
					}
					attrs := admission.NewAttributesRecord(nil, shootBase.DeepCopyObject(), core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

					Expect(shootStore.Add(&shoot)).NotTo(HaveOccurred())

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(matcher)
				},
				Entry("should reject if the shoot has lastOperation=Migrate:Processing", core.LastOperationTypeMigrate, core.LastOperationStateProcessing, BeForbiddenError()),
				Entry("should reject if the shoot has lastOperation=Migrate:Succeeded", core.LastOperationTypeMigrate, core.LastOperationStateSucceeded, BeForbiddenError()),
				Entry("should reject if the shoot has lastOperation=Migrate:Error", core.LastOperationTypeMigrate, core.LastOperationStateError, BeForbiddenError()),
				Entry("should reject if the shoot has lastOperation=Restore:Processing", core.LastOperationTypeRestore, core.LastOperationStateProcessing, BeForbiddenError()),
				Entry("should reject if the shoot has lastOperation=Restore:Error", core.LastOperationTypeRestore, core.LastOperationStateError, BeForbiddenError()),
				Entry("should not reject if the shoot has lastOperation=Restore:Succeeded ", core.LastOperationTypeRestore, core.LastOperationStateSucceeded, BeNil()),
				Entry("should not reject the delete operation", core.LastOperationTypeReconcile, core.LastOperationStateSucceeded, BeNil()))
		})
	})
})
