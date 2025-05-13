// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"go.uber.org/mock/gomock"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	securityinformers "github.com/gardener/gardener/pkg/client/security/informers/externalversions"
	"github.com/gardener/gardener/pkg/controllerutils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/plugin/pkg/shoot/validator"
	mockauthorizer "github.com/gardener/gardener/third_party/mock/apiserver/authorization/authorizer"
)

var _ = Describe("validator", func() {
	Describe("#Admit", func() {
		var (
			ctx                     context.Context
			admissionHandler        *ValidateShoot
			ctrl                    *gomock.Controller
			auth                    *mockauthorizer.MockAuthorizer
			kubeInformerFactory     kubeinformers.SharedInformerFactory
			coreInformerFactory     gardencoreinformers.SharedInformerFactory
			securityInformerFactory securityinformers.SharedInformerFactory
			cloudProfile            gardencorev1beta1.CloudProfile
			namespacedCloudProfile  gardencorev1beta1.NamespacedCloudProfile
			seed                    gardencorev1beta1.Seed
			secretBinding           gardencorev1beta1.SecretBinding
			credentialsBinding      securityv1alpha1.CredentialsBinding
			project                 gardencorev1beta1.Project
			shoot                   core.Shoot
			versionedShoot          gardencorev1beta1.Shoot

			userInfo            = &user.DefaultInfo{Name: "foo"}
			authorizeAttributes authorizer.AttributesRecord

			podsCIDR     = "100.96.0.0/11"
			servicesCIDR = "100.64.0.0/13"
			nodesCIDR    = "10.250.0.0/16"

			seedName      = "seed"
			namespaceName = "garden-my-project"
			projectName   = "my-project"
			newSeedName   = "new-seed"
			profileName   = "namespaced-profile"

			unmanagedDNSProvider = core.DNSUnmanaged
			baseDomain           = "example.com"

			validMachineImageName     = "some-machine-image"
			validMachineImageVersions = []gardencorev1beta1.MachineImageVersion{
				{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version: "0.0.1",
					},
					CRI: []gardencorev1beta1.CRI{
						{
							Name: gardencorev1beta1.CRINameContainerD,
							ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{
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

			projectBase = gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
				Spec: gardencorev1beta1.ProjectSpec{
					Namespace: &namespaceName,
				},
			}
			cloudProfileBase = gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "profile",
				},
				Spec: gardencorev1beta1.CloudProfileSpec{
					Type: "unknown",
					Kubernetes: gardencorev1beta1.KubernetesSettings{
						Versions: []gardencorev1beta1.ExpirableVersion{{Version: "1.6.4"}},
					},
					MachineImages: []gardencorev1beta1.MachineImage{
						{
							Name:     validMachineImageName,
							Versions: validMachineImageVersions,
						},
					},
					MachineTypes: []gardencorev1beta1.MachineType{
						{
							Name:         "machine-type-1",
							CPU:          resource.MustParse("2"),
							GPU:          resource.MustParse("0"),
							Memory:       resource.MustParse("100Gi"),
							Architecture: ptr.To("amd64"),
							Usable:       ptr.To(true),
						},
						{
							Name:         "machine-type-old",
							CPU:          resource.MustParse("2"),
							GPU:          resource.MustParse("0"),
							Memory:       resource.MustParse("100Gi"),
							Usable:       ptr.To(false),
							Architecture: ptr.To("amd64"),
						},
						{
							Name:   "machine-type-2",
							CPU:    resource.MustParse("2"),
							GPU:    resource.MustParse("0"),
							Memory: resource.MustParse("100Gi"),
							Storage: &gardencorev1beta1.MachineTypeStorage{
								Type:    volumeType,
								MinSize: &minVolSizeMachine,
							},
							Architecture: ptr.To("amd64"),
							Usable:       ptr.To(true),
						},
						{
							Name:         "machine-type-3",
							CPU:          resource.MustParse("2"),
							GPU:          resource.MustParse("0"),
							Memory:       resource.MustParse("100Gi"),
							Architecture: ptr.To("arm64"),
							Usable:       ptr.To(true),
						},
					},
					VolumeTypes: []gardencorev1beta1.VolumeType{
						{
							Name:   volumeType,
							Class:  "super-premium",
							Usable: ptr.To(true),
						},
						{
							Name:    volumeType2,
							Class:   "super-premium",
							MinSize: &minVolSize,
							Usable:  ptr.To(true),
						},
					},
					Regions: []gardencorev1beta1.Region{
						{
							Name:  "europe",
							Zones: []gardencorev1beta1.AvailabilityZone{{Name: "europe-a"}},
						},
						{
							Name:  "asia",
							Zones: []gardencorev1beta1.AvailabilityZone{{Name: "asia-a"}},
						},
					},
				},
			}
			namespacedCloudProfileBase = gardencorev1beta1.NamespacedCloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      profileName,
					Namespace: namespaceName,
				},
				Spec: gardencorev1beta1.NamespacedCloudProfileSpec{Parent: gardencorev1beta1.CloudProfileReference{
					Kind: "CloudProfile",
					Name: cloudProfileBase.Name,
				}},
				Status: gardencorev1beta1.NamespacedCloudProfileStatus{
					CloudProfileSpec: cloudProfileBase.Spec,
				},
			}
			seedBase = gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
				Spec: gardencorev1beta1.SeedSpec{
					Backup: &gardencorev1beta1.SeedBackup{},
					Networks: gardencorev1beta1.SeedNetworks{
						Pods:       seedPodsCIDR,
						Services:   seedServicesCIDR,
						Nodes:      &seedNodesCIDR,
						IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
					},
				},
			}
			secretBindingBase = gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: namespaceName,
				},
				Provider: &gardencorev1beta1.SecretBindingProvider{
					Type: "unknown",
				},
			}
			credentialsBindingBase = securityv1alpha1.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret",
					Namespace: namespaceName,
				},
				Provider: securityv1alpha1.CredentialsBindingProvider{
					Type: "unknown",
				},
			}
			shootBase = core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: namespaceName,
				},
				Spec: core.ShootSpec{
					CloudProfileName:       ptr.To("profile"),
					Region:                 "europe",
					SecretBindingName:      ptr.To("my-secret"),
					CredentialsBindingName: ptr.To("my-secret"),
					SeedName:               &seedName,
					DNS: &core.DNS{
						Domain: ptr.To("shoot." + baseDomain),
						Providers: []core.DNSProvider{
							{
								Type: &unmanagedDNSProvider,
							},
						},
					},
					Kubernetes: core.Kubernetes{
						Version: "1.6.4",
						KubeControllerManager: &core.KubeControllerManagerConfig{
							NodeMonitorGracePeriod: &metav1.Duration{Duration: 40 * time.Second},
						},
					},
					Networking: &core.Networking{
						Nodes:    &nodesCIDR,
						Pods:     &podsCIDR,
						Services: &servicesCIDR,
						IPFamilies: []core.IPFamily{
							core.IPFamilyIPv4,
						},
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
									Architecture: ptr.To("amd64"),
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

			secret = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-1",
					Namespace: namespaceName,
				},
			}
		)

		BeforeEach(func() {
			ctx = context.Background()
			ctrl = gomock.NewController(GinkgoT())
			auth = nil

			project = projectBase
			cloudProfile = *cloudProfileBase.DeepCopy()
			namespacedCloudProfile = *namespacedCloudProfileBase.DeepCopy()
			seed = seedBase
			secretBinding = secretBindingBase
			credentialsBinding = credentialsBindingBase
			shoot = *shootBase.DeepCopy()

			admissionHandler, _ = New()
			admissionHandler.SetAuthorizer(auth)
			admissionHandler.AssignReadyFunc(func() bool { return true })
			kubeInformerFactory = kubeinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetKubeInformerFactory(kubeInformerFactory)
			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetCoreInformerFactory(coreInformerFactory)
			securityInformerFactory = securityinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetSecurityInformerFactory(securityInformerFactory)

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

			versionedShoot = gardencorev1beta1.Shoot{}
			err := gardencorev1beta1.Convert_core_Shoot_To_v1beta1_Shoot(&shoot, &versionedShoot, nil)
			Expect(err).NotTo(HaveOccurred())
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
				project.DeletionTimestamp = &deletionTimestamp

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

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

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

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

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

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

		Context("shoot creation", func() {
			BeforeEach(func() {
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
			})

			Context("with generate name", func() {
				BeforeEach(func() {
					shoot.ObjectMeta = metav1.ObjectMeta{
						GenerateName: "demo-",
						Namespace:    namespaceName,
					}
				})

				It("should admit Shoot resources", func() {
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

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())

					authorizeAttributes.Name = shoot.Name

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeInvalidError())
					Expect(err.Error()).To(ContainSubstring("name must not exceed"))
				})
			})

			It("should add the created-by annotation", func() {
				Expect(shoot.Annotations).NotTo(HaveKeyWithValue(v1beta1constants.GardenCreatedBy, userInfo.Name))

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				Expect(admissionHandler.Admit(ctx, attrs, nil)).NotTo(HaveOccurred())

				Expect(shoot.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenCreatedBy, userInfo.Name))
			})
		})

		Context("hibernation checks", func() {
			var (
				oldShoot *core.Shoot
			)

			BeforeEach(func() {
				shoot = *shootBase.DeepCopy()
				oldShoot = shoot.DeepCopy()
				oldShoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(false)}

				shoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(true)}
			})

			DescribeTable("should allow/deny hibernating the Shoot according to HibernationPossible constraint",
				func(constraints []core.Condition, match types.GomegaMatcher) {
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

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
				operationFailed    = &core.LastOperation{State: core.LastOperationStateFailed}
				operationSucceeded = &core.LastOperation{State: core.LastOperationStateSucceeded}
			)
			BeforeEach(func() {
				shoot = *shootBase.DeepCopy()
				shoot.Spec.Maintenance = &core.Maintenance{}
				oldShoot = shoot.DeepCopy()

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
			})

			DescribeTable("confine spec roll-out checks",
				func(specChange, oldConfine, confine bool, oldOperation, operation *core.LastOperation, matcher types.GomegaMatcher) {
					oldShoot.Spec.Maintenance.ConfineSpecUpdateRollout = ptr.To(oldConfine)
					oldShoot.Status.LastOperation = oldOperation
					shoot.Spec.Maintenance.ConfineSpecUpdateRollout = ptr.To(confine)
					shoot.Status.LastOperation = operation
					if specChange {
						shoot.Spec.Kubernetes.KubeControllerManager = &core.KubeControllerManagerConfig{
							NodeMonitorGracePeriod: &metav1.Duration{Duration: 100 * time.Second},
						}
					}

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())

					Expect(shoot.Annotations).To(matcher)
				},
				Entry(
					"should add annotation for failed shoot",
					specUpdate, confineEnabled, confineEnabled, operationFailed, operationFailed,
					HaveKeyWithValue(v1beta1constants.FailedShootNeedsRetryOperation, "true"),
				),
				Entry(
					"should not add annotation for failed shoot because of missing spec change",
					!specUpdate, confineEnabled, confineEnabled, operationFailed, operationFailed,
					Not(HaveKey(v1beta1constants.FailedShootNeedsRetryOperation)),
				),
				Entry(
					"should not add annotation for succeeded shoot",
					specUpdate, confineEnabled, confineEnabled, operationFailed, operationSucceeded,
					Not(HaveKey(v1beta1constants.FailedShootNeedsRetryOperation)),
				),
				Entry(
					"should not add annotation for shoot w/o confine spec roll-out enabled",
					specUpdate, confineEnabled, !confineEnabled, operationFailed, operationFailed,
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

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
			})

			It("should reject creating a shoot on a seed which is marked for deletion", func() {
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
				shoot.Annotations[v1beta1constants.ConfirmationDeletion] = "true"

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
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
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
				oldShoot core.Shoot
				newSeed  gardencorev1beta1.Seed
			)

			BeforeEach(func() {
				oldShoot = *shootBase.DeepCopy()

				seed = *seedBase.DeepCopy()
				newSeed = *seedBase.DeepCopy()
				newSeed.Name = "new-seed"

				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&newSeed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
			})

			It("should not allow changing the seedName on admission.Update if the subresource is not binding", func() {
				shoot.Spec.SeedName = &newSeed.Name

				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err).To(MatchError(ContainSubstring("spec.seedName 'seed' cannot be changed to 'new-seed' by patching the shoot, please use the shoots/binding subresource")))
			})

			It("should not forbid changing the seedName on admission.Update if the subresource is binding", func() {
				shoot.Spec.SeedName = &newSeed.Name

				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should not allow setting the seedName to nil on admission.Update if the subresource is not binding", func() {
				shoot.Spec.SeedName = nil

				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err).To(MatchError(ContainSubstring("spec.seedName is already set to 'seed' and cannot be changed to 'nil'")))
			})

			It("should not allow setting the seedName to nil on admission.Update even if the subresource is binding", func() {
				shoot.Spec.SeedName = nil

				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err).To(MatchError(ContainSubstring("spec.seedName is already set to 'seed' and cannot be changed to 'nil'")))
			})

			It("should not allow setting seedName even if old seedName was nil on admission.Update if the subresource is not binding", func() {
				oldShoot.Spec.SeedName = nil

				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err).To(MatchError(ContainSubstring("spec.seedName 'nil' cannot be changed to 'seed' by patching the shoot, please use the shoots/binding subresource")))
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

			Context("CloudProfile reference and CloudProfileName", func() {
				BeforeEach(func() {
					shoot.Spec.SeedName = nil
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(&namespacedCloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

					shoot.Spec.CloudProfile = nil
					shoot.Spec.CloudProfileName = nil
				})

				It("should fail when both cloudProfileName and cloudProfile are provided for a new shoot", func() {
					shoot.Spec.CloudProfileName = ptr.To("profile")
					shoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "profile",
					}
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("either cloudProfileName or cloudProfile reference")))
				})

				It("should pass on update for a unchanged CloudProfile reference with CloudProfileName set accordingly", func() {
					shoot.Spec.CloudProfileName = ptr.To("profile")
					shoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "profile",
					}
					oldShoot := shoot.DeepCopy()
					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).ToNot(HaveOccurred())
				})

				It("should pass for a given CloudProfile by CloudProfileName", func() {
					shoot.Spec.CloudProfileName = ptr.To("profile")
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).ToNot(HaveOccurred())
				})

				It("should pass for a given CloudProfile by CloudProfile reference", func() {
					shoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "profile",
					}
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).ToNot(HaveOccurred())
				})

				It("should pass for a given NamespacedCloudProfile", func() {
					shoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "NamespacedCloudProfile",
						Name: profileName,
					}
					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).ToNot(HaveOccurred())
				})

				It("should pass validation on a change from a CloudProfile to a NamespacedCloudProfile", func() {
					oldShoot := shoot.DeepCopy()
					oldShoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "profile",
					}
					shoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "NamespacedCloudProfile",
						Name: profileName,
					}
					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).ToNot(HaveOccurred())
				})

				It("should pass validation on a change from a CloudProfileName to a NamespacedCloudProfile", func() {
					oldShoot := shoot.DeepCopy()
					oldShoot.Spec.CloudProfileName = ptr.To("profile")
					shoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "NamespacedCloudProfile",
						Name: profileName,
					}
					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).ToNot(HaveOccurred())
				})

				It("should fail validation on a change from a CloudProfile to a NamespacedCloudProfile with forbidden parent", func() {
					anotherCloudProfile := *cloudProfileBase.DeepCopy()
					anotherCloudProfile.Name = "another-root-profile"
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&anotherCloudProfile)).To(Succeed())

					anotherNamespacedCloudProfile := *namespacedCloudProfileBase.DeepCopy()
					anotherNamespacedCloudProfile.Name = "another-" + profileName
					anotherNamespacedCloudProfile.Spec.Parent.Name = "another-root-profile"
					Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(&anotherNamespacedCloudProfile)).To(Succeed())

					oldShoot := shoot.DeepCopy()
					oldShoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "profile",
					}
					shoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "NamespacedCloudProfile",
						Name: "another-namespaced-profile",
					}
					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("cannot change from \"profile\" to \"another-namespaced-profile\" (root: \"another-root-profile\")")))
				})

				It("should fail validation on a change from a CloudProfileName to a NamespacedCloudProfile with forbidden parent", func() {
					anotherCloudProfile := *cloudProfileBase.DeepCopy()
					anotherCloudProfile.Name = "another-root-profile"
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&anotherCloudProfile)).To(Succeed())

					anotherNamespacedCloudProfile := *namespacedCloudProfileBase.DeepCopy()
					anotherNamespacedCloudProfile.Name = "another-namespaced-profile"
					anotherNamespacedCloudProfile.Spec.Parent.Name = "another-root-profile"
					Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(&anotherNamespacedCloudProfile)).To(Succeed())

					oldShoot := shoot.DeepCopy()
					oldShoot.Spec.CloudProfileName = ptr.To("profile")
					shoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "NamespacedCloudProfile",
						Name: "another-namespaced-profile",
					}
					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("cannot change from \"profile\" to \"another-namespaced-profile\" (root: \"another-root-profile\")")))
				})

				It("should pass validation on a change from a NamespacedCloudProfile to a CloudProfile", func() {
					oldShoot := shoot.DeepCopy()
					oldShoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "NamespacedCloudProfile",
						Name: profileName,
					}
					shoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "profile",
					}
					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).ToNot(HaveOccurred())
				})

				It("should pass validation on a change from a NamespacedCloudProfile to another NamespacedCloudProfile with the same parent", func() {
					anotherNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
					anotherNamespacedCloudProfile.Name = profileName + "-1"
					Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(anotherNamespacedCloudProfile)).To(Succeed())

					oldShoot := shoot.DeepCopy()
					oldShoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "NamespacedCloudProfile",
						Name: profileName,
					}
					shoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "NamespacedCloudProfile",
						Name: profileName + "-1",
					}
					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).ToNot(HaveOccurred())
				})

				It("should fail validation on a change from a NamespacedCloudProfile to another NamespacedCloudProfile with different parents", func() {
					anotherNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
					anotherNamespacedCloudProfile.Name = "namespaced-profile-unrelated"
					anotherNamespacedCloudProfile.Spec.Parent.Name = "unrelated-profile"
					Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(anotherNamespacedCloudProfile)).To(Succeed())

					oldShoot := shoot.DeepCopy()
					oldShoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "NamespacedCloudProfile",
						Name: profileName,
					}
					shoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "NamespacedCloudProfile",
						Name: profileName + "-unrelated",
					}
					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("cannot change from \"namespaced-profile\" (root: \"profile\") to \"namespaced-profile-unrelated\" (root: \"unrelated-profile\")")))
				})

				It("should reject because the cloud profile changed to does not contain the Shoot's current machine type", func() {
					shoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "profile",
					}
					shoot.Spec.Provider.Workers = []core.Worker{
						{
							Name: "testing",
							Machine: core.Machine{
								Type: "a-special-machine-type",
							},
						},
					}
					oldShoot := shoot.DeepCopy()
					oldShoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "NamespacedCloudProfile",
						Name: profileName,
					}

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("newly referenced cloud profile does not contain the machine type \"a-special-machine-type\" currently in use by worker \"testing\"")))
				})

				It("should reject because the cloud profile changed to does not contain the Shoot's current volume type", func() {
					cloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{{Name: "a-special-machine-type"}}
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Update(&cloudProfile)).To(Succeed())

					shoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "profile",
					}
					shoot.Spec.Provider.Workers = []core.Worker{
						{
							Name:    "testing",
							Machine: core.Machine{Type: "a-special-machine-type"},
							Volume: &core.Volume{
								Type: ptr.To("a-special-volume-type"),
							},
						},
					}
					oldShoot := shoot.DeepCopy()
					oldShoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "NamespacedCloudProfile",
						Name: profileName,
					}

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("newly referenced cloud profile does not contain the volume type \"a-special-volume-type\" currently in use by worker \"testing\"")))
				})

				It("should reject because the cloud profile changed to does not contain the Shoot's current machine image version", func() {
					cloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{{Name: "a-special-machine-type"}}
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Update(&cloudProfile)).To(Succeed())

					namespacedCloudProfile.Status.CloudProfileSpec.MachineImages = []gardencorev1beta1.MachineImage{
						{
							Name: "gardenlinux",
							Versions: []gardencorev1beta1.MachineImageVersion{
								{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1592.1.0-dev"}},
							},
						},
					}
					Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Update(&namespacedCloudProfile)).To(Succeed())

					shoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "profile",
					}
					shoot.Spec.Provider.Workers = []core.Worker{
						{
							Name: "testing",
							Machine: core.Machine{
								Type: "a-special-machine-type",
								Image: &core.ShootMachineImage{
									Name:    "gardenlinux",
									Version: "1592.1.0-dev",
								},
							},
						},
					}
					oldShoot := shoot.DeepCopy()
					oldShoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "NamespacedCloudProfile",
						Name: profileName,
					}

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("newly referenced cloud profile does not contain the machine image version \"gardenlinux@1592.1.0-dev\" currently in use by worker \"testing\"")))
				})
			})

			It("should reject because the referenced seed was not found", func() {
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeInternalServerError())
			})

			It("should reject because the referenced project was not found", func() {
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeInternalServerError())
			})

			It("should reject because the cloud provider in shoot and profile differ", func() {
				cloudProfile.Spec.Type = "gcp"
				shoot.Spec.Provider.Type = "aws"

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("provider type in shoot must equal provider type of referenced CloudProfile: %q", cloudProfile.Spec.Type))
			})

			It("should reject because the cloud provider in shoot and secret binding differ", func() {
				secretBinding.Provider = &gardencorev1beta1.SecretBindingProvider{
					Type: "gcp",
				}
				shoot.Spec.Provider.Type = "aws"
				cloudProfile.Spec.Type = "aws"

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("provider type in shoot must match provider type of referenced SecretBinding: %q", secretBinding.Provider.Type))
			})

			It("should reject because the cloud provider in shoot and credentials binding differ", func() {
				secretBinding.Provider = &gardencorev1beta1.SecretBindingProvider{
					Type: "aws",
				}
				credentialsBinding.Provider = securityv1alpha1.CredentialsBindingProvider{
					Type: "gcp",
				}
				shoot.Spec.Provider.Type = "aws"
				cloudProfile.Spec.Type = "aws"

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("provider type in shoot must match provider type of referenced CredentialsBinding: %q", credentialsBinding.Provider.Type))
			})

			It("should reject migration to credentials binding because a different secret is referenced", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.SecretBindingName = nil
				oldShoot.Spec.CredentialsBindingName = nil

				secretBinding.SecretRef = corev1.SecretReference{
					Namespace: shoot.Namespace,
					Name:      "secret1",
				}
				credentialsBinding.CredentialsRef = corev1.ObjectReference{
					Namespace:  shoot.Namespace,
					Name:       "another-secret1",
					Kind:       "Secret",
					APIVersion: corev1.SchemeGroupVersion.String(),
				}
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("it is not allowed to change the referenced Secret when migrating from SecretBindingName to CredentialsBindingName"))
			})

			It("should reject migration to credentials binding because a workload identity is referenced", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.SecretBindingName = nil
				oldShoot.Spec.CredentialsBindingName = nil

				secretBinding.SecretRef = corev1.SecretReference{
					Namespace: shoot.Namespace,
					Name:      "secret1",
				}
				credentialsBinding.CredentialsRef = corev1.ObjectReference{
					Namespace:  shoot.Namespace,
					Name:       "secret1",
					Kind:       "WorkloadIdentity",
					APIVersion: securityv1alpha1.SchemeGroupVersion.String(),
				}
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("it is not allowed to change the referenced Secret when migrating from SecretBindingName to CredentialsBindingName"))
			})

			It("should allow migration to credentials binding because the referenced secret stays the same", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.SecretBindingName = nil
				oldShoot.Spec.CredentialsBindingName = nil

				secretBinding.SecretRef = corev1.SecretReference{
					Namespace: shoot.Namespace,
					Name:      "secret1",
				}
				credentialsBinding.CredentialsRef = corev1.ObjectReference{
					Namespace:  shoot.Namespace,
					Name:       "secret1",
					Kind:       "Secret",
					APIVersion: corev1.SchemeGroupVersion.String(),
				}
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
			})

			It("should not error if and secret and credentials binding are nil", func() {
				shoot.Spec.Provider.Type = "aws"
				shoot.Spec.SecretBindingName = nil
				cloudProfile.Spec.Type = "aws"
				shoot.Spec.CredentialsBindingName = nil

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should pass because no seed has to be specified (however can be). The scheduler sets the seed instead.", func() {
				shoot.Spec.SeedName = nil
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

		})

		Context("changing shoot.spec.credentialsBindingName", func() {
			var (
				oldShoot *core.Shoot
			)
			BeforeEach(func() {
				auth = mockauthorizer.NewMockAuthorizer(ctrl)

				shoot.Spec.SecretBindingName = nil
				oldShoot = shoot.DeepCopy()
				shoot.Spec.CredentialsBindingName = ptr.To("new-credentialsbinding")

				credentialsBinding.CredentialsRef = corev1.ObjectReference{
					Namespace:  shoot.Namespace,
					Name:       "secret1",
					Kind:       "Secret",
					APIVersion: corev1.SchemeGroupVersion.String(),
				}

				newCredentialsBinding := credentialsBinding.DeepCopy()
				newCredentialsBinding.Name = "new-credentialsbinding"
				newCredentialsBinding.CredentialsRef.Name = "new-secret"

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(newCredentialsBinding)).To(Succeed())
			})

			It("should deny the change if user has no permissions to read old credentials", func() {
				authorizeAttributes := authorizer.AttributesRecord{
					User:            userInfo,
					APIGroup:        "",
					APIVersion:      "v1",
					Resource:        "secrets",
					Namespace:       shoot.Namespace,
					Name:            "secret1",
					Verb:            "get",
					ResourceRequest: true,
				}

				auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionDeny, "", nil)

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)
				Expect(err).To(BeForbiddenError())
				Expect(err).To(MatchError(ContainSubstring("user %q is not allowed to read the previously referenced Secret %q", userInfo.Name, shoot.Namespace+"/secret1")))
			})

			It("should deny the change if user has no permissions to read new credentials", func() {
				oldAuthorizeAttributes := authorizer.AttributesRecord{
					User:            userInfo,
					APIGroup:        "",
					APIVersion:      "v1",
					Resource:        "secrets",
					Namespace:       shoot.Namespace,
					Name:            "secret1",
					Verb:            "get",
					ResourceRequest: true,
				}
				auth.EXPECT().Authorize(ctx, oldAuthorizeAttributes).Return(authorizer.DecisionAllow, "", nil)

				newAuthorizeAttributes := authorizer.AttributesRecord{
					User:            userInfo,
					APIGroup:        "",
					APIVersion:      "v1",
					Resource:        "secrets",
					Namespace:       shoot.Namespace,
					Name:            "new-secret",
					Verb:            "get",
					ResourceRequest: true,
				}
				auth.EXPECT().Authorize(ctx, newAuthorizeAttributes).Return(authorizer.DecisionDeny, "", nil)

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)
				Expect(err).To(BeForbiddenError())
				Expect(err).To(MatchError(ContainSubstring("user %q is not allowed to read the newly referenced Secret %q", userInfo.Name, shoot.Namespace+"/new-secret")))
			})

			It("should allow the change if user has permissions to read both old and new credentials", func() {
				oldAuthorizeAttributes := authorizer.AttributesRecord{
					User:            userInfo,
					APIGroup:        "",
					APIVersion:      "v1",
					Resource:        "secrets",
					Namespace:       shoot.Namespace,
					Name:            "secret1",
					Verb:            "get",
					ResourceRequest: true,
				}
				auth.EXPECT().Authorize(ctx, oldAuthorizeAttributes).Return(authorizer.DecisionAllow, "", nil)

				newAuthorizeAttributes := authorizer.AttributesRecord{
					User:            userInfo,
					APIGroup:        "",
					APIVersion:      "v1",
					Resource:        "secrets",
					Namespace:       shoot.Namespace,
					Name:            "new-secret",
					Verb:            "get",
					ResourceRequest: true,
				}
				auth.EXPECT().Authorize(ctx, newAuthorizeAttributes).Return(authorizer.DecisionAllow, "", nil)

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
			})
		})

		Context("tests deploy task", func() {
			var (
				oldShoot *core.Shoot
			)

			BeforeEach(func() {
				oldShoot = shootBase.DeepCopy()
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
			})

			It("should add deploy tasks because shoot is being created", func() {
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
					Enabled: ptr.To(true),
				}
				shoot.Spec.Hibernation = &core.Hibernation{
					Enabled: ptr.To(false),
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

			It("should add deploy infrastructure task because shoot operation annotation to rotate all credentials w/o workers rollout was set", func() {
				shoot.Annotations = make(map[string]string)
				shoot.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.OperationRotateCredentialsStartWithoutWorkersRollout

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
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
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
				cloudProfile.Spec.Regions[0].Zones = []gardencorev1beta1.AvailabilityZone{{Name: "not-available"}}

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
				cloudProfile.Spec.Regions[0].Zones = []gardencorev1beta1.AvailabilityZone{{Name: "zone-1"}, {Name: "zone-2"}}
				shoot.Spec.Provider.Workers[0].Zones = append(shoot.Spec.Provider.Workers[0].Zones, "zone-1")

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("Unsupported value: \"europe-a\": supported values: \"zone-1\", \"zone-2\""))
			})

			It("should reject due to an invalid zone", func() {
				shoot.Spec.Provider.Workers[0].Zones = []string{"invalid-zone"}

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
			})

			It("should reject due to a duplicate zone", func() {
				shoot.Spec.Provider.Workers[0].Zones = []string{"europe-a", "europe-a"}

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
			})

			It("should reject due to an invalid zone update", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Provider.Workers[0].Zones = append(shoot.Spec.Provider.Workers[0].Zones, oldShoot.Spec.Provider.Workers[0].Zones...)
				shoot.Spec.Provider.Workers[0].Zones = append(shoot.Spec.Provider.Workers[0].Zones, "invalid-zone")

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
			})

			It("should allow update when zone has removed from CloudProfile", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Provider.Workers[0].Zones = []string{}
				cloudProfile.Spec.Regions = cloudProfile.Spec.Regions[1:]

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})

			It("should reject creation because shoot access restrictions are not supported in this region", func() {
				shoot.Spec.SeedName = nil
				shoot.Spec.AccessRestrictions = []core.AccessRestrictionWithOptions{{AccessRestriction: core.AccessRestriction{Name: "foo"}}}

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.UpdateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("spec.accessRestrictions[0]: Unsupported value: \"foo\""))
			})

			It("should reject creation because shoot access restrictions are supported in this region, but not supported by the seed", func() {
				cloudProfile.Spec.Regions[0].AccessRestrictions = []gardencorev1beta1.AccessRestriction{{Name: "foo"}}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Update(&cloudProfile)).To(Succeed())

				shoot.Spec.AccessRestrictions = []core.AccessRestrictionWithOptions{{AccessRestriction: core.AccessRestriction{Name: "foo"}}}

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.UpdateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("forbidden to use a seed which doesn't support the access restrictions of the shoot"))
			})

			It("should allow creation because shoot access restrictions are supported in this region and by the seed", func() {
				cloudProfile.Spec.Regions[0].AccessRestrictions = []gardencorev1beta1.AccessRestriction{{Name: "foo"}}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Update(&cloudProfile)).To(Succeed())

				seed.Spec.AccessRestrictions = []gardencorev1beta1.AccessRestriction{{Name: "foo"}}
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Update(&seed)).To(Succeed())

				shoot.Spec.AccessRestrictions = []core.AccessRestrictionWithOptions{{AccessRestriction: core.AccessRestriction{Name: "foo"}}}

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.UpdateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject update because shoot access restrictions are not supported in this region", func() {
				shoot.Spec.AccessRestrictions = []core.AccessRestrictionWithOptions{{AccessRestriction: core.AccessRestriction{Name: "foo"}}}

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("spec.accessRestrictions[0]: Unsupported value: \"foo\""))
			})

			It("should reject update because shoot access restrictions are supported in this region, but not supported by the seed", func() {
				cloudProfile.Spec.Regions[0].AccessRestrictions = []gardencorev1beta1.AccessRestriction{{Name: "foo"}}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Update(&cloudProfile)).To(Succeed())

				shoot.Spec.AccessRestrictions = []core.AccessRestrictionWithOptions{{AccessRestriction: core.AccessRestriction{Name: "foo"}}}

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("spec.accessRestrictions[0]: Forbidden: access restriction \"foo\" is not supported by the seed"))
			})

			It("should allow update because shoot access restrictions are supported in this region and by the seed", func() {
				cloudProfile.Spec.Regions[0].AccessRestrictions = []gardencorev1beta1.AccessRestriction{{Name: "foo"}}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Update(&cloudProfile)).To(Succeed())

				seed.Spec.AccessRestrictions = []gardencorev1beta1.AccessRestriction{{Name: "foo"}}
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Update(&seed)).To(Succeed())

				shoot.Spec.AccessRestrictions = []core.AccessRestrictionWithOptions{{AccessRestriction: core.AccessRestriction{Name: "foo"}}}

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject seed migration because shoot access restrictions are not supported by the new seed", func() {
				cloudProfile.Spec.Regions[0].AccessRestrictions = []gardencorev1beta1.AccessRestriction{{Name: "foo"}}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Update(&cloudProfile)).To(Succeed())

				seed.Spec.AccessRestrictions = []gardencorev1beta1.AccessRestriction{{Name: "foo"}}
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Update(&seed)).To(Succeed())

				newSeed := seed.DeepCopy()
				newSeed.Name = newSeedName
				newSeed.Spec.AccessRestrictions = []gardencorev1beta1.AccessRestriction{{Name: "bar"}}
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(newSeed)).To(Succeed())

				shoot.Spec.AccessRestrictions = []core.AccessRestrictionWithOptions{{AccessRestriction: core.AccessRestriction{Name: "foo"}}}
				oldShoot.Spec.AccessRestrictions = shoot.Spec.AccessRestrictions
				shoot.Spec.SeedName = &newSeedName

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("forbidden to use a seed which doesn't support the access restrictions of the shoot"))
			})

			It("should allow removing access restrictions", func() {
				cloudProfile.Spec.Regions[0].AccessRestrictions = []gardencorev1beta1.AccessRestriction{{Name: "foo"}}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Update(&cloudProfile)).To(Succeed())

				shoot.Spec.AccessRestrictions = []core.AccessRestrictionWithOptions{{AccessRestriction: core.AccessRestriction{Name: "foo"}}}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&shoot)).To(Succeed())

				shoot.Spec.AccessRestrictions = nil

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
			})

			It("should allow deletion even if shoot access restrictions are (no longer) supported in this region", func() {
				seed.Spec.AccessRestrictions = []gardencorev1beta1.AccessRestriction{{Name: "foo"}}
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Update(&seed)).To(Succeed())

				shoot.Spec.AccessRestrictions = []core.AccessRestrictionWithOptions{{AccessRestriction: core.AccessRestriction{Name: "foo"}}}

				oldShoot = shoot.DeepCopy()
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.UpdateOptions{}, false, nil)
				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
			})

			It("should allow deletion even if shoot access restrictions are (no longer) supported by the seed", func() {
				cloudProfile.Spec.Regions[0].AccessRestrictions = []gardencorev1beta1.AccessRestriction{{Name: "foo"}}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Update(&cloudProfile)).To(Succeed())

				shoot.Spec.AccessRestrictions = []core.AccessRestrictionWithOptions{{AccessRestriction: core.AccessRestriction{Name: "foo"}}}

				oldShoot = shoot.DeepCopy()
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.UpdateOptions{}, false, nil)
				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
			})
		})

		Context("tests for unknown provider", func() {
			Context("scheduling checks for Create operation", func() {
				var (
					oldShoot *core.Shoot
				)

				BeforeEach(func() {
					oldShoot = shoot.DeepCopy()
					oldShoot.Spec.SeedName = nil

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
				})

				Context("taints and tolerations", func() {
					It("create should pass because the Seed specified in shoot manifest does not have any taints", func() {
						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).ToNot(HaveOccurred())
					})

					It("create should fail because the Seed specified in shoot manifest has non-tolerated taints", func() {
						seed.Spec.Taints = []gardencorev1beta1.SeedTaint{{Key: gardencorev1beta1.SeedTaintProtected}}

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
					})

					It("create should pass because shoot tolerates all taints of the seed", func() {
						seed.Spec.Taints = []gardencorev1beta1.SeedTaint{{Key: gardencorev1beta1.SeedTaintProtected}}
						versionedShoot.Spec.Tolerations = []gardencorev1beta1.Toleration{{Key: gardencorev1beta1.SeedTaintProtected}}
						shoot.Spec.Tolerations = []core.Toleration{{Key: core.SeedTaintProtected}}
						Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Update(&versionedShoot)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).ToNot(HaveOccurred())
					})

					It("delete should pass even if the Seed specified in shoot manifest has non-tolerated taints", func() {
						seed.Spec.Taints = []gardencorev1beta1.SeedTaint{{Key: gardencorev1beta1.SeedTaintProtected}}

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

						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
						Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
					})

					It("should pass because seed allocatable capacity is not set", func() {
						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})

					It("should pass because seed allocatable capacity is not exhausted", func() {
						seed.Status.Allocatable = corev1.ResourceList{"shoots": allocatableShoots}

						otherShoot := versionedShoot.DeepCopy()
						otherShoot.Name = "other-shoot-1"
						otherShoot.Spec.SeedName = ptr.To("other-seed")
						Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

						otherShoot = versionedShoot.DeepCopy()
						otherShoot.Name = "other-shoot-2"
						otherShoot.Spec.SeedName = nil
						Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})

					It("should reject because seed allocatable capacity is exhausted", func() {
						seed.Status.Allocatable = corev1.ResourceList{"shoots": allocatableShoots}

						otherShoot := versionedShoot.DeepCopy()
						otherShoot.Name = "other-shoot-1"
						otherShoot.Spec.SeedName = &seedName
						Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

						otherShoot = versionedShoot.DeepCopy()
						otherShoot.Name = "other-shoot-2"
						otherShoot.Spec.SeedName = nil
						Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(MatchError(ContainSubstring("already has the maximum number of shoots scheduled on it")))
					})

					It("should reject because seed allocatable capacity is over-exhausted", func() {
						seed.Status.Allocatable = corev1.ResourceList{"shoots": allocatableShoots}

						otherShoot := versionedShoot.DeepCopy()
						otherShoot.Name = "other-shoot-1"
						otherShoot.Spec.SeedName = &seedName
						Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

						otherShoot = versionedShoot.DeepCopy()
						otherShoot.Name = "other-shoot-2"
						otherShoot.Spec.SeedName = &seedName
						Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(MatchError(ContainSubstring("already has the maximum number of shoots scheduled on it")))
					})

					It("should allow Shoot deletion even though seed's allocatable capacity is exhausted / over exhausted", func() {
						seed.Status.Allocatable = corev1.ResourceList{"shoots": allocatableShoots}

						otherShoot := *versionedShoot.DeepCopy()
						otherShoot.Name = "other-shoot-1"
						otherShoot.Spec.SeedName = &seedName
						Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(&otherShoot)).To(Succeed())

						otherShoot = *versionedShoot.DeepCopy()
						otherShoot.Name = "other-shoot-2"
						otherShoot.Spec.SeedName = &seedName
						Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(&otherShoot)).To(Succeed())

						// admission for DELETION uses the old Shoot object
						oldShoot.Spec.SeedName = &seedName

						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).ToNot(HaveOccurred())
					})
				})

				Context("multi-zonal shoot scheduling checks on seed", func() {
					BeforeEach(func() {
						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
						Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
					})

					Context("seed has less than 3 zones", func() {
						BeforeEach(func() {
							seed.Spec.Provider.Zones = []string{"1", "2"}
						})

						It("should allow scheduling non-HA shoot", func() {
							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
						})

						It("should allow scheduling HA shoot with failure tolerance type 'node'", func() {
							shoot.Annotations = make(map[string]string)
							shoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeNode}}}

							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
						})

						It("should reject scheduling HA shoot with failure tolerance type 'zone'", func() {
							shoot.Annotations = make(map[string]string)
							shoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeZone}}}

							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							Expect(admissionHandler.Admit(ctx, attrs, nil)).To(BeForbiddenError())
						})
					})

					Context("seed has at least 3 zones", func() {
						BeforeEach(func() {
							seed.Spec.Provider.Zones = []string{"1", "2", "3"}
						})

						It("should allow scheduling non-HA shoot", func() {
							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
						})

						It("should allow scheduling HA shoot with failure tolerance type 'node'", func() {
							shoot.Annotations = make(map[string]string)
							shoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeNode}}}

							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
						})

						It("should allow scheduling HA shoot with failure tolerance type 'zone'", func() {
							shoot.Annotations = make(map[string]string)
							shoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeZone}}}

							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
						})
					})
				})

				Context("cloud profile's seed selector", func() {
					It("should reject shoot creation on seed when the cloud profile's seed selector is invalid", func() {
						cloudProfile.Spec.SeedSelector = &gardencorev1beta1.SeedSelector{
							LabelSelector: metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{Key: "domain", Operator: "invalid-operator", Values: []string{"foo"}},
								},
							},
						}

						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
						Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("label selector conversion failed"))
					})

					It("should allow shoot creation on seed that matches the cloud profile's seed selector", func() {
						cloudProfile.Spec.SeedSelector = &gardencorev1beta1.SeedSelector{
							LabelSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"domain": "foo"},
							},
						}
						seed.Labels = map[string]string{"domain": "foo"}

						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
						Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})

					It("should reject shoot creation on seed that does not match the cloud profile's seed selector", func() {
						cloudProfile.Spec.SeedSelector = &gardencorev1beta1.SeedSelector{
							LabelSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"domain": "foo"},
							},
						}
						seed.Labels = nil

						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
						Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("cannot schedule shoot '%s' on seed '%s' because the cloud profile seed selector is not matching the labels of the seed", shoot.Name, seed.Name))
					})

					It("should allow shoot creation on seed that matches one of the provider types in the cloud profile's seed selector", func() {
						cloudProfile.Spec.SeedSelector = &gardencorev1beta1.SeedSelector{
							ProviderTypes: []string{"foo", "bar", "baz"},
						}
						seed.Spec.Provider = gardencorev1beta1.SeedProvider{
							Type: "baz",
						}

						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
						Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})

					It("should reject shoot creation on seed that does not match any of the provider types in the cloud profile's seed selector", func() {
						cloudProfile.Spec.SeedSelector = &gardencorev1beta1.SeedSelector{
							ProviderTypes: []string{"foo", "bar"},
						}
						seed.Spec.Provider = gardencorev1beta1.SeedProvider{
							Type: "baz",
						}

						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
						Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("cannot schedule shoot '%s' on seed '%s' because none of the provider types in the cloud profile seed selector is matching the provider type of the seed", shoot.Name, seed.Name))
					})

					It("should allow updating the seedName to seed that matches the cloud profile's seed selector (w/ shoots/binding subresource)", func() {
						cloudProfile.Spec.SeedSelector = &gardencorev1beta1.SeedSelector{
							LabelSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"domain": "foo"},
							},
						}
						seed.Labels = map[string]string{"domain": "foo"}

						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
						Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})

					It("should reject updating the seedName to seed that does not match the cloud profile's seed selector (w/ shoots/binding subresource)", func() {
						cloudProfile.Spec.SeedSelector = &gardencorev1beta1.SeedSelector{
							LabelSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"domain": "foo"},
							},
						}
						seed.Labels = nil

						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
						Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("cannot schedule shoot '%s' on seed '%s' because the cloud profile seed selector is not matching the labels of the seed", shoot.Name, seed.Name))
					})

					It("should allow updating the seedName to seed that matches one of the provider types in the cloud profile's seed selector (w/ shoots/binding subresource)", func() {
						cloudProfile.Spec.SeedSelector = &gardencorev1beta1.SeedSelector{
							LabelSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"domain": "foo"},
							},
						}
						seed.Labels = map[string]string{"domain": "foo"}

						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
						Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})

					It("should reject updating the seedName to seed that does not match any of the provider types in the cloud profile's seed selector (w/ shoots/binding subresource)", func() {
						cloudProfile.Spec.SeedSelector = &gardencorev1beta1.SeedSelector{
							ProviderTypes: []string{"foo", "bar"},
						}
						seed.Spec.Provider = gardencorev1beta1.SeedProvider{
							Type: "baz",
						}

						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
						Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("cannot schedule shoot '%s' on seed '%s' because none of the provider types in the cloud profile seed selector is matching the provider type of the seed", shoot.Name, seed.Name))
					})
				})
			})

			Context("admission plugin check", func() {
				BeforeEach(func() {
					shoot.Spec.Kubernetes.KubeAPIServer = &core.KubeAPIServerConfig{}
				})

				Context("KubeconfigSecretName is not present for admission plugin", func() {
					It("should allow because KubeconfigSecretName is not present for admission plugin", func() {
						shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []core.AdmissionPlugin{
							{
								Name: "plugin-1",
							},
						}

						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
						Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})

					It("should not allow because kubeconfig secret is not referenced in shoot .spec.resources", func() {
						shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []core.AdmissionPlugin{
							{
								Name:                 "plugin-1",
								KubeconfigSecretName: ptr.To("secret-1"),
							},
						}

						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
						Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
					})

					It("should not allow because referenced kubeconfig secret does not exist", func() {
						shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []core.AdmissionPlugin{
							{
								Name:                 "plugin-1",
								KubeconfigSecretName: ptr.To("secret-1"),
							},
						}
						shoot.Spec.Resources = []core.NamedResourceReference{
							{
								Name: "ref-1",
								ResourceRef: autoscalingv1.CrossVersionObjectReference{
									Name:       "secret-1",
									APIVersion: "v1",
									Kind:       "Secret",
								},
							},
						}

						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
						Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
					})

					It("should not allow because referenced kubeconfig secret does not contain data kubeconfig", func() {
						shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []core.AdmissionPlugin{
							{
								Name:                 "plugin-1",
								KubeconfigSecretName: ptr.To("secret-1"),
							},
						}
						shoot.Spec.Resources = []core.NamedResourceReference{
							{
								Name: "ref-1",
								ResourceRef: autoscalingv1.CrossVersionObjectReference{
									Name:       "secret-1",
									APIVersion: "v1",
									Kind:       "Secret",
								},
							},
						}

						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
						Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
						Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
					})

					It("should allow because referenced kubeconfig secret does is valid and contain data kubeconfig", func() {
						shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []core.AdmissionPlugin{
							{
								Name:                 "plugin-1",
								KubeconfigSecretName: ptr.To("secret-1"),
							},
						}
						shoot.Spec.Resources = []core.NamedResourceReference{
							{
								Name: "ref-1",
								ResourceRef: autoscalingv1.CrossVersionObjectReference{
									Name:       "secret-1",
									APIVersion: "v1",
									Kind:       "Secret",
								},
							},
						}
						secret.Data = map[string][]byte{"kubeconfig": {}}

						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
						Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
						Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)).To(Succeed())

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})
				})
			})

			Context("oidc config check", func() {
				BeforeEach(func() {
					shoot.Spec.Kubernetes.KubeAPIServer = &core.KubeAPIServerConfig{}
				})

				DescribeTable("validate oidc config on shoot create", func(clientID, issuerURL *string, errorMatcher types.GomegaMatcher) {
					shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig = &core.OIDCConfig{
						ClientID:  clientID,
						IssuerURL: issuerURL,
					}

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(errorMatcher)
				},
					Entry("should allow when oidcConfig is valid", ptr.To("someClientID"), ptr.To("https://issuer.com"), BeNil()),
					Entry("should forbid when oidcConfig clientID is nil", nil, ptr.To("https://issuer.com"), BeForbiddenError()),
					Entry("should forbid when oidcConfig clientID is empty string", ptr.To(""), ptr.To("https://issuer.com"), BeForbiddenError()),
					Entry("should forbid when oidcConfig issuerURL is nil", ptr.To("someClientID"), nil, BeForbiddenError()),
					Entry("should forbid when oidcConfig issuerURL is empty string", ptr.To("someClientID"), ptr.To(""), BeForbiddenError()),
				)

				DescribeTable("do not validate oidc config when operation is not create", func(admissionOperation admission.Operation, operationOptions runtime.Object) {
					oldShoot := shoot.DeepCopy()
					shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig = &core.OIDCConfig{}

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admissionOperation, operationOptions, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).ToNot(HaveOccurred())
				},
					Entry("should allow invalid oidcConfig on shoot update", admission.Update, &metav1.UpdateOptions{}),
					Entry("should allow invalid oidcConfig on shoot delete", admission.Delete, &metav1.DeleteOptions{}),
				)
			})

			Context("networking settings checks", func() {
				var (
					oldShoot *core.Shoot
				)

				BeforeEach(func() {
					oldShoot = shoot.DeepCopy()
					oldShoot.Spec.SeedName = nil
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
				})

				It("update should pass because validation of network disjointedness should not be executed", func() {
					// set shoot pod cidr to overlap with vpn pod cidr
					shoot.Spec.Networking.Pods = ptr.To(v1beta1constants.DefaultVPNRangeV6)
					oldShoot.Spec.SeedName = shoot.Spec.SeedName

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).ToNot(HaveOccurred())
				})

				It("update should fail because validation of network disjointedness is executed", func() {
					// set shoot pod cidr to overlap with vpn pod cidr
					shoot.Spec.Networking.Pods = ptr.To(v1beta1constants.DefaultVPNRangeV6)

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("delete should pass because validation of network disjointedness should not be executed", func() {
					// set shoot pod cidr to overlap with vpn pod cidr
					shoot.Spec.Networking.Pods = ptr.To(v1beta1constants.DefaultVPNRangeV6)

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(nil, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).ToNot(HaveOccurred())
				})

				It("should reject because shoot pods network is missing", func() {
					shoot.Spec.Networking.Pods = nil

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should not reject because shoot pods network is nil (workerless Shoot)", func() {
					shoot.Spec.Provider.Workers = nil
					shoot.Spec.Networking.Pods = nil

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should reject because shoot services network is missing", func() {
					shoot.Spec.Networking.Services = nil

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err).To(MatchError(ContainSubstring("services is required, spec.networking.services")))
				})

				It("should reject because shoot services network is nil (workerless Shoot)", func() {
					shoot.Spec.Provider.Workers = nil
					shoot.Spec.Networking.Services = nil

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("services is required, spec.networking.services")))

				})

				It("should default shoot networks if seed provides ShootDefaults", func() {
					seed.Spec.Networks.ShootDefaults = &gardencorev1beta1.ShootNetworks{
						Pods:     &podsCIDR,
						Services: &servicesCIDR,
					}
					shoot.Spec.Networking.Pods = nil
					shoot.Spec.Networking.Services = nil

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.Networking.Pods).To(Equal(&podsCIDR))
					Expect(shoot.Spec.Networking.Services).To(Equal(&servicesCIDR))
				})

				It("should reject because the shoot node and the seed node networks intersect", func() {
					shoot.Spec.Networking.Nodes = &seedNodesCIDR

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the shoot pod and the seed pod networks intersect", func() {
					shoot.Spec.Networking.Pods = &seedPodsCIDR

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the shoot service and the seed service networks intersect", func() {
					shoot.Spec.Networking.Services = &seedServicesCIDR

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the shoot pod and the seed node networks intersect", func() {
					shoot.Spec.Networking.Pods = &seedNodesCIDR

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the shoot service and the seed node networks intersect", func() {
					shoot.Spec.Networking.Services = &seedNodesCIDR

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the shoot service and the shoot node networks intersect", func() {
					shoot.Spec.Networking.Services = shoot.Spec.Networking.Nodes

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the shoot pod and the shoot node networks intersect", func() {
					shoot.Spec.Networking.Pods = shoot.Spec.Networking.Nodes

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the shoot pod and the shoot service networks intersect", func() {
					shoot.Spec.Networking.Pods = shoot.Spec.Networking.Services

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})
			})

			Context("dns settings checks", func() {
				It("should reject because the specified domain is already used by another shoot", func() {
					anotherShoot := versionedShoot.DeepCopy()
					anotherShoot.Name = "another-shoot"

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
				})

				It("should reject because the specified domain is a subdomain of a domain already used by another shoot (case one)", func() {
					anotherShoot := versionedShoot.DeepCopy()
					anotherShoot.Name = "another-shoot"

					subdomain := "subdomain." + *anotherShoot.Spec.DNS.Domain
					shoot.Spec.DNS.Domain = &subdomain

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("the domain is already used by another shoot or it is a subdomain of an already used domain")))
				})

				It("should allow to delete the shoot although the specified domain is a subdomain of a domain already used by another shoot (case one)", func() {
					anotherShoot := versionedShoot.DeepCopy()
					anotherShoot.Name = "another-shoot"

					subdomain := fmt.Sprintf("subdomain.%s", *anotherShoot.Spec.DNS.Domain)
					shoot.Spec.DNS.Domain = &subdomain

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, userInfo)
					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				})

				It("should allow to update the shoot with deletion confirmation annotation although the specified domain is a subdomain of a domain already used by another shoot (case one)", func() {
					anotherShoot := versionedShoot.DeepCopy()
					anotherShoot.Name = "another-shoot"

					subdomain := fmt.Sprintf("subdomain.%s", *anotherShoot.Spec.DNS.Domain)
					shoot.Spec.DNS.Domain = &subdomain

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					oldShoot := shoot.DeepCopy()
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "confirmation.gardener.cloud/deletion", "true")
					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				})

				It("should reject because the specified domain is a subdomain of a domain already used by another shoot (case two)", func() {
					anotherShoot := versionedShoot.DeepCopy()
					anotherShoot.Name = "another-shoot"

					shoot.Spec.DNS.Domain = &baseDomain

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("the domain is already used by another shoot or it is a subdomain of an already used domain")))
				})

				It("should allow to delete the shoot although the specified domain is a subdomain of a domain already used by another shoot (case two)", func() {
					anotherShoot := versionedShoot.DeepCopy()
					anotherShoot.Name = "another-shoot"

					shoot.Spec.DNS.Domain = &baseDomain

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, userInfo)
					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				})

				It("should allow to update the shoot with deletion confirmation annotation although the specified domain is a subdomain of a domain already used by another shoot (case two)", func() {
					anotherShoot := versionedShoot.DeepCopy()
					anotherShoot.Name = "another-shoot"

					shoot.Spec.DNS.Domain = &baseDomain

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					oldShoot := shoot.DeepCopy()
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "confirmation.gardener.cloud/deletion", "true")
					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				})

				It("should allow because the specified domain is not a subdomain of a domain already used by another shoot", func() {
					anotherShoot := versionedShoot.DeepCopy()
					anotherShoot.Name = "another-shoot"

					anotherDomain := fmt.Sprintf("someprefix%s", *anotherShoot.Spec.DNS.Domain)
					shoot.Spec.DNS.Domain = &anotherDomain

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(anotherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("kubernetes version checks against CloudProfile", func() {
				var (
					highestSupportedVersion    gardencorev1beta1.ExpirableVersion
					highestSupported126Release gardencorev1beta1.ExpirableVersion
					highestPreviewVersion      gardencorev1beta1.ExpirableVersion
					expiredVersion             gardencorev1beta1.ExpirableVersion
				)
				BeforeEach(func() {
					preview := gardencorev1beta1.ClassificationPreview
					deprecatedClassification := gardencorev1beta1.ClassificationDeprecated

					highestPreviewVersion = gardencorev1beta1.ExpirableVersion{Version: "1.28.0", Classification: &preview}
					highestSupportedVersion = gardencorev1beta1.ExpirableVersion{Version: "1.27.3"}
					highestSupported126Release = gardencorev1beta1.ExpirableVersion{Version: "1.26.7"}
					expiredVersion = gardencorev1beta1.ExpirableVersion{Version: "1.26.8", Classification: &deprecatedClassification, ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)}}

					cloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{
						highestPreviewVersion,
						highestSupportedVersion,
						{Version: "1.27.2"},
						{Version: "1.26.6"},
						highestSupported126Release,
						expiredVersion,
						{Version: "1.25.11"},
						{Version: "1.24.12", Classification: &deprecatedClassification, ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)}},
					}

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
				})

				It("should reject due to an invalid kubernetes version", func() {
					shoot.Spec.Kubernetes.Version = "1.2.3"

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("Unsupported value: \"1.2.3\"")))
				})

				It("should throw an error because of an invalid major version", func() {
					shoot.Spec.Kubernetes.Version = "foo"

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("must be a semantic version")))
				})

				It("should throw an error because of an invalid minor version", func() {
					shoot.Spec.Kubernetes.Version = "1.bar"

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("must be a semantic version")))
				})

				It("should default a kubernetes version to latest major.minor.patch version", func() {
					shoot.Spec.Kubernetes.Version = ""

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(Not(HaveOccurred()))
					Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestSupportedVersion.Version))
				})

				It("should default a major kubernetes version to latest minor.patch version", func() {
					shoot.Spec.Kubernetes.Version = "1"

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(Not(HaveOccurred()))
					Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestSupportedVersion.Version))
				})

				It("should default a major.minor kubernetes version to latest patch version", func() {
					shoot.Spec.Kubernetes.Version = "1.26"

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(Not(HaveOccurred()))
					Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestSupported126Release.Version))
				})

				It("should reject defaulting a major.minor kubernetes version if there is no higher non-preview version available for defaulting", func() {
					shoot.Spec.Kubernetes.Version = "1.24"

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("couldn't find a suitable version for 1.24")))
				})

				It("should be able to explicitly pick preview versions", func() {
					shoot.Spec.Kubernetes.Version = highestPreviewVersion.Version

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(Not(HaveOccurred()))
				})

				It("should reject: default only exactly matching minor kubernetes version", func() {
					shoot.Spec.Kubernetes.Version = "1.2"

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("couldn't find a suitable version for 1.2")))
				})

				It("should reject to create a cluster with an expired kubernetes version", func() {
					shoot.Spec.Kubernetes.Version = expiredVersion.Version

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("spec.kubernetes.version: Unsupported value: %q", expiredVersion.Version)))
				})

				It("should allow updating a cluster to an expired kubernetes version", func() {
					oldShoot := shoot.DeepCopy()
					shoot.Spec.Kubernetes.Version = expiredVersion.Version

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).ToNot(HaveOccurred())
				})

				It("should allow to delete a cluster with an expired kubernetes version", func() {
					shoot.Spec.Kubernetes.Version = expiredVersion.Version

					attrs := admission.NewAttributesRecord(nil, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).ToNot(HaveOccurred())
				})

				It("should not choose the default kubernetes version if version is not specified", func() {
					shoot.Spec.Kubernetes.Version = "1.26"
					shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{}

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.Provider.Workers[0].Kubernetes.Version).To(BeNil())
				})

				It("should choose the default kubernetes version if only major.minor is given in a worker group", func() {
					shoot.Spec.Kubernetes.Version = "1.26"
					shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: ptr.To("1.26")}

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
					Expect(*shoot.Spec.Provider.Workers[0].Kubernetes.Version).To(Equal(highestSupported126Release.Version))
				})

				It("should work to create a cluster with a worker group kubernetes version set smaller than control plane version", func() {
					shoot.Spec.Kubernetes.Version = highestSupportedVersion.Version
					shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: ptr.To("1.26.6")}

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestSupportedVersion.Version))
					Expect(shoot.Spec.Provider.Workers[0].Kubernetes.Version).To(Equal(ptr.To("1.26.6")))
				})

				It("should work to create a cluster with a worker group kubernetes version set equal to control plane version", func() {
					shoot.Spec.Kubernetes.Version = highestSupportedVersion.Version
					shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: ptr.To(highestSupportedVersion.Version)}

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestSupportedVersion.Version))
					Expect(shoot.Spec.Provider.Workers[0].Kubernetes.Version).To(Equal(ptr.To(highestSupportedVersion.Version)))
				})

				It("should reject to create a cluster with an expired worker group kubernetes version", func() {
					shoot.Spec.Kubernetes.Version = highestSupportedVersion.Version
					shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: &expiredVersion.Version}

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("spec.provider.workers[0].kubernetes.version: Unsupported value: %q", expiredVersion.Version)))
				})

				It("should allow updating a cluster to an expired worker group kubernetes version", func() {
					oldShoot := shoot.DeepCopy()
					shoot.Spec.Kubernetes.Version = highestSupportedVersion.Version
					shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: &expiredVersion.Version}

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)

					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).ToNot(HaveOccurred())
				})

				It("should forbid updating a cluster with a new worker pool with an expired worker group kubernetes version", func() {
					oldShoot := shoot.DeepCopy()
					shoot.Spec.Kubernetes.Version = highestSupportedVersion.Version

					// add new worker pool with expired Kubernetes version
					newWorker := core.Worker{
						Name:       "worker-new",
						Kubernetes: &core.WorkerKubernetes{Version: &expiredVersion.Version},
						Machine: core.Machine{
							Type: "machine-type-1",
							Image: &core.ShootMachineImage{
								Name: validMachineImageName,
							},
							Architecture: ptr.To("amd64"),
						},
						Minimum: 1,
						Maximum: 1,
						Volume: &core.Volume{
							VolumeSize: "40Gi",
							Type:       &volumeType,
						},
						Zones: []string{"europe-a"},
					}

					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, newWorker)

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)

					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err).To(BeForbiddenError())
					Expect(err).To(MatchError(ContainSubstring("spec.provider.workers[1].kubernetes.version: Unsupported value: \"1.26.8\"")))
				})

				It("should allow to delete a cluster with an expired worker group kubernetes version", func() {
					shoot.Spec.Kubernetes.Version = highestSupportedVersion.Version
					shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: &expiredVersion.Version}

					attrs := admission.NewAttributesRecord(nil, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("kubernetes version checks against NamespacedCloudProfile", func() {
				var (
					expiredVersion gardencorev1beta1.ExpirableVersion
				)

				BeforeEach(func() {
					shoot.Spec.CloudProfileName = nil
					shoot.Spec.CloudProfile = &core.CloudProfileReference{
						Kind: "NamespacedCloudProfile",
						Name: namespacedCloudProfile.Name,
					}

					expiredVersion = gardencorev1beta1.ExpirableVersion{Version: "1.26.8", Classification: ptr.To(gardencorev1beta1.ClassificationDeprecated), ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)}}

					cloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{
						expiredVersion,
					}

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				})

				It("should not allow to create a cluster with an outdated extended Kubernetes version", func() {
					namespacedCloudProfile.Status.CloudProfileSpec.Kubernetes = (cloudProfile.DeepCopy()).Spec.Kubernetes
					Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(&namespacedCloudProfile)).To(Succeed())

					shoot.Spec.Kubernetes.Version = expiredVersion.Version

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(MatchError(And(ContainSubstring("Unsupported value"), ContainSubstring("1.26.8"))))
				})

				It("should allow to create a cluster with an extended Kubernetes version", func() {
					namespacedCloudProfile.Status.CloudProfileSpec.Kubernetes = (cloudProfile.DeepCopy()).Spec.Kubernetes
					namespacedCloudProfile.Status.CloudProfileSpec.Kubernetes.Versions[0].ExpirationDate = ptr.To(metav1.Time{Time: time.Now().Add(48 * time.Hour)})
					Expect(coreInformerFactory.Core().V1beta1().NamespacedCloudProfiles().Informer().GetStore().Add(&namespacedCloudProfile)).To(Succeed())

					shoot.Spec.Kubernetes.Version = expiredVersion.Version

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
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
							Architecture: ptr.To("amd64"),
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

					machineType := gardencorev1beta1.MachineType{
						Name:         "machine-type-kc",
						CPU:          resource.MustParse("5"),
						GPU:          resource.MustParse("0"),
						Memory:       resource.MustParse("5Gi"),
						Architecture: ptr.To("amd64"),
						Usable:       ptr.To(true),
					}

					kubeletConfig = &core.KubeletConfig{
						KubeReserved:   &core.KubeletConfigReserved{},
						SystemReserved: &core.KubeletConfigReserved{},
					}

					cloudProfile.Spec.MachineTypes = append(cloudProfile.Spec.MachineTypes, machineType)

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
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
				BeforeEach(func() {
					shoot.Spec.Provider.Workers[0].Machine.Image.Version = "1.2.0"
					shoot.Spec.Provider.Workers[0].Machine.Type = "machine-type-1"

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
				})

				It("should reject due to invalid architecture", func() {
					shoot.Spec.Provider.Workers[0].Machine.Architecture = ptr.To("foo")

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err).To(MatchError(
						ContainSubstring("shoots.core.gardener.cloud \"shoot\" is forbidden: spec.provider.workers[0].machine.architecture: Unsupported value: \"foo\": supported values: \"amd64\", \"arm64\""),
					))
				})

				It("should reject because the machine in the cloud provider doesn't support the architecture in the Shoot", func() {
					shoot.Spec.Provider.Workers[0].Machine.Architecture = ptr.To("arm64")
					shoot.Spec.Provider.Workers[0].Machine.Image.Version = "1.2.0"

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err).To(MatchError(
						ContainSubstring("machine type %q does not support CPU architecture %q, supported types are [%s]", shoot.Spec.Provider.Workers[0].Machine.Type, *shoot.Spec.Provider.Workers[0].Machine.Architecture, "machine-type-3"),
					))
				})
			})

			Context("machine image checks", func() {
				var (
					classificationPreview = gardencorev1beta1.ClassificationPreview

					imageName1 = validMachineImageName
					imageName2 = "other-image"

					expiredVersion          = "1.1.1"
					expiringVersion         = "1.2.1"
					nonExpiredVersion1      = "2.0.0"
					nonExpiredVersion2      = "2.0.1"
					latestNonExpiredVersion = "2.1.0"
					previewVersion          = "3.0.0"

					cloudProfileMachineImages []gardencorev1beta1.MachineImage
				)

				BeforeEach(func() {
					cloudProfileMachineImages = []gardencorev1beta1.MachineImage{
						{
							Name: validMachineImageName,
							Versions: []gardencorev1beta1.MachineImageVersion{
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version:        previewVersion,
										Classification: &classificationPreview,
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version: latestNonExpiredVersion,
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version: nonExpiredVersion1,
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version: nonExpiredVersion2,
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version:        expiringVersion,
										ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * 1000)},
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version:        expiredVersion,
										ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
									},
									Architectures: []string{"amd64", "arm64"},
								},
							},
						}, {
							Name: imageName2,
							Versions: []gardencorev1beta1.MachineImageVersion{
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version:        previewVersion,
										Classification: &classificationPreview,
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version: latestNonExpiredVersion,
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version: nonExpiredVersion1,
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version: nonExpiredVersion2,
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version:        expiringVersion,
										ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * 1000)},
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version:        expiredVersion,
										ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
									},
									Architectures: []string{"amd64", "arm64"},
								},
							},
						},
					}
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
								Architecture: ptr.To("arm64"),
							},
							Minimum: 1,
							Maximum: 1,
							Volume: &core.Volume{
								VolumeSize: "40Gi",
								Type:       &volumeType,
							},
							Zones: []string{"europe-a"},
						})

						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
						Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
					})

					It("should reject due to an invalid machine image (not present in cloudprofile)", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    "not-supported",
							Version: "not-supported",
						}

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
						Expect(err.Error()).To(ContainSubstring("image is not supported"))
					})

					It("should reject due to an invalid machine image (version unset)", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name: "not-supported",
						}

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("spec.provider.workers[0]: Invalid value: %q: image is not supported", "not-supported"))
					})

					It("should reject due to a machine image with expiration date in the past", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    validMachineImageName,
							Version: expiredVersion,
						}

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
						Expect(err).To(MatchError(ContainSubstring("machine image version 'some-machine-image:1.1.1' is expired")))
					})

					It("should reject due to a machine image version with non-supported architecture", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    validMachineImageName,
							Version: nonExpiredVersion1,
						}
						shoot.Spec.Provider.Workers[0].Machine.Architecture = ptr.To(v1beta1constants.ArchitectureAMD64)

						cloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
							{
								Name: validMachineImageName,
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: nonExpiredVersion1,
										},
										Architectures: []string{"arm64"},
									},
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: nonExpiredVersion2,
										},
										Architectures: []string{"amd64", "arm64"},
									},
								},
							},
						}

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
						Expect(err).To(MatchError(ContainSubstring("machine image version '%s' does not support CPU architecture %q, supported machine image versions are: [%s]", fmt.Sprintf("%s:%s", validMachineImageName, nonExpiredVersion1), "amd64", fmt.Sprintf("%s:%s", validMachineImageName, nonExpiredVersion2))))
					})

					It("should reject due to a machine image version with non-supported architecture and expired version", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    validMachineImageName,
							Version: expiredVersion,
						}
						shoot.Spec.Provider.Workers[0].Machine.Architecture = ptr.To(v1beta1constants.ArchitectureAMD64)

						cloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
							{
								Name: validMachineImageName,
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version:        expiredVersion,
											ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
										},
										Architectures: []string{"arm64"},
									},
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: nonExpiredVersion2,
										},
										Architectures: []string{"amd64", "arm64"},
									},
								},
							},
						}

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
						Expect(err).To(MatchError(ContainSubstring("machine image version '%s' does not support CPU architecture %q, is expired, supported machine image versions are: [%s]", fmt.Sprintf("%s:%s", validMachineImageName, expiredVersion), "amd64", fmt.Sprintf("%s:%s", validMachineImageName, nonExpiredVersion2))))
					})

					It("should reject due to a machine image version with no support for inplace updates when the workerpool update strategy is an in-place update strategy", func() {
						shoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    validMachineImageName,
							Version: latestNonExpiredVersion,
						}

						cloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
							{
								Name: validMachineImageName,
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: latestNonExpiredVersion,
										},
										Architectures: []string{"amd64", "arm64"},
										InPlaceUpdates: &gardencorev1beta1.InPlaceUpdates{
											Supported: false,
										},
									},
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: nonExpiredVersion1,
										},
										Architectures: []string{"amd64", "arm64"},
										InPlaceUpdates: &gardencorev1beta1.InPlaceUpdates{
											Supported: true,
										},
									},
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: nonExpiredVersion2,
										},
										Architectures: []string{"amd64", "arm64"},
									},
								},
							},
						}

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
						Expect(err).To(MatchError(ContainSubstring("machine image version '%s' does not support in-place updates, supported machine image versions are: [%s]", fmt.Sprintf("%s:%s", validMachineImageName, latestNonExpiredVersion), fmt.Sprintf("%s:%s", validMachineImageName, nonExpiredVersion1))))
					})

					It("should reject due to a machine image version with non-supported architecture, expired version and no support for inplace updates when the workerpool update strategy is an in-place update strategy", func() {
						shoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(core.ManualInPlaceUpdate)
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    validMachineImageName,
							Version: expiredVersion,
						}
						shoot.Spec.Provider.Workers[0].Machine.Architecture = ptr.To(v1beta1constants.ArchitectureAMD64)

						cloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
							{
								Name: validMachineImageName,
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version:        expiredVersion,
											ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
										},
										Architectures: []string{"arm64"},
									},
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: nonExpiredVersion1,
										},
										Architectures: []string{"amd64", "arm64"},
										InPlaceUpdates: &gardencorev1beta1.InPlaceUpdates{
											Supported: true,
										},
									},
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: nonExpiredVersion2,
										},
										Architectures: []string{"amd64", "arm64"},
										InPlaceUpdates: &gardencorev1beta1.InPlaceUpdates{
											Supported: true,
										},
									},
								},
							},
						}

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
						Expect(err).To(MatchError(ContainSubstring("machine image version '%s' does not support CPU architecture %q, is expired, does not support in-place updates, supported machine image versions are: [%s %s]", fmt.Sprintf("%s:%s", validMachineImageName, expiredVersion), "amd64", fmt.Sprintf("%s:%s", validMachineImageName, nonExpiredVersion1), fmt.Sprintf("%s:%s", validMachineImageName, nonExpiredVersion2))))
					})

					It("should reject due to a machine image that does not match the kubeletVersionConstraint when the control plane K8s version does not match", func() {
						cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, gardencorev1beta1.ExpirableVersion{Version: "1.26.0"})
						cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages,
							gardencorev1beta1.MachineImage{
								Name: "constraint-image-name",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: "1.2.3",
										},
										KubeletVersionConstraint: ptr.To("< 1.26"),
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
									Architecture: ptr.To("amd64"),
								},
							},
						}

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
						Expect(err.Error()).To(ContainSubstring("machine image 'constraint-image-name@1.2.3' does not support kubelet version '1.26.0', supported kubelet versions by this machine image version: '< 1.26'"))
					})

					It("should reject due to a machine image that does not match the kubeletVersionConstraint when the worker K8s version does not match", func() {
						cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, gardencorev1beta1.ExpirableVersion{Version: "1.25.0"}, gardencorev1beta1.ExpirableVersion{Version: "1.26.0"})
						cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages,
							gardencorev1beta1.MachineImage{
								Name: "constraint-image-name",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: "1.2.3",
										},
										KubeletVersionConstraint: ptr.To(">= 1.26"),
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
									Architecture: ptr.To("amd64"),
								},
								Kubernetes: &core.WorkerKubernetes{
									Version: ptr.To("1.25.0"),
								},
							},
						}

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
						Expect(err.Error()).To(ContainSubstring("machine image 'constraint-image-name@1.2.3' does not support kubelet version '1.25.0', supported kubelet versions by this machine image version: '>= 1.26'"))
					})

					It("should default version to latest non-preview version as shoot does not specify one", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = nil
						shoot.Spec.Provider.Workers[1].Machine.Image = nil

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

					Context("default machine image version", func() {
						var (
							suffixedVersion = "2.1.1-suffixed"
						)

						BeforeEach(func() {
							cloudProfile.Spec.MachineImages[0].Versions = append(cloudProfile.Spec.MachineImages[0].Versions,
								gardencorev1beta1.MachineImageVersion{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: suffixedVersion},
									Architectures:    []string{"amd64", "arm64"},
								},
							)
						})

						It("should throw an error because of an invalid major version", func() {
							shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
								Name:    imageName1,
								Version: "foo",
							}

							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							err := admissionHandler.Admit(ctx, attrs, nil)

							Expect(err).To(MatchError(ContainSubstring("must be a semantic version")))
						})

						It("should throw an error because of an invalid minor version", func() {
							shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
								Name:    imageName1,
								Version: "1.bar",
							}

							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							err := admissionHandler.Admit(ctx, attrs, nil)

							Expect(err).To(MatchError(ContainSubstring("must be a semantic version")))
						})

						It("should throw an error because of an invalid patch version", func() {
							shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
								Name:    imageName1,
								Version: "1.2.baz",
							}

							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							err := admissionHandler.Admit(ctx, attrs, nil)

							Expect(err).To(MatchError(ContainSubstring("machine image version is not supported")))
						})

						It("should default a machine image version to latest major.minor.patch version", func() {
							shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
								Name: imageName1,
							}

							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							err := admissionHandler.Admit(ctx, attrs, nil)

							Expect(err).To(Not(HaveOccurred()))
							Expect(shoot.Spec.Provider.Workers[0].Machine.Image.Version).To(Equal(suffixedVersion))
						})

						It("should default a major machine image version to latest minor.patch version", func() {
							shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
								Name:    imageName1,
								Version: "1",
							}

							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							err := admissionHandler.Admit(ctx, attrs, nil)

							Expect(err).To(Not(HaveOccurred()))
							Expect(shoot.Spec.Provider.Workers[0].Machine.Image.Version).To(Equal(expiringVersion))
						})

						It("should default a major.minor machine image version to latest patch version", func() {
							shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
								Name:    imageName1,
								Version: "2.0",
							}

							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							err := admissionHandler.Admit(ctx, attrs, nil)

							Expect(err).To(Not(HaveOccurred()))
							Expect(shoot.Spec.Provider.Workers[0].Machine.Image.Version).To(Equal(nonExpiredVersion2))
						})

						It("should reject defaulting a major.minor machine image version if there is no higher non-preview version available for defaulting", func() {
							shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
								Name:    imageName1,
								Version: "3",
							}

							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							err := admissionHandler.Admit(ctx, attrs, nil)

							Expect(err).To(MatchError(ContainSubstring("failed to determine latest machine image from cloud profile")))
						})

						It("should be able to explicitly pick preview versions", func() {
							shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
								Name:    imageName1,
								Version: "3.0.0",
							}

							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							err := admissionHandler.Admit(ctx, attrs, nil)

							Expect(err).To(Not(HaveOccurred()))
							Expect(shoot.Spec.Provider.Workers[0].Machine.Image.Version).To(Equal(previewVersion))
						})

						It("should reject: default only exactly matching minor machine image version", func() {
							shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
								Name:    imageName1,
								Version: "1.0",
							}

							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							err := admissionHandler.Admit(ctx, attrs, nil)

							Expect(err).To(MatchError(ContainSubstring("failed to determine latest machine image from cloud profile")))
						})

						It("should reject to create a worker group with an expired machine image version", func() {
							shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
								Name:    imageName1,
								Version: "1.1.1",
							}

							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							err := admissionHandler.Admit(ctx, attrs, nil)

							Expect(err).To(MatchError(ContainSubstring("machine image version 'some-machine-image:%s' is expired", expiredVersion)))
						})

						It("should reject defaulting a machine image version for worker pool with inplace update strategy if there is no machine image available in the cloud profile supporting inplace update", func() {
							shoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)

							attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							err := admissionHandler.Admit(ctx, attrs, nil)

							Expect(err).To(MatchError(ContainSubstring("failed to determine latest machine image from cloud profile")))

						})
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
									Architecture: ptr.To("amd64"),
								},
							},
						}

						cloudProfile.Spec.MachineImages = append(
							cloudProfile.Spec.MachineImages,
							gardencorev1beta1.MachineImage{
								Name: "cr-image-name",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: "1.2.3",
										},
										CRI: []gardencorev1beta1.CRI{
											{
												Name: gardencorev1beta1.CRINameContainerD,
												ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{
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
									Type: "machine-type-1",
									Image: &core.ShootMachineImage{
										Name:    "cr-image-name",
										Version: "1.2.3",
									},
									Architecture: ptr.To("amd64"),
								},
							})

						cloudProfile.Spec.MachineImages = append(
							cloudProfile.Spec.MachineImages,
							gardencorev1beta1.MachineImage{
								Name: "cr-image-name",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: "1.2.3",
										},
										CRI: []gardencorev1beta1.CRI{
											{
												Name: gardencorev1beta1.CRINameContainerD,
												ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{
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
									Type: "machine-type-1",
									Image: &core.ShootMachineImage{
										Name:    "cr-image-name",
										Version: "1.2.3",
									},
									Architecture: ptr.To("amd64"),
								},
							})

						cloudProfile.Spec.MachineImages = append(
							cloudProfile.Spec.MachineImages,
							gardencorev1beta1.MachineImage{
								Name: "cr-image-name",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: "1.2.3",
										},
										CRI: []gardencorev1beta1.CRI{
											{
												Name: gardencorev1beta1.CRINameContainerD,
												ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{
													{Type: "supported-cr-1"},
													{Type: "supported-cr-2"},
												},
											},
										},
										Architectures: []string{"amd64"},
									},
								},
							})

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
						shoot.Spec.Provider.Workers[0].Machine.Architecture = ptr.To("amd64")

						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
						Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
					})

					It("should deny updating to an MachineImage which does not support the selected container runtime", func() {
						cloudProfile.Spec.MachineImages = append(
							cloudProfile.Spec.MachineImages,
							gardencorev1beta1.MachineImage{
								Name: "cr-image-name",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: "1.2.3",
										},
										CRI: []gardencorev1beta1.CRI{
											{
												Name: gardencorev1beta1.CRINameContainerD,
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

						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(HaveOccurred())
					})

					It("should deny updating to an MachineImageVersion which does not support the selected container runtime", func() {
						cloudProfile.Spec.MachineImages = append(
							cloudProfile.Spec.MachineImages,
							gardencorev1beta1.MachineImage{
								Name: "cr-image-name",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: "1.2.3",
										},
										CRI: []gardencorev1beta1.CRI{
											{
												Name: gardencorev1beta1.CRINameContainerD,
											},
										},
										Architectures: []string{"amd64"},
									},
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
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

						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(HaveOccurred())
					})

					It("should reject due to a machine image that does not match the kubeletVersionConstraint when the control plane K8s version does not match", func() {
						cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, gardencorev1beta1.ExpirableVersion{Version: "1.25.0"}, gardencorev1beta1.ExpirableVersion{Version: "1.26.0"})
						cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages,
							gardencorev1beta1.MachineImage{
								Name: "constraint-image-name",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: "1.2.3",
										},
										KubeletVersionConstraint: ptr.To("< 1.26"),
										Architectures:            []string{"amd64"},
									},
								},
							},
						)

						shoot.Spec.Kubernetes.Version = "1.25.0"
						shoot.Spec.Provider.Workers = []core.Worker{
							{
								Machine: core.Machine{
									Type: "machine-type-1",
									Image: &core.ShootMachineImage{
										Name:    "constraint-image-name",
										Version: "1.2.3",
									},
									Architecture: ptr.To("amd64"),
								},
							},
						}
						newShoot := shoot.DeepCopy()
						newShoot.Spec.Kubernetes.Version = "1.26.0"

						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
						Expect(err.Error()).To(ContainSubstring("machine image 'constraint-image-name@1.2.3' does not support kubelet version '1.26.0', supported kubelet versions by this machine image version: '< 1.26'"))
					})

					It("should reject due to a machine image that does not match the kubeletVersionConstraint when the worker K8s version does not match", func() {
						cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, gardencorev1beta1.ExpirableVersion{Version: "1.24.0"}, gardencorev1beta1.ExpirableVersion{Version: "1.25.0"}, gardencorev1beta1.ExpirableVersion{Version: "1.26.0"})
						cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages,
							gardencorev1beta1.MachineImage{
								Name: "constraint-image-name",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: "1.2.3",
										},
										KubeletVersionConstraint: ptr.To(">= 1.26"),
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
									Architecture: ptr.To("amd64"),
								},
								Kubernetes: &core.WorkerKubernetes{
									Version: ptr.To("1.24.0"),
								},
							},
						}
						newShoot := shoot.DeepCopy()
						newShoot.Spec.Provider.Workers[0].Kubernetes.Version = ptr.To("1.25.0")

						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
						Expect(err.Error()).To(ContainSubstring("machine image 'constraint-image-name@1.2.3' does not support kubelet version '1.25.0', supported kubelet versions by this machine image version: '>= 1.26'"))
					})

					It("should forbid creating a new worker pool with an expired machine image version", func() {
						cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages,
							gardencorev1beta1.MachineImage{
								Name: "constraint-image-name",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version:        "1.2.4",
											ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
										},
										Architectures: []string{"amd64"},
									},
								},
							},
						)

						shoot.Spec.Provider.Workers = []core.Worker{
							{
								Machine: core.Machine{
									Type: "machine-type-1",
									Image: &core.ShootMachineImage{
										Name:    "constraint-image-name",
										Version: "1.2.4",
									},
									Architecture: ptr.To("amd64"),
								},
							},
						}

						newShoot := shoot.DeepCopy()

						newWorker := core.Worker{
							Name: "new-worker",
							Machine: core.Machine{
								Type: "machine-type-1",
								Image: &core.ShootMachineImage{
									Name: "constraint-image-name",
									// expired version
									Version: "1.2.4",
								},
								Architecture: ptr.To("amd64"),
							},
						}

						newShoot.Spec.Provider.Workers = append(newShoot.Spec.Provider.Workers, newWorker)

						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
						Expect(err.Error()).To(ContainSubstring("machine image version 'constraint-image-name:1.2.4' is expired"))
						Expect(err.Error()).To(ContainSubstring("spec.provider.workers[1].machine.image"))
					})

					It("should forbid updating an existing worker pool machine image to a lower expired version", func() {
						cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages,
							gardencorev1beta1.MachineImage{
								Name: "constraint-image-name",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: "1.2.4",
										},
										Architectures: []string{"amd64"},
									},
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version:        "1.2.3",
											ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
										},
										Architectures: []string{"amd64"},
									},
								},
							},
						)

						shoot.Spec.Provider.Workers = []core.Worker{
							{
								Machine: core.Machine{
									Type: "machine-type-1",
									Image: &core.ShootMachineImage{
										Name:    "constraint-image-name",
										Version: "1.2.4",
									},
									Architecture: ptr.To("amd64"),
								},
							},
						}

						newShoot := shoot.DeepCopy()

						newShoot.Spec.Provider.Workers = []core.Worker{
							{
								Machine: core.Machine{
									Type: "machine-type-1",
									Image: &core.ShootMachineImage{
										Name: "constraint-image-name",
										// updated to lower expired version
										Version: "1.2.3",
									},
									Architecture: ptr.To("amd64"),
								},
							},
						}

						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
						Expect(err.Error()).To(ContainSubstring("machine image version 'constraint-image-name:1.2.3' is expired"))
					})

					It("should allow updating to a higher expired machine image for an existing worker pool", func() {
						cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages,
							gardencorev1beta1.MachineImage{
								Name: "constraint-image-name",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version:        "1.2.4",
											ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
										},
										Architectures: []string{"amd64"},
									},
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: "1.2.3",
										},
										Architectures: []string{"amd64"},
									},
								},
							},
						)

						shoot.Spec.Provider.Workers = []core.Worker{
							{
								Machine: core.Machine{
									Type: "machine-type-1",
									Image: &core.ShootMachineImage{
										Name:    "constraint-image-name",
										Version: "1.2.3",
									},
									Architecture: ptr.To("amd64"),
								},
							},
						}

						newShoot := shoot.DeepCopy()

						newShoot.Spec.Provider.Workers = []core.Worker{
							{
								Machine: core.Machine{
									Type: "machine-type-1",
									Image: &core.ShootMachineImage{
										Name: "constraint-image-name",
										// updated to higher expired version
										Version: "1.2.4",
									},
									Architecture: ptr.To("amd64"),
								},
							},
						}

						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).ToNot(HaveOccurred())
					})

					It("should forbid updating to a higher machine image for an existing worker pool with in-place update strategy if the image does not support in-place update", func() {
						cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages,
							gardencorev1beta1.MachineImage{
								Name: "constraint-image-name",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version:        "1.2.5",
											ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * 1000)},
										},
										Architectures: []string{"amd64"},
										InPlaceUpdates: &gardencorev1beta1.InPlaceUpdates{
											Supported: false,
										},
									},
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version:        "1.2.4",
											ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * 1000)},
										},
										Architectures: []string{"amd64"},
										InPlaceUpdates: &gardencorev1beta1.InPlaceUpdates{
											Supported:           true,
											MinVersionForUpdate: ptr.To("1.2.3"),
										},
									},
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: "1.2.3",
										},
										Architectures: []string{"amd64"},
										InPlaceUpdates: &gardencorev1beta1.InPlaceUpdates{
											Supported: true,
										},
									},
								},
							},
						)

						shoot.Spec.Provider.Workers = []core.Worker{
							{
								Machine: core.Machine{
									Type: "machine-type-1",
									Image: &core.ShootMachineImage{
										Name:    "constraint-image-name",
										Version: "1.2.3",
									},
									Architecture: ptr.To("amd64"),
								},
								UpdateStrategy: ptr.To(core.AutoInPlaceUpdate),
							},
						}

						newShoot := shoot.DeepCopy()

						newShoot.Spec.Provider.Workers = []core.Worker{
							{
								Machine: core.Machine{
									Type: "machine-type-1",
									Image: &core.ShootMachineImage{
										Name: "constraint-image-name",
										// updated to higher non-expired version
										Version: "1.2.5",
									},
									Architecture: ptr.To("amd64"),
								},
								UpdateStrategy: ptr.To(core.AutoInPlaceUpdate),
							},
						}

						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
						Expect(err).To(MatchError(ContainSubstring("machine image version '%s' cannot be in-place updated from the current version, supported machine image versions are: [%s]", fmt.Sprintf("%s:%s", "constraint-image-name", "1.2.5"), fmt.Sprintf("%s:%s", "constraint-image-name", "1.2.4"))))
					})

					It("should forbid updating to a higher machine image for an existing worker pool with in-place update strategy if MinVersionForUpdate is higher than current version", func() {
						cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages,
							gardencorev1beta1.MachineImage{
								Name: "constraint-image-name",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version:        "1.2.5",
											ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * 1000)},
										},
										Architectures: []string{"amd64"},
										InPlaceUpdates: &gardencorev1beta1.InPlaceUpdates{
											Supported:           true,
											MinVersionForUpdate: ptr.To("1.2.4"),
										},
									},
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version:        "1.2.4",
											ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * 1000)},
										},
										Architectures: []string{"amd64"},
										InPlaceUpdates: &gardencorev1beta1.InPlaceUpdates{
											Supported:           true,
											MinVersionForUpdate: ptr.To("1.2.2"),
										},
									},
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: "1.2.3",
										},
										Architectures: []string{"amd64"},
										InPlaceUpdates: &gardencorev1beta1.InPlaceUpdates{
											Supported:           true,
											MinVersionForUpdate: ptr.To("1.2.2"),
										},
									},
								},
							},
						)

						shoot.Spec.Provider.Workers = []core.Worker{
							{
								Machine: core.Machine{
									Type: "machine-type-1",
									Image: &core.ShootMachineImage{
										Name:    "constraint-image-name",
										Version: "1.2.2",
									},
									Architecture: ptr.To("amd64"),
								},
								UpdateStrategy: ptr.To(core.AutoInPlaceUpdate),
							},
						}

						newShoot := shoot.DeepCopy()

						newShoot.Spec.Provider.Workers = []core.Worker{
							{
								Machine: core.Machine{
									Type: "machine-type-1",
									Image: &core.ShootMachineImage{
										Name: "constraint-image-name",
										// updated to higher expired version
										Version: "1.2.5",
									},
									Architecture: ptr.To("amd64"),
								},
								UpdateStrategy: ptr.To(core.AutoInPlaceUpdate),
							},
						}

						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(BeForbiddenError())
						Expect(err).To(MatchError(ContainSubstring("machine image version '%s' cannot be in-place updated from the current version, supported machine image versions are: [%s]", fmt.Sprintf("%s:%s", "constraint-image-name", "1.2.5"), fmt.Sprintf("%s:%s %s:%s", "constraint-image-name", "1.2.3", "constraint-image-name", "1.2.4"))))
					})

					It("should allow updating to a higher machine image for an existing worker pool with in-place update strategy if MinVersionForUpdate is less or equal current version", func() {
						cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages,
							gardencorev1beta1.MachineImage{
								Name: "constraint-image-name",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version:        "1.2.4",
											ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * 1000)},
										},
										Architectures: []string{"amd64"},
										InPlaceUpdates: &gardencorev1beta1.InPlaceUpdates{
											Supported:           true,
											MinVersionForUpdate: ptr.To("1.2.3"),
										},
									},
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: "1.2.3",
										},
										Architectures: []string{"amd64"},
										InPlaceUpdates: &gardencorev1beta1.InPlaceUpdates{
											Supported:           true,
											MinVersionForUpdate: ptr.To("1.2.3"),
										},
									},
								},
							},
						)

						shoot.Spec.Provider.Workers = []core.Worker{
							{
								Machine: core.Machine{
									Type: "machine-type-1",
									Image: &core.ShootMachineImage{
										Name:    "constraint-image-name",
										Version: "1.2.3",
									},
									Architecture: ptr.To("amd64"),
								},
								UpdateStrategy: ptr.To(core.AutoInPlaceUpdate),
							},
						}

						newShoot := shoot.DeepCopy()

						newShoot.Spec.Provider.Workers = []core.Worker{
							{
								Machine: core.Machine{
									Type: "machine-type-1",
									Image: &core.ShootMachineImage{
										Name:    "constraint-image-name",
										Version: "1.2.4",
									},
									Architecture: ptr.To("amd64"),
								},
								UpdateStrategy: ptr.To(core.AutoInPlaceUpdate),
							},
						}

						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(newShoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(&core.ShootMachineImage{Name: "constraint-image-name", Version: "1.2.4"}))
					})

					It("should keep machine image of the old shoot (unset in new shoot)", func() {
						newShoot := shoot.DeepCopy()
						newShoot.Spec.Provider.Workers[0].Machine.Image = nil

						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(newShoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(shoot.Spec.Provider.Workers[0].Machine.Image))
					})

					It("should keep machine image of the old shoot (version unset in new shoot)", func() {
						newShoot := shoot.DeepCopy()
						newShoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name: imageName1,
						}
						attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
						Expect(newShoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(shoot.Spec.Provider.Workers[0].Machine.Image))
					})

					It("should use updated machine image version as specified", func() {
						newShoot := shoot.DeepCopy()
						newShoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    imageName1,
							Version: nonExpiredVersion2,
						}

						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

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
						newWorker2.Machine.Architecture = ptr.To("arm64")
						newShoot.Spec.Provider.Workers = append(newShoot.Spec.Provider.Workers, *newWorker, *newWorker2)

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
						newWorker2.Machine.Architecture = ptr.To("arm64")
						newWorker.Machine.Image = &core.ShootMachineImage{
							Name: imageName2,
						}
						newWorker2.Machine.Image = &core.ShootMachineImage{
							Name: imageName2,
						}
						newShoot.Spec.Provider.Workers = append(newShoot.Spec.Provider.Workers, *newWorker, *newWorker2)

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
						newShoot.Spec.Provider.Workers[0].Machine.Architecture = ptr.To("arm64")

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
						Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
						Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
						Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    imageName1,
							Version: expiredVersion,
						}

						attrs := admission.NewAttributesRecord(nil, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).NotTo(HaveOccurred())
					})
				})
			})

			Context("machine type checks", func() {
				BeforeEach(func() {
					shoot.Spec.Provider.Workers = []core.Worker{
						{
							Machine: core.Machine{
								Type:         "machine-type-1",
								Architecture: ptr.To("amd64"),
							},
						},
					}

					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
				})

				It("should not reject due to an usable machine type", func() {
					shoot.Spec.Provider.Workers[0].Machine.Type = "machine-type-1"

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should reject due to a not usable machine type", func() {
					shoot.Spec.Provider.Workers[0].Machine.Type = "machine-type-old"

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err).To(MatchError(ContainSubstring("machine type %q is unusable", shoot.Spec.Provider.Workers[0].Machine.Type)))
				})

				It("should reject due to an invalid machine type", func() {
					shoot.Spec.Provider.Workers[0].Machine.Type = "not-present-in-cloudprofile"

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err).To(MatchError(ContainSubstring("Unsupported value: %q: supported values: %q, %q", "not-present-in-cloudprofile", "machine-type-1", "machine-type-2")))
				})

				It("should reject if the machine is unavailable in atleast one zone", func() {
					unavailableMachine := "unavailable-machine"
					zone := "some-zone"
					shoot.Spec.Provider.Workers[0].Machine.Type = unavailableMachine
					shoot.Spec.Provider.Workers[0].Zones = []string{
						zone,
					}

					cloudProfile.Spec.MachineTypes = append(cloudProfile.Spec.MachineTypes,
						gardencorev1beta1.MachineType{
							Name:         unavailableMachine,
							Architecture: ptr.To("amd64"),
							Usable:       ptr.To(true),
						},
					)
					cloudProfile.Spec.Regions = append(cloudProfile.Spec.Regions,
						gardencorev1beta1.Region{
							Name: shoot.Spec.Region,
							Zones: []gardencorev1beta1.AvailabilityZone{
								{
									Name: zone,
									UnavailableMachineTypes: []string{
										unavailableMachine,
									},
								},
							},
						},
					)

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err).To(MatchError(ContainSubstring("machine type %q is unavailable in at least one zone", unavailableMachine)))
				})

				It("should reject if the machine is not usable, is not having the same architecture mentioned in the cloudprofile and is not available in all zones", func() {
					zone := "some-zone"
					architecture := "amd64"
					shoot.Spec.Provider.Workers[0].Machine.Type = "machine-type-1"
					shoot.Spec.Provider.Workers[0].Machine.Architecture = &architecture
					shoot.Spec.Provider.Workers[0].Zones = []string{
						zone,
					}

					cloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{
						{
							Name:         "machine-type-1",
							Architecture: ptr.To("arm64"),
							Usable:       ptr.To(false),
						},
						{
							Name:         "machine-type-2",
							Architecture: ptr.To("amd64"),
							Usable:       ptr.To(true),
						},
					}
					cloudProfile.Spec.Regions = append(cloudProfile.Spec.Regions,
						gardencorev1beta1.Region{
							Name: shoot.Spec.Region,
							Zones: []gardencorev1beta1.AvailabilityZone{
								{
									Name: zone,
									UnavailableMachineTypes: []string{
										"machine-type-1",
									},
								},
							},
						},
					)

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err).To(MatchError(ContainSubstring("machine type %q is unusable, is unavailable in at least one zone, does not support CPU architecture %q, supported types are [%s]", "machine-type-1", architecture, "machine-type-2")))
				})
			})

			Context("volume checks", func() {
				BeforeEach(func() {
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
				})

				It("should reject due to an invalid volume type", func() {
					notAllowed := "not-allowed"
					shoot.Spec.Provider.Workers = []core.Worker{
						{
							Machine: core.Machine{
								Type:         "machine-type-1",
								Architecture: ptr.To("amd64"),
							},
							Volume: &core.Volume{
								Type: &notAllowed,
							},
						},
					}

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err).To(MatchError(ContainSubstring("Unsupported value: %q", notAllowed)))
				})

				It("should reject if the volume is unavailable in atleast one zone", func() {
					unavailableVolume := "unavailable-volume"
					zone := "europe-a"

					cloudProfile.Spec.VolumeTypes = []gardencorev1beta1.VolumeType{
						{
							Name:   unavailableVolume,
							Class:  "super-premium",
							Usable: ptr.To(true),
						},
						{
							Name:    volumeType2,
							Class:   "super-premium",
							MinSize: &minVolSize,
							Usable:  ptr.To(true),
						},
					}

					cloudProfile.Spec.Regions = []gardencorev1beta1.Region{{
						Name: shoot.Spec.Region,
						Zones: []gardencorev1beta1.AvailabilityZone{
							{
								Name:                   zone,
								UnavailableVolumeTypes: []string{unavailableVolume},
							},
						},
					}}

					shoot.Spec.Provider.Workers = []core.Worker{
						{
							Machine: core.Machine{
								Type:         "machine-type-1",
								Architecture: ptr.To("amd64"),
							},
							Volume: &core.Volume{
								Type: &unavailableVolume,
							},
							Zones: []string{zone},
						},
					}

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err).To(MatchError(ContainSubstring("volume type %q is unavailable in at least one zone, supported types are [%s]", unavailableVolume, volumeType2)))
				})

				It("should reject if the volume is unusable and unavailable in atleast one zone", func() {
					unavailableVolume := "unavailable-volume"
					zone := "europe-a"

					cloudProfile.Spec.VolumeTypes = []gardencorev1beta1.VolumeType{
						{
							Name:   unavailableVolume,
							Class:  "super-premium",
							Usable: ptr.To(false),
						},
						{
							Name:    volumeType2,
							Class:   "super-premium",
							MinSize: &minVolSize,
							Usable:  ptr.To(true),
						},
					}

					cloudProfile.Spec.Regions = []gardencorev1beta1.Region{{
						Name: shoot.Spec.Region,
						Zones: []gardencorev1beta1.AvailabilityZone{
							{
								Name:                   zone,
								UnavailableVolumeTypes: []string{unavailableVolume},
							},
						},
					}}

					shoot.Spec.Provider.Workers = []core.Worker{
						{
							Machine: core.Machine{
								Type:         "machine-type-1",
								Architecture: ptr.To("amd64"),
							},
							Volume: &core.Volume{
								Type: &unavailableVolume,
							},
							Zones: []string{zone},
						},
					}

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err).To(MatchError(ContainSubstring("volume type %q is unusable, is unavailable in at least one zone, supported types are [%s]", unavailableVolume, volumeType2)))
				})

				It("should reject because volume type is unusable", func() {
					cloudProfile.Spec.VolumeTypes = []gardencorev1beta1.VolumeType{
						{
							Name:   volumeType,
							Class:  "super-premium",
							Usable: ptr.To(false),
						},
						{
							Name:    volumeType2,
							Class:   "super-premium",
							MinSize: &minVolSize,
							Usable:  ptr.To(true),
						},
					}

					shoot.Spec.Provider.Workers = []core.Worker{
						{
							Machine: core.Machine{
								Type:         "machine-type-1",
								Architecture: ptr.To("amd64"),
							},
							Volume: &core.Volume{
								Type: &volumeType,
							},
						},
					}

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(BeForbiddenError())
					Expect(err).To(MatchError(ContainSubstring("volume type %q is unusable, supported types are [%s]", volumeType, volumeType2)))
				})

				It("should allow volume removal", func() {
					oldShoot := shoot.DeepCopy()
					shoot.Spec.Provider.Workers[0].Volume = nil
					oldShoot.Spec.Provider.Workers[0].Volume.VolumeSize = "20Gi"

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
								Architecture: ptr.To("amd64"),
							},
							Volume: &core.Volume{
								Type:       &volumeType2,
								VolumeSize: boundaryVolSize.String(),
							},
						},
						{
							Machine: core.Machine{
								Type:         "machine-type-2",
								Architecture: ptr.To("amd64"),
							},
							Volume: &core.Volume{
								Type:       &volumeType,
								VolumeSize: boundaryVolSize.String(),
							},
						},
						{
							Machine: core.Machine{
								Type:         "machine-type-2",
								Architecture: ptr.To("amd64"),
							},
							Volume: &core.Volume{
								Type:       &volumeType,
								VolumeSize: boundaryVolSizeMachine.String(),
							},
						},
					}

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
							Architecture: ptr.To("amd64"),
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
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

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
					Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

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

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

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

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

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

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("control plane migration", func() {
			var (
				oldSeedName string
				oldSeed     *gardencorev1beta1.Seed
				oldShoot    *core.Shoot
			)
			BeforeEach(func() {
				oldSeedName = fmt.Sprintf("old-%s", seedName)
				oldSeed = seed.DeepCopy()
				oldSeed.Name = oldSeedName

				oldShoot = shoot.DeepCopy()
				oldShoot.Spec.SeedName = &oldSeedName

				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(oldSeed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
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

			DescribeTable("Validating networking status during migration",
				func(networking *core.NetworkingStatus, matcher types.GomegaMatcher) {
					shoot.Status.Networking = networking

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(matcher)
				},
				Entry("should allow nil networking status", nil, Not(HaveOccurred())),
				Entry("should allow empty networking status", &core.NetworkingStatus{}, Not(HaveOccurred())),
				Entry("should allow correct networking status", &core.NetworkingStatus{Nodes: []string{nodesCIDR}, Pods: []string{podsCIDR}, Services: []string{servicesCIDR}}, Not(HaveOccurred())),
				Entry("should reject networking status if it is not disjoint with seed network", &core.NetworkingStatus{Nodes: []string{seedNodesCIDR}, Pods: []string{seedPodsCIDR}, Services: []string{seedServicesCIDR}}, BeForbiddenError()),
				Entry("should allow networking status with only egressCIDRs filled", &core.NetworkingStatus{EgressCIDRs: []string{"1.2.3.4/5"}}, Not(HaveOccurred())),
			)
		})

		Context("binding subresource", func() {
			var (
				oldShoot core.Shoot
				newSeed  gardencorev1beta1.Seed
			)

			BeforeEach(func() {
				oldShoot = *shootBase.DeepCopy()
				shoot = *shootBase.DeepCopy()
				seed = *seedBase.DeepCopy()
				newSeed = *seedBase.DeepCopy()
				newSeed.Name = "new-seed"

				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&newSeed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
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
					Expect(err.Error()).To(ContainSubstring("spec.seedName is already set to 'seed' and cannot be changed to 'nil'"))
				})

				It("should allow update of binding when shoot.spec.seedName is not nil", func() {
					shoot.Spec.SeedName = ptr.To(newSeed.Name)

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should reject update of binding if target seed does not exist", func() {
					shoot.Spec.SeedName = ptr.To(newSeed.Name + " other")

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Internal error occurred: could not find referenced seed"))
				})

				It("should reject update of binding if spec other than .spec.seedName is changed", func() {
					shoot.Spec.SeedName = ptr.To(newSeed.Name)
					shoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(true)}

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
					shoot.Spec.SeedName = ptr.To(newSeedName)
				})

				It("should reject update of binding if target seed is marked for deletion", func() {
					now := metav1.Now()
					newSeed.DeletionTimestamp = &now

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("cannot schedule shoot '%s' on seed '%s' that is already marked for deletion", shoot.Name, newSeedName))
				})

				It("should reject update of binding, because target Seed doesn't have configuration for backup", func() {
					newSeed.Spec.Backup = nil

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("backup is not configured for seed %q", newSeedName)))
				})

				It("should reject update of binding, because old Seed doesn't have configuration for backup", func() {
					seed.Spec.Backup = nil

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("backup is not configured for old seed %q", seedName)))
				})

				It("should allow update of binding to Seed with different provider type", func() {
					seed.Spec.Provider.Type = "gcp"
					newSeed.Spec.Provider.Type = "aws"

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("taints and tolerations", func() {
				BeforeEach(func() {
					shoot.Spec.SeedName = ptr.To(newSeedName)
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
				})

				It("update of binding should succeed because the Seed specified in the binding does not have any taints", func() {
					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("update of binding should fail because the seed specified in the binding has non-tolerated taints", func() {
					newSeed.Spec.Taints = []gardencorev1beta1.SeedTaint{{Key: gardencorev1beta1.SeedTaintProtected}}

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("forbidden to use a seed whose taints are not tolerated by the shoot"))
				})

				It("update of binding should fail because the new Seed specified in the binding has non-tolerated taints", func() {
					shoot.Spec.SeedName = ptr.To(newSeedName)
					newSeed.Spec.Taints = []gardencorev1beta1.SeedTaint{{Key: gardencorev1beta1.SeedTaintProtected}}

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("forbidden to use a seed whose taints are not tolerated by the shoot"))
				})

				It("update of binding should pass because shoot tolerates all taints of the seed", func() {
					newSeed.Spec.Taints = []gardencorev1beta1.SeedTaint{{Key: "foo"}}
					shoot.Spec.Tolerations = []core.Toleration{{Key: "foo", Value: ptr.To("bar")}}
					oldShoot.Spec.Tolerations = []core.Toleration{{Key: "foo", Value: ptr.To("bar")}}

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
					shoot.Spec.SeedName = ptr.To(newSeedName)
					allocatableShoots = *resource.NewQuantity(1, resource.DecimalSI)
					Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
					Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
				})

				It("update of binding should pass because seed allocatable capacity is not set", func() {
					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("update of binding should pass because seed allocatable capacity is not exhausted", func() {
					newSeed.Status.Allocatable = corev1.ResourceList{"shoots": allocatableShoots}

					otherShoot := versionedShoot.DeepCopy()
					otherShoot.Name = "other-shoot-1"
					otherShoot.Spec.SeedName = ptr.To("other-seed")
					Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

					otherShoot = versionedShoot.DeepCopy()
					otherShoot.Name = "other-shoot-2"
					otherShoot.Spec.SeedName = nil
					Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).NotTo(HaveOccurred())
				})

				It("update of binding should fail because seed allocatable capacity is exhausted", func() {
					newSeed.Status.Allocatable = corev1.ResourceList{"shoots": allocatableShoots}

					otherShoot := versionedShoot.DeepCopy()
					otherShoot.Name = "other-shoot-1"
					otherShoot.Spec.SeedName = ptr.To(newSeedName)
					Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

					otherShoot = versionedShoot.DeepCopy()
					otherShoot.Name = "other-shoot-2"
					otherShoot.Spec.SeedName = nil
					Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("already has the maximum number of shoots scheduled on it")))
				})

				It("update of binding should fail because seed allocatable capacity is over-exhausted", func() {
					newSeed.Status.Allocatable = corev1.ResourceList{"shoots": allocatableShoots}

					otherShoot := versionedShoot.DeepCopy()
					otherShoot.Name = "other-shoot-1"
					otherShoot.Spec.SeedName = ptr.To(newSeedName)
					Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

					otherShoot = versionedShoot.DeepCopy()
					otherShoot.Name = "other-shoot-2"
					otherShoot.Spec.SeedName = ptr.To(newSeedName)
					Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(otherShoot)).To(Succeed())

					attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "binding", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(context.TODO(), attrs, nil)

					Expect(err).To(MatchError(ContainSubstring("already has the maximum number of shoots scheduled on it")))
				})
			})
		})

		Context("shoot deletion", func() {
			var shootStore cache.Store

			BeforeEach(func() {
				shootStore = coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore()
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
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
						v1beta1constants.ConfirmationDeletion: "true",
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

		Context("checks for managed service account issuer", func() {
			var (
				oldShoot     *core.Shoot
				issuerSecret *corev1.Secret
			)

			BeforeEach(func() {
				issuerSecret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-issuer",
						Namespace: "garden",
						Labels: map[string]string{
							"gardener.cloud/role": "shoot-service-account-issuer",
						},
					},
					Data: map[string][]byte{
						"hostname": []byte("foo.bar"),
					},
				}

				shoot.Annotations = map[string]string{
					"authentication.gardener.cloud/issuer": "managed",
				}
				oldShoot = shoot.DeepCopy()
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seedBase)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
			})

			It("should reject creating a shoot if managed service account issuer is not configured", func() {
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("cannot enable managed service account issuer as it is not supported in this Gardener installation"))
			})

			It("should reject updating a shoot if managed service account issuer is not configured but old shoot has been annotated", func() {
				shoot.Annotations = nil
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeInternalServerError())
				Expect(err.Error()).To(ContainSubstring("old shoot object has managed service account issuer enabled, but Gardener configuration is missing"))
			})

			It("should reject disabling managed service account issuer", func() {
				shoot.Annotations = nil
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(issuerSecret)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("once enabled managed service account issuer cannot be disabled"))
			})

			It("should reject shoots with conflicting configuration managed service account issuer", func() {
				shoot.Spec.Kubernetes.KubeAPIServer = &core.KubeAPIServerConfig{
					ServiceAccountConfig: &core.ServiceAccountConfig{
						Issuer: ptr.To("foo"),
					},
				}
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(issuerSecret)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeForbiddenError())
				Expect(err.Error()).To(ContainSubstring("managed service account issuer cannot be enabled when .kubernetes.kubeAPIServer.serviceAccountConfig.issuer is set"))
			})

			It("should allow enabling managed service account issuer", func() {
				Expect(kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(issuerSecret)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
			})
		})

		Context("limits enforcement", func() {
			BeforeEach(func() {
				cloudProfile.Spec.Limits = &gardencorev1beta1.Limits{}
			})

			JustBeforeEach(func() {
				Expect(coreInformerFactory.Core().V1beta1().Projects().Informer().GetStore().Add(&project)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
			})

			It("should allow shoots if there are no limits", func() {
				cloudProfile.Spec.Limits = nil

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
			})

			It("should allow updating shoots with deletionTimestamp independent of limits", func() {
				shoot.DeletionTimestamp = &metav1.Time{Time: time.Now()}

				attrs := admission.NewAttributesRecord(&shoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.DeleteOptions{}, false, userInfo)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
			})

			It("should allow deleting shoots independent of limits", func() {
				attrs := admission.NewAttributesRecord(nil, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, userInfo)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
			})

			Context("maxNodesTotal", func() {
				const limit int32 = 3

				BeforeEach(func() {
					cloudProfile.Spec.Limits.MaxNodesTotal = ptr.To(limit)
				})

				It("should allow shoots within the limit", func() {
					shoot.Spec.Provider.Workers[0].Minimum = limit - 1
					shoot.Spec.Provider.Workers[0].Maximum = limit
					worker2 := shoot.Spec.Provider.Workers[0].DeepCopy()
					worker2.Minimum = 1
					worker2.Maximum = limit
					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, *worker2)

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)

					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				})

				It("should forbid shoots with individual maximum over the limit", func() {
					shoot.Spec.Provider.Workers[0].Minimum = 1
					shoot.Spec.Provider.Workers[0].Maximum = limit + 1
					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, *shoot.Spec.Provider.Workers[0].DeepCopy())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)

					err := admissionHandler.Admit(ctx, attrs, nil)
					Expect(err).To(BeForbiddenError())
					Expect(err).To(MatchError(And(
						ContainSubstring("spec.provider.workers[0].maximum"),
						ContainSubstring("the maximum node count of a worker pool must not exceed the limit of %d configured in the CloudProfile", limit),
						ContainSubstring("spec.provider.workers[1].maximum"),
						ContainSubstring("the maximum node count of a worker pool must not exceed the limit of %d configured in the CloudProfile", limit),
						Not(ContainSubstring("total minimum node count")),
					)))
				})

				It("should forbid shoots with total minimum over the limit", func() {
					shoot.Spec.Provider.Workers[0].Minimum = limit
					worker2 := shoot.Spec.Provider.Workers[0].DeepCopy()
					worker2.Minimum = 1
					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, *worker2)

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)

					err := admissionHandler.Admit(ctx, attrs, nil)
					Expect(err).To(BeForbiddenError())
					Expect(err).To(MatchError(And(
						ContainSubstring("spec.provider.workers"),
						ContainSubstring("total minimum node count"),
						Not(ContainSubstring("maximum node count of a worker pool")),
					)))
				})

				It("should forbid shoots with individual maximum and total minimum over the limit", func() {
					shoot.Spec.Provider.Workers[0].Minimum = limit
					shoot.Spec.Provider.Workers[0].Maximum = limit + 1
					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, *shoot.Spec.Provider.Workers[0].DeepCopy())

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)

					err := admissionHandler.Admit(ctx, attrs, nil)
					Expect(err).To(BeForbiddenError())
					Expect(err).To(MatchError(And(
						ContainSubstring("spec.provider.workers[0].maximum"),
						ContainSubstring("the maximum node count of a worker pool must not exceed the limit of %d configured in the CloudProfile", limit),
						ContainSubstring("spec.provider.workers[1].maximum"),
						ContainSubstring("the maximum node count of a worker pool must not exceed the limit of %d configured in the CloudProfile", limit),
						ContainSubstring("spec.provider.workers"),
						ContainSubstring("total minimum node count"),
					)))
				})

				It("should allow updating shoots with deletionTimestamp over the limit", func() {
					shoot.Spec.Provider.Workers[0].Minimum = limit + 1
					shoot.DeletionTimestamp = &metav1.Time{Time: time.Now()}

					attrs := admission.NewAttributesRecord(&shoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.DeleteOptions{}, false, userInfo)

					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				})

				It("should allow deleting shoots over the limit", func() {
					shoot.Spec.Provider.Workers[0].Minimum = limit + 1

					attrs := admission.NewAttributesRecord(nil, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, userInfo)

					Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
				})
			})
		})
	})
})
