// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/features"
	mockauthorizer "github.com/gardener/gardener/pkg/mock/apiserver/authorization/authorizer"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/plugin/pkg/shoot/validator"
)

var _ = Describe("validator", func() {
	Describe("#Admit", func() {
		var (
			ctx                 context.Context
			admissionHandler    *ValidateShoot
			ctrl                *gomock.Controller
			auth                *mockauthorizer.MockAuthorizer
			coreInformerFactory gardencoreinformers.SharedInformerFactory
			cloudProfile        core.CloudProfile
			seed                core.Seed
			secretBinding       core.SecretBinding
			project             core.Project
			shoot               core.Shoot

			userInfo            = &user.DefaultInfo{Name: "foo"}
			authorizeAttributes authorizer.AttributesRecord

			podsCIDR     = "100.96.0.0/11"
			servicesCIDR = "100.64.0.0/13"
			nodesCIDR    = "10.250.0.0/16"

			falseVar = false

			seedName      = "seed"
			namespaceName = "garden-my-project"
			projectName   = "my-project"
			newSeedName   = "new-seed"

			unmanagedDNSProvider = core.DNSUnmanaged
			baseDomain           = "example.com"

			validMachineImageName     = "some-machineimage"
			validMachineImageVersions = []core.MachineImageVersion{
				{
					ExpirableVersion: core.ExpirableVersion{
						Version: "0.0.1",
					},
					CRI: []core.CRI{
						{
							Name: core.CRINameContainerD,
							ContainerRuntimes: []core.ContainerRuntime{
								{
									Type: "test-cr",
								},
							},
						},
					},
					Architectures: []string{"amd64", "arm64"},
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
							Name:         "machine-type-1",
							CPU:          resource.MustParse("2"),
							GPU:          resource.MustParse("0"),
							Memory:       resource.MustParse("100Gi"),
							Architecture: pointer.String("amd64"),
						},
						{
							Name:         "machine-type-old",
							CPU:          resource.MustParse("2"),
							GPU:          resource.MustParse("0"),
							Memory:       resource.MustParse("100Gi"),
							Usable:       &falseVar,
							Architecture: pointer.String("amd64"),
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
							Architecture: pointer.String("amd64"),
						},
						{
							Name:         "machine-type-3",
							CPU:          resource.MustParse("2"),
							GPU:          resource.MustParse("0"),
							Memory:       resource.MustParse("100Gi"),
							Architecture: pointer.String("arm64"),
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
			secretBindingBase = core.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: namespaceName,
				},
				Provider: &core.SecretBindingProvider{
					Type: "unknown",
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
									Architecture: pointer.String("amd64"),
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
						WorkersSettings: &core.WorkersSettings{
							SSHAccess: &core.SSHAccess{
								Enabled: true,
							},
						},
						InfrastructureConfig: &runtime.RawExtension{Raw: []byte(`{
"kind": "InfrastructureConfig",
"apiVersion": "some.random.config/v1beta1"}`)},
					},
				},
			}

			shootStateBase = core.ShootState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootBase.Name,
					Namespace: shootBase.Namespace,
				},
				Spec: core.ShootStateSpec{
					Gardener: []core.GardenerResourceData{
						{
							Labels: map[string]string{
								"name":       "kube-apiserver-etcd-encryption-key",
								"managed-by": "secrets-manager",
							},
						},
					},
				},
			}
		)

		BeforeEach(func() {
			ctx = context.Background()
			ctrl = gomock.NewController(GinkgoT())
			auth = nil

			project = projectBase
			cloudProfile = *cloudProfileBase.DeepCopy()
			seed = seedBase
			secretBinding = secretBindingBase
			shoot = *shootBase.DeepCopy()

			admissionHandler, _ = New()
			admissionHandler.SetAuthorizer(auth)
			admissionHandler.AssignReadyFunc(func() bool { return true })
			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalCoreInformerFactory(coreInformerFactory)

			authorizeAttributes = authorizer.AttributesRecord{
				User:            userInfo,
				APIGroup:        "core.gardener.cloud",
				Resource:        "shoots",
				Subresource:     "binding",
				Namespace:       shoot.Namespace,
				Name:            shoot.Name,
				Verb:            "update",
				ResourceRequest: true,
			}
		})

		JustBeforeEach(func() {
			if auth == nil {
				auth = mockauthorizer.NewMockAuthorizer(ctrl)
				auth.EXPECT().Authorize(ctx, gomock.Any()).Return(authorizer.DecisionAllow, "", nil).AnyTimes()
			}
			admissionHandler.SetAuthorizer(auth)
		})

		Context("name/project length checks", func() {
			It("should reject create operations on Shoot resources in projects which shall be deleted", func() {
				deletionTimestamp := metav1.NewTime(time.Now())
				project.ObjectMeta.DeletionTimestamp = &deletionTimestamp

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

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
				Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

				authorizeAttributes.Name = shoot.Name

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeInvalidError())
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
				err := admissionHandler.Admit(ctx, attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).NotTo(ContainSubstring("name must not exceed"))

				attrs = admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
				err = admissionHandler.Admit(ctx, attrs, nil)
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
				Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

				authorizeAttributes.Name = shoot.Name

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

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
				Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

				authorizeAttributes.Name = shoot.Name

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeInvalidError())
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
				finalizers := sets.New[string](oldShoot.GetFinalizers()...)
				finalizers.Insert(core.GardenerName)
				oldShoot.SetFinalizers(finalizers.UnsortedList())
			})

			It("should reject removing the gardener finalizer if the shoot has not yet been deleted successfully", func() {
				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)
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
				err := admissionHandler.Admit(ctx, attrs, nil)
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
					err := admissionHandler.Admit(ctx, attrs, nil)
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
					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())

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
				Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot schedule shoot '%s' on seed '%s' that is already marked for deletion", shoot.Name, seed.Name))
			})

			It("should allow no-op updates", func() {
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})

			It("should allow modifying the finalizers array", func() {
				oldShoot.Finalizers = []string{core.GardenerName}
				shoot.Finalizers = []string{}

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})

			It("should allow adding the deletion confirmation", func() {
				shoot.Annotations = make(map[string]string)
				shoot.Annotations[gardenerutils.ConfirmationDeletion] = "true"

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})

			It("should reject modifying the shoot spec when seed is marked for deletion", func() {
				shoot.Spec.Region = "other-region"

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot update spec of shoot '%s' on seed '%s' already marked for deletion", shoot.Name, seed.Name))
			})

			It("should reject modifying other annotations than the deletion confirmation when seed is marked for deletion", func() {
				shoot.Annotations = make(map[string]string)
				shoot.Annotations["foo"] = "bar"

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot update annotations of shoot '%s' on seed '%s' already marked for deletion", shoot.Name, seed.Name))
			})
		})

		Context("setting spec.seedName", func() {
			BeforeEach(func() {
				auth = mockauthorizer.NewMockAuthorizer(ctrl)
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
			})

			It("should allow setting the seedName on create if the user has required permissions", func() {
				auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionAllow, "", nil)

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should deny setting the seedName on create if the user lacks required permissions", func() {
				auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionDeny, "", nil)

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err).To(MatchError(ContainSubstring("user %q is not allowed to set .spec.seedName for %q", userInfo.Name, "shoots")))
			})
		})

		Context("seedName change", func() {
			var (
				oldShoot   core.Shoot
				newSeed    core.Seed
				shootState core.ShootState
			)
			BeforeEach(func() {
				oldShoot = *shootBase.DeepCopy()

				seed = *seedBase.DeepCopy()
				newSeed = *seedBase.DeepCopy()
				newSeed.Name = "new-seed"
				shootState = *shootStateBase.DeepCopy()

				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&newSeed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().ShootStates().Informer().GetStore().Add(&shootState)).To(Succeed())
			})

			It("should not allow changing the seedName on admission.Update if the subresource is not binding", func() {
				shoot.Spec.SeedName = &newSeed.Name

				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err).To(MatchError(ContainSubstring("spec.seedName cannot be changed by patching the shoot, please use the shoots/binding subresource")))
			})

			It("should not forbid changing the seedName on admission.Update if the subresource is binding", func() {
				shoot.Spec.SeedName = &newSeed.Name

				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeNil())
			})

			It("should not allow setting the seedName to nil on admission.Update if the subresource is not binding", func() {
				shoot.Spec.SeedName = nil

				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err).To(MatchError(ContainSubstring("spec.seedName cannot be changed by patching the shoot, please use the shoots/binding subresource")))
			})

			It("should not allow setting the seedName to nil on admission.Update even if the subresource is binding", func() {
				shoot.Spec.SeedName = nil

				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err).To(MatchError(ContainSubstring("spec.seedName cannot be set to nil")))
			})

			It("should not allow setting seedName even if old seedName was nil on admission.Update if the subresource is not binding", func() {
				oldShoot.Spec.SeedName = nil

				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err).To(MatchError(ContainSubstring("spec.seedName cannot be changed by patching the shoot, please use the shoots/binding subresource")))
			})

			It("should reject update of binding when shoot.spec.seedName is not nil and the binding has the same seedName", func() {
				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("update of binding rejected, shoot is already assigned to the same seed")))
			})
		})

		Context("reference checks", func() {
			It("should reject because the referenced cloud profile was not found", func() {

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeInternalServerError())
			})

			It("should reject because the referenced seed was not found", func() {
				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeInternalServerError())
			})

			It("should reject because the referenced project was not found", func() {
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeInternalServerError())
			})

			It("should reject because the cloud provider in shoot and profile differ", func() {
				cloudProfile.Spec.Type = "gcp"
				shoot.Spec.Provider.Type = "aws"

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("provider type in shoot must equal provider type of referenced CloudProfile: %q", cloudProfile.Spec.Type))
			})

			It("should reject because the cloud provider in shoot and secret binding differ", func() {
				secretBinding.Provider = &core.SecretBindingProvider{
					Type: "gcp",
				}
				shoot.Spec.Provider.Type = "aws"
				cloudProfile.Spec.Type = "aws"

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("provider type in shoot must match provider type of referenced SecretBinding: %q", secretBinding.Provider.Type))
			})

			It("should pass because no seed has to be specified (however can be). The scheduler sets the seed instead.", func() {
				shoot.Spec.SeedName = nil
				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("tests deploy task", func() {
			var (
				oldShoot *core.Shoot
			)

			BeforeEach(func() {
				oldShoot = shootBase.DeepCopy()
				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
			})

			It("should add deploy tasks because shoot is being created", func() {
				Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployInfrastructure")).To(BeTrue())
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployDNSRecordInternal")).To(BeTrue())
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployDNSRecordExternal")).To(BeTrue())
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployDNSRecordIngress")).To(BeTrue())
			})

			It("should add deploy tasks because shoot is waking up from hibernation", func() {
				oldShoot.Spec.Hibernation = &core.Hibernation{
					Enabled: pointer.Bool(true),
				}
				shoot.Spec.Hibernation = &core.Hibernation{
					Enabled: pointer.Bool(false),
				}

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployInfrastructure")).To(BeTrue())
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployDNSRecordInternal")).To(BeTrue())
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployDNSRecordExternal")).To(BeTrue())
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployDNSRecordIngress")).To(BeTrue())
			})

			It("should add deploy infrastructure task because infrastructure config has changed", func() {
				shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{
					Raw: []byte("infrastructure"),
				}

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployInfrastructure")).To(BeTrue())
			})

			It("should add deploy infrastructure task because SSHAccess in WorkersSettings config has changed", func() {
				shoot.Spec.Provider.WorkersSettings = &core.WorkersSettings{
					SSHAccess: &core.SSHAccess{
						Enabled: false,
					},
				}

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployInfrastructure")).To(BeTrue())
			})

			It("should add deploy dnsrecord tasks because dns config has changed", func() {
				shoot.Spec.DNS = &core.DNS{}

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployDNSRecordInternal")).To(BeTrue())
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployDNSRecordExternal")).To(BeTrue())
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployDNSRecordIngress")).To(BeTrue())
			})

			It("should add deploy infrastructure task because shoot operation annotation to rotate ssh keypair was set", func() {
				shoot.Annotations = make(map[string]string)
				shoot.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.ShootOperationRotateSSHKeypair

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployInfrastructure")).To(BeTrue())
			})

			It("should add deploy infrastructure task because shoot operation annotation to rotate all credentials was set", func() {
				shoot.Annotations = make(map[string]string)
				shoot.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.OperationRotateCredentialsStart

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployInfrastructure")).To(BeTrue())
			})

			It("should not add deploy tasks because spec has not changed", func() {
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployInfrastructure")).To(BeFalse())
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployDNSRecordInternal")).To(BeFalse())
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployDNSRecordExternal")).To(BeFalse())
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployDNSRecordIngress")).To(BeFalse())
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
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
			})

			It("should reject update because shoot changed to unknown region", func() {
				shoot.Spec.Region = "does-not-exist"

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("Unsupported value: \"does-not-exist\": supported values: \"europe\", \"asia\""))
			})

			It("should pass update for non existing zone in cloud profile because shoot worker zone is unchanged", func() {
				cloudProfile.Spec.Regions[0].Zones = []core.AvailabilityZone{{Name: "not-available"}}

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
			})

			It("should reject update because shoot changed to region with unavailable zone", func() {
				shoot.Spec.Region = "asia"

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("Unsupported value: \"europe-a\": supported values: \"asia-a\""))
			})

			It("should reject update because shoot and cloud profile changed zones", func() {
				cloudProfile.Spec.Regions[0].Zones = []core.AvailabilityZone{{Name: "zone-1"}, {Name: "zone-2"}}
				shoot.Spec.Provider.Workers[0].Zones = append(shoot.Spec.Provider.Workers[0].Zones, "zone-1")

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("Unsupported value: \"europe-a\": supported values: \"zone-1\", \"zone-2\""))
			})

			It("should reject due to an invalid zone", func() {
				shoot.Spec.Provider.Workers[0].Zones = []string{"invalid-zone"}

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
			})

			It("should reject due to a duplicate zone", func() {
				shoot.Spec.Provider.Workers[0].Zones = []string{"europe-a", "europe-a"}

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

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
				err := admissionHandler.Admit(ctx, attrs, nil)

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
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("tests for unknown provider", func() {
			Context("scheduling checks for Create operation", func() {
				var (
					oldShoot   *core.Shoot
					shootState core.ShootState
				)

				BeforeEach(func() {
					oldShoot = shoot.DeepCopy()
					oldShoot.Spec.SeedName = nil
					shootState = *shootStateBase.DeepCopy()

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().ShootStates().Informer().GetStore().Add(&shootState)).To(Succeed())
				})

				Context("taints and tolerations", func() {
					It("create should pass because the Seed specified in shoot manifest does not have any taints", func() {
						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).ToNot(HaveOccurred())
					})

					It("create should fail because the Seed specified in shoot manifest has non-tolerated taints", func() {
						seed.Spec.Taints = []core.SeedTaint{{Key: core.SeedTaintProtected}}

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
					})

					It("create should pass because shoot tolerates all taints of the seed", func() {
						seed.Spec.Taints = []core.SeedTaint{{Key: core.SeedTaintProtected}}
						shoot.Spec.Tolerations = []core.Toleration{{Key: core.SeedTaintProtected}}
						Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Update(&shoot)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).ToNot(HaveOccurred())
					})

					It("delete should pass even if the Seed specified in shoot manifest has non-tolerated taints", func() {
						seed.Spec.Taints = []core.SeedTaint{{Key: core.SeedTaintProtected}}

						attrs := admission.NewAttributesRecord(nil, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

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

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					})

					It("should pass because seed allocatable capacity is not set", func() {
						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

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

						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

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

						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

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

						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(MatchError(ContainSubstring("already has the maximum number of shoots scheduled on it")))
					})

					It("should allow Shoot deletion even though seed's allocatable capacity is exhausted / over exhausted", func() {
						seed.Status.Allocatable = corev1.ResourceList{"shoots": allocatableShoots}

						otherShoot := *shoot.DeepCopy()
						otherShoot.Name = "other-shoot-1"
						otherShoot.Spec.SeedName = &seedName
						Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&otherShoot)).To(Succeed())

						otherShoot = *shoot.DeepCopy()
						otherShoot.Name = "other-shoot-2"
						otherShoot.Spec.SeedName = &seedName
						Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&otherShoot)).To(Succeed())

						// admission for DELETION uses the old Shoot object
						oldShoot.Spec.SeedName = &seedName

						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).ToNot(HaveOccurred())
					})
				})

				Context("multi-zonal shoot scheduling checks on seed", func() {
					BeforeEach(func() {
						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					})

					Context("seed has less than 3 zones", func() {
						BeforeEach(func() {
							seed.Spec.Provider.Zones = []string{"1", "2"}
						})

						It("should allow scheduling non-HA shoot", func() {
							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
							Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
						})

						It("should allow scheduling HA shoot with failure tolerance type 'node'", func() {
							shoot.Annotations = make(map[string]string)
							shoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeNode}}}

							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
							Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
						})

						It("should reject scheduling HA shoot with failure tolerance type 'zone'", func() {
							shoot.Annotations = make(map[string]string)
							shoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeZone}}}

							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
							Expect(admissionHandler.Admit(ctx, attrs, nil)).To(BeForbiddenError())
						})
					})

					Context("seed has at least 3 zones", func() {
						BeforeEach(func() {
							seed.Spec.Provider.Zones = []string{"1", "2", "3"}
						})

						It("should allow scheduling non-HA shoot", func() {
							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
							Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
						})

						It("should allow scheduling HA shoot with failure tolerance type 'node'", func() {
							shoot.Annotations = make(map[string]string)
							shoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeNode}}}

							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
							Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
						})

						It("should allow scheduling HA shoot with failure tolerance type 'zone'", func() {
							shoot.Annotations = make(map[string]string)
							shoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeZone}}}

							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
							Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
						})
					})
				})

				Context("cloud profile's seed selector", func() {
					It("should reject shoot creation on seed when the cloud profile's seed selector is invalid", func() {
						cloudProfile.Spec.SeedSelector = &core.SeedSelector{
							LabelSelector: metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{Key: "domain", Operator: "invalid-operator", Values: []string{"foo"}},
								},
							},
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("label selector conversion failed"))
					})

					It("should allow shoot creation on seed that matches the cloud profile's seed selector", func() {
						cloudProfile.Spec.SeedSelector = &core.SeedSelector{
							LabelSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"domain": "foo"},
							},
						}
						seed.Labels = map[string]string{"domain": "foo"}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})

					It("should reject shoot creation on seed that does not match the cloud profile's seed selector", func() {
						cloudProfile.Spec.SeedSelector = &core.SeedSelector{
							LabelSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"domain": "foo"},
							},
						}
						seed.Labels = nil

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("cannot schedule shoot '%s' on seed '%s' because the seed selector of cloud profile '%s' is not matching the labels of the seed", shoot.Name, seed.Name, cloudProfile.Name))
					})

					It("should allow shoot creation on seed that matches one of the provider types in the cloud profile's seed selector", func() {
						cloudProfile.Spec.SeedSelector = &core.SeedSelector{
							ProviderTypes: []string{"foo", "bar", "baz"},
						}
						seed.Spec.Provider = core.SeedProvider{
							Type: "baz",
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})

					It("should reject shoot creation on seed that does not match any of the provider types in the cloud profile's seed selector", func() {
						cloudProfile.Spec.SeedSelector = &core.SeedSelector{
							ProviderTypes: []string{"foo", "bar"},
						}
						seed.Spec.Provider = core.SeedProvider{
							Type: "baz",
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("cannot schedule shoot '%s' on seed '%s' because none of the provider types in the seed selector of cloud profile '%s' is matching the provider type of the seed", shoot.Name, seed.Name, cloudProfile.Name))
					})

					It("should allow updating the seedName to seed that matches the cloud profile's seed selector (w/ shoots/binding subresource)", func() {
						cloudProfile.Spec.SeedSelector = &core.SeedSelector{
							LabelSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"domain": "foo"},
							},
						}
						seed.Labels = map[string]string{"domain": "foo"}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})

					It("should reject updating the seedName to seed that does not match the cloud profile's seed selector (w/ shoots/binding subresource)", func() {
						cloudProfile.Spec.SeedSelector = &core.SeedSelector{
							LabelSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"domain": "foo"},
							},
						}
						seed.Labels = nil

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("cannot schedule shoot '%s' on seed '%s' because the seed selector of cloud profile '%s' is not matching the labels of the seed", shoot.Name, seed.Name, cloudProfile.Name))
					})

					It("should allow updating the seedName to seed that matches one of the provider types in the cloud profile's seed selector (w/ shoots/binding subresource)", func() {
						cloudProfile.Spec.SeedSelector = &core.SeedSelector{
							LabelSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"domain": "foo"},
							},
						}
						seed.Labels = map[string]string{"domain": "foo"}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})

					It("should reject updating the seedName to seed that does not match any of the provider types in the cloud profile's seed selector (w/ shoots/binding subresource)", func() {
						cloudProfile.Spec.SeedSelector = &core.SeedSelector{
							ProviderTypes: []string{"foo", "bar"},
						}
						seed.Spec.Provider = core.SeedProvider{
							Type: "baz",
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("cannot schedule shoot '%s' on seed '%s' because none of the provider types in the seed selector of cloud profile '%s' is matching the provider type of the seed", shoot.Name, seed.Name, cloudProfile.Name))
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
					shoot.Spec.Networking.Pods = pointer.String(v1beta1constants.DefaultVPNRange)
					oldShoot.Spec.SeedName = shoot.Spec.SeedName

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).ToNot(HaveOccurred())
				})

				It("update should fail because validation of network disjointedness is executed", func() {
					// set shoot pod cidr to overlap with vpn pod cidr
					shoot.Spec.Networking.Pods = pointer.String(v1beta1constants.DefaultVPNRange)

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because shoot pods network is missing", func() {
					shoot.Spec.Networking.Pods = nil

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because shoot services network is missing", func() {
					shoot.Spec.Networking.Services = nil

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

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
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.Networking.Pods).To(Equal(&podsCIDR))
					Expect(shoot.Spec.Networking.Services).To(Equal(&servicesCIDR))
				})

				It("should reject because the shoot node and the seed node networks intersect", func() {
					shoot.Spec.Networking.Nodes = &seedNodesCIDR

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the shoot pod and the seed pod networks intersect", func() {
					shoot.Spec.Networking.Pods = &seedPodsCIDR

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the shoot service and the seed service networks intersect", func() {
					shoot.Spec.Networking.Services = &seedServicesCIDR

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the shoot pod and the seed node networks intersect", func() {
					shoot.Spec.Networking.Pods = &seedNodesCIDR

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the shoot service and the seed node networks intersect", func() {
					shoot.Spec.Networking.Services = &seedNodesCIDR

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the shoot service and the shoot node networks intersect", func() {
					shoot.Spec.Networking.Services = shoot.Spec.Networking.Nodes

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the shoot pod and the shoot node networks intersect", func() {
					shoot.Spec.Networking.Pods = shoot.Spec.Networking.Nodes

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the shoot pod and the shoot service networks intersect", func() {
					shoot.Spec.Networking.Pods = shoot.Spec.Networking.Services

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

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
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

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
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("the domain is already used by another shoot or it is a subdomain of an already used domain")))
				})

				It("should allow to delete the shoot although the specified domain is a subdomain of a domain already used by another shoot (case one)", func() {
					anotherShoot := shoot.DeepCopy()
					anotherShoot.Name = "another-shoot"

					subdomain := fmt.Sprintf("subdomain.%s", *anotherShoot.Spec.DNS.Domain)
					shoot.Spec.DNS.Domain = &subdomain

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, userInfo)
					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				})

				It("should allow to update the shoot with deletion confirmation annotation although the specified domain is a subdomain of a domain already used by another shoot (case one)", func() {
					anotherShoot := shoot.DeepCopy()
					anotherShoot.Name = "another-shoot"

					subdomain := fmt.Sprintf("subdomain.%s", *anotherShoot.Spec.DNS.Domain)
					shoot.Spec.DNS.Domain = &subdomain

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					oldShoot := shoot.DeepCopy()
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "confirmation.gardener.cloud/deletion", "true")
					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				})

				It("should reject because the specified domain is a subdomain of a domain already used by another shoot (case two)", func() {
					anotherShoot := shoot.DeepCopy()
					anotherShoot.Name = "another-shoot"

					shoot.Spec.DNS.Domain = &baseDomain

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("the domain is already used by another shoot or it is a subdomain of an already used domain")))
				})

				It("should allow to delete the shoot although the specified domain is a subdomain of a domain already used by another shoot (case two)", func() {
					anotherShoot := shoot.DeepCopy()
					anotherShoot.Name = "another-shoot"

					shoot.Spec.DNS.Domain = &baseDomain

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, userInfo)
					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				})

				It("should allow to update the shoot with deletion confirmation annotation although the specified domain is a subdomain of a domain already used by another shoot (case two)", func() {
					anotherShoot := shoot.DeepCopy()
					anotherShoot.Name = "another-shoot"

					shoot.Spec.DNS.Domain = &baseDomain

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					oldShoot := shoot.DeepCopy()
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "confirmation.gardener.cloud/deletion", "true")
					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				})

				It("should allow because the specified domain is not a subdomain of a domain already used by another shoot", func() {
					anotherShoot := shoot.DeepCopy()
					anotherShoot.Name = "another-shoot"

					anotherDomain := fmt.Sprintf("someprefix%s", *anotherShoot.Spec.DNS.Domain)
					shoot.Spec.DNS.Domain = &anotherDomain

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeNil())
				})
			})

			Context("kubernetes version checks", func() {
				It("should reject due to an invalid kubernetes version", func() {
					shoot.Spec.Kubernetes.Version = "1.2.3"

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should default a major.minor kubernetes version to latest patch version", func() {
					shoot.Spec.Kubernetes.Version = "1.6"
					highestPatchVersion := core.ExpirableVersion{Version: "1.6.6"}
					cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, highestPatchVersion, core.ExpirableVersion{Version: "1.7.1"}, core.ExpirableVersion{Version: "1.7.2"})

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

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
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

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
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

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
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(Not(HaveOccurred()))
				})

				It("should reject: default only exactly matching minor kubernetes version", func() {
					shoot.Spec.Kubernetes.Version = "1.8"
					highestPatchVersion := core.ExpirableVersion{Version: "1.81.5"}
					cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, core.ExpirableVersion{Version: "1.81.0"}, highestPatchVersion)

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject to create a cluster with an expired kubernetes version", func() {
					deprecatedClassification := core.ClassificationDeprecated
					expiredKubernetesVersion := "1.24.1"
					validKubernetesVersion := "1.24.3"
					shoot.Spec.Kubernetes.Version = expiredKubernetesVersion
					cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, core.ExpirableVersion{Version: expiredKubernetesVersion, Classification: &deprecatedClassification, ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)}}, core.ExpirableVersion{Version: validKubernetesVersion})

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("spec.kubernetes.version: Unsupported value: %q", expiredKubernetesVersion)))
				})

				It("should allow to delete a cluster with an expired kubernetes version", func() {
					deprecatedClassification := core.ClassificationDeprecated
					expiredKubernetesVersion := "1.24.1"
					validKubernetesVersion := "1.24.3"
					shoot.Spec.Kubernetes.Version = expiredKubernetesVersion
					cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, core.ExpirableVersion{Version: expiredKubernetesVersion, Classification: &deprecatedClassification, ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)}}, core.ExpirableVersion{Version: validKubernetesVersion})

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

					attrs := admission.NewAttributesRecord(nil, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).ToNot(HaveOccurred())
				})

				It("should choose the default kubernetes version if only major.minor is given in a worker group", func() {
					shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: pointer.String("1.24")}
					highestPatchVersion := core.ExpirableVersion{Version: "1.24.5"}
					cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, core.ExpirableVersion{Version: "1.24.0"}, highestPatchVersion)

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
					Expect(*shoot.Spec.Provider.Workers[0].Kubernetes.Version).To(Equal(highestPatchVersion.Version))
				})

				It("should work to create a cluster without a worker group kubernetes version set", func() {
					shoot.Spec.Kubernetes.Version = "1.24.5"
					highestPatchVersion := core.ExpirableVersion{Version: "1.24.5"}
					cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, core.ExpirableVersion{Version: "1.24.0"}, highestPatchVersion)

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.Provider.Workers[0].Kubernetes).To(BeNil())
				})

				It("should work to create a cluster with a worker group kubernetes version set smaller than control plane version", func() {
					shoot.Spec.Kubernetes.Version = "1.24.5"
					shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: pointer.String("1.23.0")}
					cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, core.ExpirableVersion{Version: "1.23.0"}, core.ExpirableVersion{Version: "1.24.5"})

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.Provider.Workers[0].Kubernetes.Version).To(Equal(pointer.String("1.23.0")))
				})

				It("should work to create a cluster with a worker group kubernetes version set equal to control plane version", func() {
					shoot.Spec.Kubernetes.Version = "1.24.5"
					shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: pointer.String("1.24.5")}
					cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, core.ExpirableVersion{Version: "1.23.0"}, core.ExpirableVersion{Version: "1.24.5"})

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.Provider.Workers[0].Kubernetes.Version).To(Equal(pointer.String("1.24.5")))
				})

				It("should reject to create a cluster with an expired worker group kubernetes version", func() {
					deprecatedClassification := core.ClassificationDeprecated
					expiredKubernetesVersion := "1.24.1"
					validKubernetesVersion := "1.24.3"
					shoot.Spec.Kubernetes.Version = validKubernetesVersion
					shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: &expiredKubernetesVersion}
					cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, core.ExpirableVersion{Version: expiredKubernetesVersion, Classification: &deprecatedClassification, ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)}}, core.ExpirableVersion{Version: validKubernetesVersion})

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("spec.provider.workers[0].kubernetes.version: Unsupported value: %q", expiredKubernetesVersion)))
				})

				It("should allow to delete a cluster with an expired worker group kubernetes version", func() {
					deprecatedClassification := core.ClassificationDeprecated
					expiredKubernetesVersion := "1.24.1"
					validKubernetesVersion := "1.24.3"
					shoot.Spec.Kubernetes.Version = validKubernetesVersion
					shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: &expiredKubernetesVersion}
					cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, core.ExpirableVersion{Version: expiredKubernetesVersion, Classification: &deprecatedClassification, ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)}}, core.ExpirableVersion{Version: validKubernetesVersion})

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

					attrs := admission.NewAttributesRecord(nil, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("kubelet config checks", func() {
				var (
					worker          core.Worker
					resourceCPU1    = resource.MustParse("2")
					resourceCPU2    = resource.MustParse("3")
					resourceMemory1 = resource.MustParse("2Gi")
					resourceMemory2 = resource.MustParse("3Gi")
					kubeletConfig   *core.KubeletConfig
				)

				BeforeEach(func() {
					worker = core.Worker{
						Name: "worker-name-kc",
						Machine: core.Machine{
							Type: "machine-type-kc",
							Image: &core.ShootMachineImage{
								Name: validMachineImageName,
							},
							Architecture: pointer.String("amd64"),
						},
						Minimum: 1,
						Maximum: 1,
						Volume: &core.Volume{
							VolumeSize: "40Gi",
							Type:       &volumeType,
						},
						Zones: []string{"europe-a"},
						Kubernetes: &core.WorkerKubernetes{
							Kubelet: &core.KubeletConfig{
								KubeReserved: &core.KubeletConfigReserved{
									CPU:    &resourceCPU1,
									Memory: &resourceMemory1,
								},
								SystemReserved: &core.KubeletConfigReserved{
									CPU:    &resourceCPU1,
									Memory: &resourceMemory1,
								},
							},
						},
					}

					machineType := core.MachineType{
						Name:         "machine-type-kc",
						CPU:          resource.MustParse("5"),
						GPU:          resource.MustParse("0"),
						Memory:       resource.MustParse("5Gi"),
						Architecture: pointer.String("amd64"),
					}

					kubeletConfig = &core.KubeletConfig{
						KubeReserved:   &core.KubeletConfigReserved{},
						SystemReserved: &core.KubeletConfigReserved{},
					}

					cloudProfile.Spec.MachineTypes = append(cloudProfile.Spec.MachineTypes, machineType)

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				})

				It("should allow creation of Shoot if reserved resources are less than resource capacity", func() {
					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should allow creation of Shoot if both global and worker kubeletConfigs are nil", func() {
					worker.Kubernetes.Kubelet = nil
					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should not allow creation of Shoot if reserved CPU in the global kubeletConfig is more than CPU capacity and worker kubeletConfig is nil", func() {
					kubeletConfig.KubeReserved.CPU = &resourceCPU2
					kubeletConfig.SystemReserved.CPU = &resourceCPU2
					shoot.Spec.Kubernetes.Kubelet = kubeletConfig

					worker.Kubernetes.Kubelet = nil
					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err.Error()).To(ContainSubstring("total reserved CPU (kubeReserved + systemReserved) cannot be more than the Node's CPU capacity"))
				})

				It("should allow creation of Shoot if reserved CPU in the global kubeletConfig is more than CPU capacity but the worker kubeletConfig has lesser reserved CPU", func() {
					kubeletConfig.KubeReserved.CPU = &resourceCPU2
					kubeletConfig.SystemReserved.CPU = &resourceCPU2
					shoot.Spec.Kubernetes.Kubelet = kubeletConfig

					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should not allow creation of Shoot if reserved CPU in the global kubeletConfig is less than CPU capacity but the worker kubeletConfig has more reserved CPU", func() {
					kubeletConfig.KubeReserved.CPU = &resourceCPU1
					kubeletConfig.SystemReserved.CPU = &resourceCPU1
					shoot.Spec.Kubernetes.Kubelet = kubeletConfig

					worker.Kubernetes.Kubelet.KubeReserved.CPU = &resourceCPU2
					worker.Kubernetes.Kubelet.SystemReserved.CPU = &resourceCPU2
					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err.Error()).To(ContainSubstring("total reserved CPU (kubeReserved + systemReserved) cannot be more than the Node's CPU capacity"))
				})

				It("should not allow creation of Shoot if kubeReserved CPU is more than CPU capacity", func() {
					resource := resourceCPU2
					resource.Add(resourceCPU2)
					worker.Kubernetes.Kubelet.SystemReserved = nil
					worker.Kubernetes.Kubelet.KubeReserved.CPU = &resource

					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err.Error()).To(ContainSubstring("total reserved CPU (kubeReserved + systemReserved) cannot be more than the Node's CPU capacity"))
				})

				It("should not allow creation of Shoot if systemReserved CPU is more than CPU capacity", func() {
					resource := resourceCPU2
					resource.Add(resourceCPU2)
					worker.Kubernetes.Kubelet.KubeReserved = nil
					worker.Kubernetes.Kubelet.SystemReserved.CPU = &resource

					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err.Error()).To(ContainSubstring("total reserved CPU (kubeReserved + systemReserved) cannot be more than the Node's CPU capacity"))
				})

				It("should not allow creation of Shoot if sum of kubeReserved and systemReserved CPU is more than CPU capacity", func() {
					worker.Kubernetes.Kubelet.KubeReserved.CPU = &resourceCPU2
					worker.Kubernetes.Kubelet.SystemReserved.CPU = &resourceCPU2

					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err.Error()).To(ContainSubstring("total reserved CPU (kubeReserved + systemReserved) cannot be more than the Node's CPU capacity"))
				})

				It("should not allow creation of Shoot if reserved memory in the global kubeletConfig is more than memory capacity and worker kubeletConfig is nil", func() {
					kubeletConfig.KubeReserved.Memory = &resourceMemory2
					kubeletConfig.SystemReserved.Memory = &resourceMemory2
					shoot.Spec.Kubernetes.Kubelet = kubeletConfig

					worker.Kubernetes.Kubelet = nil
					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err.Error()).To(ContainSubstring("total reserved memory (kubeReserved + systemReserved) cannot be more than the Node's memory capacity"))
				})

				It("should allow creation of Shoot if reserved memory in the global kubeletConfig is more than memory capacity but the worker kubeletConfig have lesser reserved memory", func() {
					kubeletConfig.KubeReserved.Memory = &resourceMemory2
					kubeletConfig.SystemReserved.Memory = &resourceMemory2
					shoot.Spec.Kubernetes.Kubelet = kubeletConfig

					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should not allow creation of Shoot if reserved memory in the global kubeletConfig is less than memory capacity but the worker kubeletConfig has more reserved memory", func() {
					kubeletConfig.KubeReserved.Memory = &resourceMemory1
					kubeletConfig.SystemReserved.Memory = &resourceMemory1
					shoot.Spec.Kubernetes.Kubelet = kubeletConfig

					worker.Kubernetes.Kubelet.KubeReserved.Memory = &resourceMemory2
					worker.Kubernetes.Kubelet.SystemReserved.Memory = &resourceMemory2
					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err.Error()).To(ContainSubstring("total reserved memory (kubeReserved + systemReserved) cannot be more than the Node's memory capacity"))
				})

				It("should not allow creation of Shoot if kubeReserved memory is more than memory capacity", func() {
					resource := resourceMemory2
					resource.Add(resourceMemory2)
					worker.Kubernetes.Kubelet.SystemReserved = nil
					worker.Kubernetes.Kubelet.KubeReserved.Memory = &resource

					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err.Error()).To(ContainSubstring("total reserved memory (kubeReserved + systemReserved) cannot be more than the Node's memory capacity"))
				})

				It("should not allow creation of Shoot if systemReserved memory is more than memory capacity", func() {
					resource := resourceMemory2
					resource.Add(resourceMemory2)
					worker.Kubernetes.Kubelet.KubeReserved = nil
					worker.Kubernetes.Kubelet.SystemReserved.Memory = &resource

					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err.Error()).To(ContainSubstring("total reserved memory (kubeReserved + systemReserved) cannot be more than the Node's memory capacity"))
				})

				It("should not allow creation of Shoot if sum of kubeReserved and systemReserved memory is more than memory capacity", func() {
					worker.Kubernetes.Kubelet.KubeReserved.Memory = &resourceMemory2
					worker.Kubernetes.Kubelet.SystemReserved.Memory = &resourceMemory2

					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err.Error()).To(ContainSubstring("total reserved memory (kubeReserved + systemReserved) cannot be more than the Node's memory capacity"))
				})

				It("should not allow update of Shoot if reserved CPU is more than CPU capacity", func() {
					oldShoot := shoot.DeepCopy()
					worker.Kubernetes.Kubelet.KubeReserved.CPU = &resourceCPU2
					worker.Kubernetes.Kubelet.SystemReserved.CPU = &resourceCPU2

					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err.Error()).To(ContainSubstring("total reserved CPU (kubeReserved + systemReserved) cannot be more than the Node's CPU capacity"))
				})

				It("should not allow update of Shoot if reserved memory is more than memory capacity", func() {
					oldShoot := shoot.DeepCopy()
					worker.Kubernetes.Kubelet.KubeReserved.Memory = &resourceMemory2
					worker.Kubernetes.Kubelet.SystemReserved.Memory = &resourceMemory2

					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err.Error()).To(ContainSubstring("total reserved memory (kubeReserved + systemReserved) cannot be more than the Node's memory capacity"))
				})
			})

			Context("machine architecture check", func() {
				It("should reject due to invalid architecture", func() {
					shoot.Spec.Provider.Workers[0].Machine.Architecture = pointer.String("foo")
					shoot.Spec.Provider.Workers[0].Machine.Image.Version = "1.2.0"

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err).To(MatchError(
						ContainSubstring("shoots.core.gardener.cloud \"shoot\" is forbidden: [spec.provider.workers[0].machine.architecture: Unsupported value: \"foo\": supported values: \"amd64\", \"arm64\"]"),
					))
				})
			})

			Context("machine image checks", func() {
				var (
					classificationPreview = core.ClassificationPreview

					imageName1 = validMachineImageName
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
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: latestNonExpiredVersion,
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: nonExpiredVersion1,
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: nonExpiredVersion2,
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        expiringVersion,
										ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * 1000)},
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        expiredVersion,
										ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
									},
									Architectures: []string{"amd64", "arm64"},
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
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: latestNonExpiredVersion,
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: nonExpiredVersion1,
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: nonExpiredVersion2,
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        expiringVersion,
										ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * 1000)},
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        expiredVersion,
										ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
									},
									Architectures: []string{"amd64", "arm64"},
								},
							},
						},
					}
				)

				BeforeEach(func() {
					cloudProfile.Spec.MachineImages = cloudProfileMachineImages
				})

				Context("create Shoot", func() {
					BeforeEach(func() {
						shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, core.Worker{
							Name: "worker-name-1",
							Machine: core.Machine{
								Type: "machine-type-3",
								Image: &core.ShootMachineImage{
									Name: validMachineImageName,
								},
								Architecture: pointer.String("arm64"),
							},
							Minimum: 1,
							Maximum: 1,
							Volume: &core.Volume{
								VolumeSize: "40Gi",
								Type:       &volumeType,
							},
							Zones: []string{"europe-a"},
						})
					})

					It("should reject due to an invalid machine image", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    "not-supported",
							Version: "not-supported",
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
					})

					It("should reject due to an invalid machine image (version unset)", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name: "not-supported",
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

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
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
					})

					It("should reject due to a machine image that does not match the kubeletVersionConstraint constraint when the control plane K8s version does not match", func() {
						cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages,
							core.MachineImage{
								Name: "constraint-image-name",
								Versions: []core.MachineImageVersion{
									{
										ExpirableVersion: core.ExpirableVersion{
											Version: "1.2.3",
										},
										KubeletVersionConstraint: pointer.String("< 1.26"),
										Architectures:            []string{"amd64"},
									},
								},
							},
						)

						shoot.Spec.Kubernetes.Version = "1.26.0"
						shoot.Spec.Provider.Workers = []core.Worker{
							{
								Machine: core.Machine{
									Type: "machine-type-1",
									Image: &core.ShootMachineImage{
										Name:    "constraint-image-name",
										Version: "1.2.3",
									},
									Architecture: pointer.String("amd64"),
								},
							},
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
						Expect(err.Error()).To(ContainSubstring("does not support CPU architecture 'amd64', is expired or does not match kubelet version constraint"))
					})

					It("should reject due to a machine image that does not match the kubeletVersionConstraint when the worker K8s version does not match", func() {
						cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages,
							core.MachineImage{
								Name: "constraint-image-name",
								Versions: []core.MachineImageVersion{
									{
										ExpirableVersion: core.ExpirableVersion{
											Version: "1.2.3",
										},
										KubeletVersionConstraint: pointer.String(">= 1.26"),
										Architectures:            []string{"amd64"},
									},
								},
							},
						)

						shoot.Spec.Kubernetes.Version = "1.26.0"
						shoot.Spec.Provider.Workers = []core.Worker{
							{
								Machine: core.Machine{
									Type: "machine-type-1",
									Image: &core.ShootMachineImage{
										Name:    "constraint-image-name",
										Version: "1.2.3",
									},
									Architecture: pointer.String("amd64"),
								},
								Kubernetes: &core.WorkerKubernetes{
									Version: pointer.String("1.25.0"),
								},
							},
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
						Expect(err.Error()).To(ContainSubstring("does not support CPU architecture 'amd64', is expired or does not match kubelet version constraint"))
					})

					It("should default version to latest non-preview version as shoot does not specify one", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = nil
						shoot.Spec.Provider.Workers[1].Machine.Image = nil

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(&core.ShootMachineImage{
							Name:    imageName1,
							Version: latestNonExpiredVersion,
						}))
						Expect(shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(&core.ShootMachineImage{
							Name:    imageName1,
							Version: latestNonExpiredVersion,
						}))
					})

					It("should default version to latest non-preview version as shoot only specifies name", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name: imageName1,
						}
						shoot.Spec.Provider.Workers[1].Machine.Image = &core.ShootMachineImage{
							Name: imageName1,
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(&core.ShootMachineImage{
							Name:    imageName1,
							Version: latestNonExpiredVersion,
						}))
						Expect(shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(&core.ShootMachineImage{
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
									Type: "machine-type-1",
									Image: &core.ShootMachineImage{
										Name:    "cr-image-name",
										Version: "1.2.3",
									},
									Architecture: pointer.String("amd64"),
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
										Architectures: []string{"amd64"},
									},
								},
							})

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})

					It("should reject unsupported CRI", func() {
						shoot.Spec.Provider.Workers = append(
							shoot.Spec.Provider.Workers,
							core.Worker{
								CRI: &core.CRI{
									Name: "unsupported-cri",
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
									Architecture: pointer.String("amd64"),
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
													{
														Type: "supported-cr-1",
													},
													{
														Type: "supported-cr-2",
													},
												},
											},
										},
										Architectures: []string{"amd64"},
									},
								},
							})

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
						Expect(err.Error()).To(ContainSubstring("machine image 'cr-image-name@1.2.3' does not support CRI 'unsupported-cri', supported values: [containerd]"))
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
									Architecture: pointer.String("amd64"),
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
										Architectures: []string{"amd64"},
									},
								},
							})

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
						Expect(err.Error()).To(ContainSubstring("machine image 'cr-image-name@1.2.3' does not support container runtime 'unsupported-cr-1', supported values: [supported-cr-1 supported-cr-2"))
					})
				})

				Context("update Shoot", func() {
					BeforeEach(func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    imageName1,
							Version: nonExpiredVersion1,
						}
						shoot.Spec.Provider.Workers[0].Machine.Architecture = pointer.String("amd64")
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
										Architectures: []string{"amd64"},
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
						err := admissionHandler.Admit(ctx, attrs, nil)

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
										Architectures: []string{"amd64"},
									},
									{
										ExpirableVersion: core.ExpirableVersion{
											Version: "2.3.4",
										},
										Architectures: []string{"amd64"},
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
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(HaveOccurred())
					})

					It("should keep machine image of the old shoot (unset in new shoot)", func() {
						newShoot := shoot.DeepCopy()
						newShoot.Spec.Provider.Workers[0].Machine.Image = nil

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

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
						err := admissionHandler.Admit(ctx, attrs, nil)

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
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(newShoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(&core.ShootMachineImage{
							Name:    imageName1,
							Version: nonExpiredVersion2,
						}))
					})

					It("should default version of new worker pool to latest non-preview version", func() {
						newShoot := shoot.DeepCopy()
						newWorker := newShoot.Spec.Provider.Workers[0].DeepCopy()
						newWorker2 := newShoot.Spec.Provider.Workers[0].DeepCopy()
						newWorker.Name = "second-worker"
						newWorker2.Name = "third-worker"
						newWorker.Machine.Image = nil
						newWorker2.Machine.Image = nil
						newWorker2.Machine.Type = "machine-type-3"
						newWorker2.Machine.Architecture = pointer.String("arm64")
						newShoot.Spec.Provider.Workers = append(newShoot.Spec.Provider.Workers, *newWorker, *newWorker2)

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), newShoot.Namespace, newShoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(newShoot.Spec.Provider.Workers[0]).To(Equal(shoot.Spec.Provider.Workers[0]))
						Expect(newShoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(&core.ShootMachineImage{
							Name:    imageName1,
							Version: latestNonExpiredVersion,
						}))
						Expect(newShoot.Spec.Provider.Workers[2].Machine.Image).To(Equal(&core.ShootMachineImage{
							Name:    imageName1,
							Version: latestNonExpiredVersion,
						}))
					})

					It("should default version of new worker pool to latest non-preview version (version unset)", func() {
						newShoot := shoot.DeepCopy()
						newWorker := newShoot.Spec.Provider.Workers[0].DeepCopy()
						newWorker2 := newShoot.Spec.Provider.Workers[0].DeepCopy()
						newWorker.Name = "second-worker"
						newWorker2.Name = "third-worker"
						newWorker2.Machine.Type = "machine-type-3"
						newWorker2.Machine.Architecture = pointer.String("arm64")
						newWorker.Machine.Image = &core.ShootMachineImage{
							Name: imageName2,
						}
						newWorker2.Machine.Image = &core.ShootMachineImage{
							Name: imageName2,
						}
						newShoot.Spec.Provider.Workers = append(newShoot.Spec.Provider.Workers, *newWorker, *newWorker2)

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), newShoot.Namespace, newShoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(newShoot.Spec.Provider.Workers[0]).To(Equal(shoot.Spec.Provider.Workers[0]))
						Expect(newShoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(&core.ShootMachineImage{
							Name:    imageName2,
							Version: latestNonExpiredVersion,
						}))
						Expect(newShoot.Spec.Provider.Workers[2].Machine.Image).To(Equal(&core.ShootMachineImage{
							Name:    imageName2,
							Version: latestNonExpiredVersion,
						}))
					})

					It("should default version of worker pool to latest non-preview version when machine architecture is changed", func() {
						newShoot := shoot.DeepCopy()
						newShoot.Spec.Provider.Workers[0].Machine.Type = "machine-type-3"
						newShoot.Spec.Provider.Workers[0].Machine.Image = nil
						newShoot.Spec.Provider.Workers[0].Machine.Architecture = pointer.String("arm64")

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), newShoot.Namespace, newShoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(newShoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(&core.ShootMachineImage{
							Name:    imageName1,
							Version: nonExpiredVersion1,
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

						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), newShoot.Namespace, newShoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

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

						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), newShoot.Namespace, newShoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

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

						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), newShoot.Namespace, newShoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(newShoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(&core.ShootMachineImage{
							Name:    imageName2,
							Version: nonExpiredVersion2,
						}))
					})
				})

				Context("delete Shoot", func() {
					It("should allow even if a machine image has an expiration date in the past", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    imageName1,
							Version: expiredVersion,
						}

						Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

						attrs := admission.NewAttributesRecord(nil, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})
				})
			})

			Context("machine type checks", func() {
				It("should not reject due to an usable machine type", func() {
					shoot.Spec.Provider.Workers = []core.Worker{
						{
							Machine: core.Machine{
								Type:         "machine-type-1",
								Architecture: pointer.String("amd64"),
							},
						},
					}

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should reject due to a not usable machine type", func() {
					shoot.Spec.Provider.Workers = []core.Worker{
						{
							Machine: core.Machine{
								Type:         "machine-type-old",
								Architecture: pointer.String("amd64"),
							},
						},
					}

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject due to an invalid machine type", func() {
					shoot.Spec.Provider.Workers = []core.Worker{
						{
							Machine: core.Machine{
								Type:         "not-allowed",
								Architecture: pointer.String("amd64"),
							},
						},
					}

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})
			})

			Context("volume checks", func() {
				It("should reject due to an invalid volume type", func() {
					notAllowed := "not-allowed"
					shoot.Spec.Provider.Workers = []core.Worker{
						{
							Machine: core.Machine{
								Type:         "machine-type-1",
								Architecture: pointer.String("amd64"),
							},
							Volume: &core.Volume{
								Type: &notAllowed,
							},
						},
					}

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

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
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should reject due to wrong volume size (volume type constraint)", func() {
					boundaryVolSize := minVolSize
					boundaryVolSize.Add(resource.MustParse("-1"))

					boundaryVolSizeMachine := minVolSizeMachine
					boundaryVolSizeMachine.Add(resource.MustParse("-1"))

					shoot.Spec.Provider.Workers = []core.Worker{
						{
							Machine: core.Machine{
								Type:         "machine-type-1",
								Architecture: pointer.String("amd64"),
							},
							Volume: &core.Volume{
								Type:       &volumeType2,
								VolumeSize: boundaryVolSize.String(),
							},
						},
						{
							Machine: core.Machine{
								Type:         "machine-type-2",
								Architecture: pointer.String("amd64"),
							},
							Volume: &core.Volume{
								Type:       &volumeType,
								VolumeSize: boundaryVolSize.String(),
							},
						},
						{
							Machine: core.Machine{
								Type:         "machine-type-2",
								Architecture: pointer.String("amd64"),
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
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err.Error()).To(ContainSubstring("spec.provider.workers[0].volume.size"))
					Expect(err.Error()).To(ContainSubstring("spec.provider.workers[2].volume.size"))
				})
			})

			Context("RawExtension internal API usage checks", func() {
				BeforeEach(func() {
					shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{
						Raw: []byte(`{
						"kind": "InfrastructureConfig",
						"apiVersion": "azure.provider.extensions.gardener.cloud/__internal",
						"key": "value"
						}`),
					}

					shoot.Spec.Provider.ControlPlaneConfig = &runtime.RawExtension{
						Raw: []byte(`{
						"apiVersion": "aws.provider.extensions.gardener.cloud/__internal",
						"kind": "ControlPlaneConfig",
						"key": "value"
						}`),
					}

					shoot.Spec.Networking.ProviderConfig = &runtime.RawExtension{
						Raw: []byte(`{
						"apiVersion": "calico.networking.extensions.gardener.cloud/__internal",
						"kind": "NetworkConfig",
						"key": "value"
						}`)}

					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, core.Worker{
						Name: "worker-with-invalid-providerConfig",
						Machine: core.Machine{
							Type: "machine-type-1",
							Image: &core.ShootMachineImage{
								Name:    validMachineImageName,
								Version: "0.0.1",
								ProviderConfig: &runtime.RawExtension{
									Raw: []byte(`{
									"apiVersion": "memoryone-chost.os.extensions.gardener.cloud/__internal",
									"kind": "OperatingSystemConfiguration",
									"key": "value"
									}`)},
							},
							Architecture: pointer.String("amd64"),
						},
						CRI: &core.CRI{
							Name: core.CRINameContainerD,
							ContainerRuntimes: []core.ContainerRuntime{
								{
									Type: "test-cr",
									ProviderConfig: &runtime.RawExtension{
										Raw: []byte(`{
										"apiVersion": "some.api/__internal",
										"kind": "ContainerRuntimeConfig",
										"some-key": "some-value"
										}`)},
								},
							},
						},

						Minimum: 1,
						Maximum: 1,
						Volume: &core.Volume{
							VolumeSize: "40Gi",
							Type:       &volumeType,
						},
						Zones: []string{"europe-a"},
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte(`{
							"apiVersion": "aws.provider.extensions.gardener.cloud/__internal",
							"kind": "WorkerConfig",
							"key": "value"
							}`)},
					})
				})

				It("ensures new clusters cannot use the apiVersion 'internal'", func() {
					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err.Error()).To(ContainSubstring("spec.provider.infrastructureConfig: Invalid value: \"azure.provider.extensions.gardener.cloud/__internal, Kind=InfrastructureConfig\": must not use apiVersion 'internal'"))
					Expect(err.Error()).To(ContainSubstring("spec.provider.controlPlaneConfig: Invalid value: \"aws.provider.extensions.gardener.cloud/__internal, Kind=ControlPlaneConfig\": must not use apiVersion 'internal'"))
					Expect(err.Error()).To(ContainSubstring("spec.networking.providerConfig: Invalid value: \"calico.networking.extensions.gardener.cloud/__internal, Kind=NetworkConfig\": must not use apiVersion 'internal'"))
					Expect(err.Error()).To(ContainSubstring("spec.provider.workers[1].providerConfig: Invalid value: \"aws.provider.extensions.gardener.cloud/__internal, Kind=WorkerConfig\": must not use apiVersion 'internal'"))
					Expect(err.Error()).To(ContainSubstring("spec.provider.workers[1].machine.image.providerConfig: Invalid value: \"memoryone-chost.os.extensions.gardener.cloud/__internal, Kind=OperatingSystemConfiguration\": must not use apiVersion 'internal'"))
					Expect(err.Error()).To(ContainSubstring("spec.provider.workers[1].cri.containerRuntimes[0].providerConfig: Invalid value: \"some.api/__internal, Kind=ContainerRuntimeConfig\": must not use apiVersion 'internal'"))
				})

				// TODO (voelzmo): remove this test and the associated production code once we gave owners of existing Shoots a nice grace period to move away from 'internal' apiVersion
				It("ensures existing clusters can still use the apiVersion 'internal' for compatibility reasons", func() {
					oldShoot := shoot.DeepCopy()

					// update the Shoot spec to avoid early exit in the admission process
					shoot.Spec.Provider.Workers[0].Maximum = 1337

					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("admits new clusters using other apiVersion than 'internal'", func() {
					shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{
						Raw: []byte(`{
						"kind": "InfrastructureConfig",
						"apiVersion": "azure.provider.extensions.gardener.cloud/v1",
						"key": "value"
						}`),
					}

					shoot.Spec.Provider.ControlPlaneConfig = &runtime.RawExtension{
						Raw: []byte(`{
						"apiVersion": "aws.provider.extensions.gardener.cloud/v1alpha1",
						"kind": "ControlPlaneConfig",
						"key": "value"
						}`),
					}

					shoot.Spec.Networking.ProviderConfig = &runtime.RawExtension{
						Raw: []byte(`{
						"apiVersion": "calico.networking.extensions.gardener.cloud/v1alpha1",
						"kind": "NetworkConfig",
						"key": "value"
						}`)}

					shoot.Spec.Provider.Workers[1].Machine.Image = &core.ShootMachineImage{
						Name:    validMachineImageName,
						Version: "0.0.1",
						ProviderConfig: &runtime.RawExtension{Raw: []byte(`{
						"apiVersion": "memoryone-chost.os.extensions.gardener.cloud/v1alpha1",
						"kind": "OperatingSystemConfiguration",
						"key": "value"
						}`)},
					}

					shoot.Spec.Provider.Workers[1].ProviderConfig = &runtime.RawExtension{
						Raw: []byte(`{
						"apiVersion": "aws.provider.extensions.gardener.cloud/v1alpha1",
						"kind": "WorkerConfig",
						"key": "value"
						}`)}

					shoot.Spec.Provider.Workers[1].CRI = &core.CRI{
						Name: core.CRINameContainerD,
						ContainerRuntimes: []core.ContainerRuntime{
							{
								Type: "test-cr",
								ProviderConfig: &runtime.RawExtension{
									Raw: []byte(`{
									"apiVersion": "some.api/v1alpha1",
									"kind": "ContainerRuntimeConfig",
									"key": "value"
									}`)},
							},
						},
					}
					Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})
			})

			It("allows RawExtensions to contain arbitrary json blobs", func() {
				shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{
					Raw: []byte(`{
					"key": "value"
					}`),
				}

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})

			It("doesn't throw an error when the passed json is invalid", func() {
				shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{
					Raw: []byte(`{
					"key": invalid-value
					}`),
				}

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).ToNot(HaveOccurred())
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
				Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("control plane migration", func() {
			var (
				oldSeedName string
				oldSeed     *core.Seed
				oldShoot    *core.Shoot
			)
			BeforeEach(func() {
				oldSeedName = fmt.Sprintf("old-%s", seedName)
				oldSeed = seed.DeepCopy()
				oldSeed.Name = oldSeedName

				oldShoot = shoot.DeepCopy()
				oldShoot.Spec.SeedName = &oldSeedName

				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(oldSeed)).To(Succeed())
			})

			DescribeTable("Changing shoot spec during migration",
				func(newSeedName *string, lastOperationType core.LastOperationType, lastOperationState core.LastOperationState, matcher types.GomegaMatcher) {
					shoot.Status.LastOperation = &core.LastOperation{
						Type:  lastOperationType,
						State: lastOperationState,
					}

					oldShoot.Spec.SeedName = &seedName
					oldShoot.Spec.Kubernetes.Version = "1.6.3"

					shoot.Spec.Kubernetes.Version = "1.6.4"
					shoot.Status.SeedName = newSeedName

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(matcher)
				},
				Entry("should reject if migration has not started, but seed was previously changed", &oldSeedName, core.LastOperationTypeReconcile, core.LastOperationStateSucceeded, BeForbiddenError()),
				Entry("should reject if the shoot has lastOperation=Migrate:Processing", &oldSeedName, core.LastOperationTypeMigrate, core.LastOperationStateProcessing, BeForbiddenError()),
				Entry("should reject if the shoot has lastOperation=Migrate:Error", &oldSeedName, core.LastOperationTypeMigrate, core.LastOperationStateError, BeForbiddenError()),
				Entry("should reject if the shoot has lastOperation=Migrate:Succeeded", nil, core.LastOperationTypeMigrate, core.LastOperationStateSucceeded, BeForbiddenError()),
				Entry("should reject if the shoot has lastOperation=Restore:Pending", nil, core.LastOperationTypeRestore, core.LastOperationStatePending, BeForbiddenError()),
				Entry("should reject if the shoot has lastOperation=Restore:Processing", &seedName, core.LastOperationTypeRestore, core.LastOperationStateProcessing, BeForbiddenError()),
				Entry("should reject if the shoot has lastOperation=Restore:Error", &seedName, core.LastOperationTypeRestore, core.LastOperationStateError, BeForbiddenError()),
				Entry("should allow if the shoot has lastOperation=Restore:Succeeded", &seedName, core.LastOperationTypeRestore, core.LastOperationStateSucceeded, Not(HaveOccurred())),
			)

			It("should allow changes to shoot spec if nothing else has changed", func() {
				oldShoot.Spec.SeedName = &seedName
				shoot.Spec.Kubernetes.Version = "1.6.4"
				oldShoot.Spec.Kubernetes.Version = "1.6.3"

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("binding subresource", func() {
			var (
				oldShoot   core.Shoot
				newSeed    core.Seed
				shootState core.ShootState
			)
			BeforeEach(func() {
				oldShoot = *shootBase.DeepCopy()
				shoot = *shootBase.DeepCopy()
				seed = *seedBase.DeepCopy()
				newSeed = *seedBase.DeepCopy()
				newSeed.Name = "new-seed"
				shootState = *shootStateBase.DeepCopy()

				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&newSeed)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().ShootStates().Informer().GetStore().Add(&shootState)).To(Succeed())
			})

			Context("when binding is updated", func() {
				It("should allow update of binding when shoot.spec.seedName is nil", func() {
					oldShoot.Spec.SeedName = nil
					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should reject update of binding if the non-nil seedName is set to nil", func() {
					shoot.Spec.SeedName = nil
					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("spec.seedName cannot be set to nil"))
				})

				It("should allow update of binding when shoot.spec.seedName is not nil and SeedChange feature gate is enabled", func() {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()
					shoot.Spec.SeedName = pointer.String(newSeed.Name)

					shootState.Spec.Gardener = append(shootState.Spec.Gardener, core.GardenerResourceData{
						Labels: map[string]string{
							"name":       "kube-apiserver-etcd-encryption-key",
							"managed-by": "secrets-manager",
						},
					})

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should reject update of binding when shoot.spec.seedName is not nil and SeedChange feature gate is disabled", func() {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, false)()
					shoot.Spec.SeedName = pointer.String(newSeed.Name)

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("spec.seedName: Invalid value: %q: field is immutable", seedName))
				})

				It("should reject update of binding if target seed does not exist", func() {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()
					shoot.Spec.SeedName = pointer.String(newSeed.Name + " other")

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Internal error occurred: could not find referenced seed"))
				})

				It("should reject update of binding if spec other than .spec.seedName is changed", func() {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()
					shoot.Spec.SeedName = pointer.String(newSeed.Name)
					shoot.Spec.Hibernation = &core.Hibernation{Enabled: pointer.Bool(true)}

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("only spec.seedName can be changed using the binding subresource when the shoot is being rescheduled to a new seed"))
				})
			})

			Context("shootIsBeingScheduled", func() {
				It("should reject update of binding if target seed is marked for deletion", func() {
					oldShoot.Spec.SeedName = nil
					now := metav1.Now()
					seed.DeletionTimestamp = &now

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("cannot schedule shoot 'shoot' on seed 'seed' that is already marked for deletion"))
				})
			})

			Context("shootIsBeingRescheduled a.k.a Control-Plane migration", func() {
				BeforeEach(func() {
					shoot.Spec.SeedName = pointer.String(newSeedName)
				})

				It("should reject update of binding if target seed is marked for deletion", func() {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()
					now := metav1.Now()
					newSeed.DeletionTimestamp = &now

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("cannot schedule shoot '%s' on seed '%s' that is already marked for deletion", shoot.Name, newSeedName))
				})

				It("should reject update of binding, because target Seed doesn't have configuration for backup", func() {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

					newSeed.Spec.Backup = nil

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("backup is not configured for seed %q", newSeedName)))
				})

				It("should reject update of binding, because old Seed doesn't have configuration for backup", func() {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

					seed.Spec.Backup = nil

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("backup is not configured for old seed %q", seedName)))
				})

				It("should reject update of binding, because cloud provider for new Seed is not equal to cloud provider for old Seed", func() {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

					seed.Spec.Provider.Type = "gcp"
					newSeed.Spec.Provider.Type = "aws"

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err.Error()).To(ContainSubstring("cannot change seed because cloud provider for new seed (%s) is not equal to cloud provider for old seed (%s)", newSeed.Spec.Provider.Type, seed.Spec.Provider.Type))
				})

				It("should reject update of binding when etcd encryption key is missing", func() {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

					shootState.Spec.Gardener = nil

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err.Error()).To(ContainSubstring("cannot change seed because etcd encryption key not found in shoot state"))
				})
			})

			Context("taints and tolerations", func() {
				BeforeEach(func() {
					shoot.Spec.SeedName = pointer.String(newSeedName)
				})

				It("update of binding should succeed because the Seed specified in the binding does not have any taints", func() {
					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("update of binding should fail because the seed specified in the binding has non-tolerated taints", func() {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

					newSeed.Spec.Taints = []core.SeedTaint{{Key: core.SeedTaintProtected}}

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("forbidden to use a seed whose taints are not tolerated by the shoot"))
				})

				It("update of binding should fail because the new Seed specified in the binding has non-tolerated taints", func() {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.SeedChange, true)()

					shoot.Spec.SeedName = pointer.String(newSeedName)
					newSeed.Spec.Taints = []core.SeedTaint{{Key: core.SeedTaintProtected}}

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("forbidden to use a seed whose taints are not tolerated by the shoot"))
				})

				It("update of binding should pass because shoot tolerates all taints of the seed", func() {
					newSeed.Spec.Taints = []core.SeedTaint{{Key: "foo"}}
					shoot.Spec.Tolerations = []core.Toleration{{Key: "foo", Value: pointer.String("bar")}}
					oldShoot.Spec.Tolerations = []core.Toleration{{Key: "foo", Value: pointer.String("bar")}}

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("seed capacity", func() {
				var allocatableShoots resource.Quantity

				BeforeEach(func() {
					shoot.Spec.DNS = nil
					oldShoot = *shoot.DeepCopy()
					shoot.Spec.SeedName = pointer.String(newSeedName)
					allocatableShoots = *resource.NewQuantity(1, resource.DecimalSI)
				})

				It("update of binding should pass because seed allocatable capacity is not set", func() {
					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("update of binding should pass because seed allocatable capacity is not exhausted", func() {
					newSeed.Status.Allocatable = corev1.ResourceList{"shoots": allocatableShoots}

					otherShoot := shoot.DeepCopy()
					otherShoot.Name = "other-shoot-1"
					otherShoot.Spec.SeedName = pointer.String("other-seed")
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

					otherShoot = shoot.DeepCopy()
					otherShoot.Name = "other-shoot-2"
					otherShoot.Spec.SeedName = nil
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("update of binding should fail because seed allocatable capacity is exhausted", func() {
					newSeed.Status.Allocatable = corev1.ResourceList{"shoots": allocatableShoots}

					otherShoot := shoot.DeepCopy()
					otherShoot.Name = "other-shoot-1"
					otherShoot.Spec.SeedName = pointer.String(newSeedName)
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

					otherShoot = shoot.DeepCopy()
					otherShoot.Name = "other-shoot-2"
					otherShoot.Spec.SeedName = nil
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("already has the maximum number of shoots scheduled on it")))
				})

				It("update of binding should fail because seed allocatable capacity is over-exhausted", func() {
					newSeed.Status.Allocatable = corev1.ResourceList{"shoots": allocatableShoots}

					otherShoot := shoot.DeepCopy()
					otherShoot.Name = "other-shoot-1"
					otherShoot.Spec.SeedName = pointer.String(newSeedName)
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

					otherShoot = shoot.DeepCopy()
					otherShoot.Name = "other-shoot-2"
					otherShoot.Spec.SeedName = pointer.String(newSeedName)
					Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("already has the maximum number of shoots scheduled on it")))
				})
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
						gardenerutils.ConfirmationDeletion: "true",
					}

					Expect(shootStore.Add(&shoot)).NotTo(HaveOccurred())

					attrs := admission.NewAttributesRecord(nil, shootBase.DeepCopyObject(), core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

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
