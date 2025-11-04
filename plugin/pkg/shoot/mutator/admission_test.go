// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator_test

import (
	"context"
	"net"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/controllerutils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/plugin/pkg/shoot/mutator"
)

var _ = Describe("mutator", func() {
	Describe("#Register", func() {
		It("should register the plugin", func() {
			plugins := admission.NewPlugins()
			Register(plugins)

			registered := plugins.Registered()
			Expect(registered).To(HaveLen(1))
			Expect(registered).To(ContainElement("ShootMutator"))
		})
	})

	Describe("#New", func() {
		It("should handle CREATE and UPDATE operations", func() {
			admissionHandler, err := New()
			Expect(err).NotTo(HaveOccurred())
			Expect(admissionHandler.Handles(admission.Create)).To(BeTrue())
			Expect(admissionHandler.Handles(admission.Update)).To(BeTrue())
			Expect(admissionHandler.Handles(admission.Connect)).To(BeFalse())
			Expect(admissionHandler.Handles(admission.Delete)).To(BeFalse())
		})
	})

	Describe("#ValidateInitialization", func() {
		It("should return error if a lister is missing", func() {
			admissionHandler, err := New()
			Expect(err).NotTo(HaveOccurred())

			err = admissionHandler.ValidateInitialization()
			Expect(err).To(MatchError("missing cloudProfile lister"))
		})

		It("should not return error if all listers are set", func() {
			admissionHandler, err := New()
			Expect(err).NotTo(HaveOccurred())
			coreInformerFactory := gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetCoreInformerFactory(coreInformerFactory)

			Expect(admissionHandler.ValidateInitialization()).To(Succeed())
		})
	})

	Describe("#Admit", func() {
		var (
			ctx context.Context

			userInfo              = &user.DefaultInfo{Name: "foo"}
			validMachineImageName = "some-machine-image"

			cloudProfile gardencorev1beta1.CloudProfile
			seed         gardencorev1beta1.Seed
			shoot        core.Shoot

			coreInformerFactory gardencoreinformers.SharedInformerFactory

			admissionHandler *MutateShoot
		)

		BeforeEach(func() {
			ctx = context.Background()

			cloudProfile = gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "profile",
				},
				Spec: gardencorev1beta1.CloudProfileSpec{
					MachineImages: []gardencorev1beta1.MachineImage{
						{
							Name: validMachineImageName,
							Versions: []gardencorev1beta1.MachineImageVersion{
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version: "0.0.1",
									},
									CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
										{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureAMD64}}},
										{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureARM64}}},
									},
									Architectures: []string{"amd64", "arm64"},
								},
							},
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
							Capabilities: gardencorev1beta1.Capabilities{
								"architecture":   []string{v1beta1constants.ArchitectureAMD64},
								"someCapability": []string{"value2"},
							},
						},
					},
				},
			}
			seed = gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: "seed",
				},
			}
			shoot = core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: "garden-my-project",
				},
				Spec: core.ShootSpec{
					CloudProfileName: ptr.To("profile"),
					Kubernetes: core.Kubernetes{
						Version: "1.6.4",
					},
					Provider: core.Provider{
						Workers: []core.Worker{
							{
								Name: "worker-name",
								Machine: core.Machine{
									Type: "machine-type-1",
									Image: &core.ShootMachineImage{
										Name:    validMachineImageName,
										Version: "0.0.1",
									},
									Architecture: ptr.To("amd64"),
								},
							},
						},
					},
				},
			}

			var err error
			admissionHandler, err = New()
			Expect(err).NotTo(HaveOccurred())
			admissionHandler.AssignReadyFunc(func() bool { return true })
			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetCoreInformerFactory(coreInformerFactory)
		})

		It("should ignore a kind other than shoot", func() {
			project := core.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
			}
			attrs := admission.NewAttributesRecord(&project, nil, core.Kind("Project").WithVersion("version"), "", project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)

			Expect(admissionHandler.Admit(ctx, attrs, nil)).NotTo(HaveOccurred())
		})

		It("should fail when object is not shoot", func() {
			project := core.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
			}
			attrs := admission.NewAttributesRecord(&project, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)

			err := admissionHandler.Admit(ctx, attrs, nil)
			Expect(err).To(BeBadRequestError())
			Expect(err).To(MatchError("could not convert object to Shoot"))
		})

		It("should fail when old object is not shoot", func() {
			project := core.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-project",
				},
			}
			attrs := admission.NewAttributesRecord(&shoot, &project, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)

			err := admissionHandler.Admit(ctx, attrs, nil)
			Expect(err).To(BeBadRequestError())
			Expect(err).To(MatchError("could not convert old object to Shoot"))
		})

		Context("reference checks", func() {
			It("should reject because the referenced cloud profile was not found", func() {
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(BeInternalServerError())
			})

			It("should exit early if CloudProfile is not set", func() {
				shoot.Spec.CloudProfileName = nil
				shoot.Spec.CloudProfile = nil

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)

				err := admissionHandler.Admit(ctx, attrs, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the referenced seed was not found", func() {
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())

				shoot.Spec.SeedName = ptr.To("seed")
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)

				err := admissionHandler.Admit(ctx, attrs, nil)
				Expect(err).To(BeInternalServerError())
			})
		})

		Context("created-by annotation", func() {
			BeforeEach(func() {
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
			})

			It("should add the created-by annotation on shoot creation", func() {
				Expect(shoot.Annotations).NotTo(HaveKeyWithValue(v1beta1constants.GardenCreatedBy, userInfo.Name))

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).NotTo(HaveOccurred())
				Expect(shoot.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenCreatedBy, userInfo.Name))
			})

			It("should not add the created-by annotation on shoot update", func() {
				oldShoot := shoot.DeepCopy()
				Expect(shoot.Annotations).NotTo(HaveKeyWithValue(v1beta1constants.GardenCreatedBy, userInfo.Name))

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).NotTo(HaveOccurred())
				Expect(shoot.Annotations).NotTo(HaveKey(v1beta1constants.GardenCreatedBy))
			})
		})

		Context("metadata annotations", func() {
			var (
				oldShoot *core.Shoot
			)

			BeforeEach(func() {
				oldShoot = shoot.DeepCopy()

				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
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

			It("should add deploy infrastructure task because ipFamilies have changed", func() {
				oldShoot.Spec.Networking = &core.Networking{
					IPFamilies: []core.IPFamily{core.IPFamilyIPv4},
				}
				shoot.Spec.Networking = &core.Networking{
					IPFamilies: []core.IPFamily{core.IPFamilyIPv4, core.IPFamilyIPv6},
				}
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, "deployInfrastructure")).To(BeTrue())
			})

			It("should add deploy infrastructure task because SSHAccess in WorkersSettings config has changed", func() {
				oldShoot.Spec.Provider.WorkersSettings = &core.WorkersSettings{
					SSHAccess: &core.SSHAccess{
						Enabled: true,
					},
				}
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

		Context("shoot maintenance checks", func() {
			var (
				oldShoot           *core.Shoot
				confineEnabled     = true
				specUpdate         = true
				operationFailed    = &core.LastOperation{State: core.LastOperationStateFailed}
				operationSucceeded = &core.LastOperation{State: core.LastOperationStateSucceeded}
			)
			BeforeEach(func() {
				shoot.Spec.Maintenance = &core.Maintenance{}
				oldShoot = shoot.DeepCopy()

				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
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

		Context("networking settings", func() {
			var (
				podsCIDR     = "100.96.0.0/11"
				servicesCIDR = "100.64.0.0/13"
			)

			BeforeEach(func() {
				seed.Spec.Networks.ShootDefaults = &gardencorev1beta1.ShootNetworks{
					Pods:     &podsCIDR,
					Services: &servicesCIDR,
				}
				shoot.Spec.SeedName = ptr.To(seed.Name)
				shoot.Spec.Networking = &core.Networking{
					Pods:       nil,
					Services:   nil,
					IPFamilies: []core.IPFamily{core.IPFamilyIPv4},
				}

				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
			})

			It("should not default shoot networks if shoot .spec.seedName is nil", func() {
				shoot.Spec.SeedName = nil

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Networking.Pods).To(BeNil())
				Expect(shoot.Spec.Networking.Services).To(BeNil())
			})

			It("should not default shoot networks if seed .spec.networks.shootDefaults is empty", func() {
				seed.Spec.Networks.ShootDefaults = &gardencorev1beta1.ShootNetworks{}

				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Networking.Pods).To(BeNil())
				Expect(shoot.Spec.Networking.Services).To(BeNil())
			})

			It("should skip pod network defaulting but default service network for workerless shoots", func() {
				shoot.Spec.Provider.Workers = nil

				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Networking.Pods).To(BeNil())
				Expect(shoot.Spec.Networking.Services).To(Equal(&servicesCIDR))
			})

			It("should generate ULA services CIDR for workerless IPv6 shoots without seed defaults", func() {
				shoot.Spec.Provider.Workers = nil
				shoot.Spec.Networking.IPFamilies = []core.IPFamily{core.IPFamilyIPv6}
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Networking.Pods).To(BeNil())

				Expect(shoot.Spec.Networking.Services).NotTo(BeNil())
				servicesCIDR := *shoot.Spec.Networking.Services
				_, ipNet, parseErr := net.ParseCIDR(servicesCIDR)
				Expect(parseErr).NotTo(HaveOccurred(), "Generated services CIDR should be valid")
				Expect(ipNet.IP.To16()).NotTo(BeNil(), "Generated services CIDR should be IPv6")
				Expect(ipNet.IP[0]).To(Equal(byte(0xfd)), "Generated services CIDR should be in ULA range (fd00::/8)")
				Expect(shoot.Spec.Networking.Services).To(Equal(&servicesCIDR))
			})

			It("should use seed defaults for workerless IPv6 shoots when available", func() {
				servicesCIDR := "2001:db8:10::/112"
				seed.Spec.Networks.ShootDefaults.Services = ptr.To(servicesCIDR)
				shoot.Spec.Provider.Workers = nil
				shoot.Spec.Networking.IPFamilies = []core.IPFamily{core.IPFamilyIPv6}
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Networking.Pods).To(BeNil())

				Expect(shoot.Spec.Networking.Services).NotTo(BeNil())
				Expect(shoot.Spec.Networking.Services).To(Equal(&servicesCIDR))
			})

			It("should not default shoot networks if shoot IP family does not match seed .spec.networks.shootDefaults", func() {
				shoot.Spec.Networking.IPFamilies = []core.IPFamily{core.IPFamilyIPv6}

				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Networking.Pods).To(BeNil())
				Expect(shoot.Spec.Networking.Services).To(BeNil())
			})

			It("should default shoot networks if all conditions are met", func() {
				Expect(coreInformerFactory.Core().V1beta1().Seeds().Informer().GetStore().Add(&seed)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Networking.Pods).To(Equal(&podsCIDR))
				Expect(shoot.Spec.Networking.Services).To(Equal(&servicesCIDR))
			})
		})

		Context("kubernetes version", func() {
			BeforeEach(func() {
				cloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{
					{Version: "1.28.0", Classification: ptr.To(gardencorev1beta1.ClassificationPreview)},
					{Version: "1.27.3"},
					{Version: "1.27.2"},
					{Version: "1.26.8", Classification: ptr.To(gardencorev1beta1.ClassificationDeprecated), ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)}},
					{Version: "1.26.7"},
					{Version: "1.26.6"},
					{Version: "1.25.11"},
					{Version: "1.24.12", Classification: ptr.To(gardencorev1beta1.ClassificationDeprecated), ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)}},
				}

				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
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
				Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.27.3"))
			})

			It("should default a major kubernetes version to latest minor.patch version", func() {
				shoot.Spec.Kubernetes.Version = "1"

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.27.3"))
			})

			It("should default a major.minor kubernetes version to latest patch version", func() {
				shoot.Spec.Kubernetes.Version = "1.26"

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.26.7"))
			})

			It("should reject defaulting a major.minor kubernetes version if there is no higher non-preview version available for defaulting", func() {
				shoot.Spec.Kubernetes.Version = "1.24"

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				err := admissionHandler.Admit(ctx, attrs, nil)

				Expect(err).To(MatchError(ContainSubstring("couldn't find a suitable version for 1.24")))
			})

			It("should be able to explicitly pick preview versions", func() {
				shoot.Spec.Kubernetes.Version = "1.28.0"

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

			Context("worker kubernetes version", func() {
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
					Expect(*shoot.Spec.Provider.Workers[0].Kubernetes.Version).To(Equal("1.26.7"))
				})
			})
		})

		DescribeTableSubtree("machine image", func(isCapabilityCloudProfile bool) {
			var (
				classificationPreview = gardencorev1beta1.ClassificationPreview

				imageName1 = validMachineImageName
				imageName2 = "other-image"

				expiredVersion                                  = "1.1.1"
				expiringVersion                                 = "1.2.1"
				nonExpiredVersion                               = "2.0.0"
				latestNonExpiredVersionThatSupportsCapabilities = "2.0.1"
				latestNonExpiredVersion                         = "2.1.0"
				previewVersion                                  = "3.0.0"

				cloudProfileMachineImages []gardencorev1beta1.MachineImage
			)

			BeforeEach(func() {
				machineCapabilities := []gardencorev1beta1.CapabilityDefinition{
					{Name: "architecture", Values: []string{v1beta1constants.ArchitectureAMD64}},
					{Name: "someCapability", Values: []string{"value1", "value2"}},
				}
				cloudProfileMachineImages = []gardencorev1beta1.MachineImage{
					{
						Name: validMachineImageName,
						Versions: []gardencorev1beta1.MachineImageVersion{
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        previewVersion,
									Classification: &classificationPreview,
								},
								CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureAMD64}}},
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureARM64}}},
								},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: latestNonExpiredVersion,
								},
								CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
									{Capabilities: gardencorev1beta1.Capabilities{
										"architecture":   []string{v1beta1constants.ArchitectureAMD64},
										"someCapability": []string{"value1"},
									}},
									{Capabilities: gardencorev1beta1.Capabilities{
										"architecture":   []string{v1beta1constants.ArchitectureARM64},
										"someCapability": []string{"value1"},
									}},
								}},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: latestNonExpiredVersionThatSupportsCapabilities,
								},
								CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureAMD64}}},
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureARM64}}},
								}},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: nonExpiredVersion,
								},
								CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureAMD64}}},
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureARM64}}},
								}},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        expiringVersion,
									ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * 1000)},
								},
								CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureAMD64}}},
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureARM64}}},
								}},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        expiredVersion,
									ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
								},
								CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureAMD64}}},
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureARM64}}},
								}},
						},
					}, {
						Name: imageName2,
						Versions: []gardencorev1beta1.MachineImageVersion{
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        previewVersion,
									Classification: &classificationPreview,
								},
								CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureAMD64}}},
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureARM64}}},
								}},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: latestNonExpiredVersion,
								},
								CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
									{Capabilities: gardencorev1beta1.Capabilities{
										"architecture":   []string{v1beta1constants.ArchitectureARM64},
										"someCapability": []string{"value1"},
									}},
									{Capabilities: gardencorev1beta1.Capabilities{
										"architecture":   []string{v1beta1constants.ArchitectureAMD64},
										"someCapability": []string{"value1"},
									}},
								}},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: latestNonExpiredVersionThatSupportsCapabilities,
								},
								CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureAMD64}}},
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureARM64}}},
								}},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: nonExpiredVersion,
								},
								CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureAMD64}}},
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureARM64}}},
								}},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        expiringVersion,
									ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * 1000)},
								},
								CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureAMD64}}},
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureARM64}}},
								}},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        expiredVersion,
									ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
								},
								CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureAMD64}}},
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureARM64}}},
								}},
						},
					},
				}

				if !isCapabilityCloudProfile {
					machineCapabilities = nil
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
										Version: nonExpiredVersion,
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version: latestNonExpiredVersionThatSupportsCapabilities,
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
										Version: nonExpiredVersion,
									},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version: latestNonExpiredVersionThatSupportsCapabilities,
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
				}

				cloudProfile.Spec.MachineCapabilities = machineCapabilities
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
						Zones:   []string{"europe-a"},
					})

					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
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

				It("should default version to latest supported non-preview version as shoot does not specify one", func() {
					expectedImageVersion := latestNonExpiredVersionThatSupportsCapabilities
					if !isCapabilityCloudProfile {
						expectedImageVersion = latestNonExpiredVersion
					}
					shoot.Spec.Provider.Workers[0].Machine.Image = nil
					shoot.Spec.Provider.Workers[1].Machine.Image = nil

					attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(&core.ShootMachineImage{
						Name:    imageName1,
						Version: expectedImageVersion,
					}))
					Expect(shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(&core.ShootMachineImage{
						Name:    imageName1,
						Version: latestNonExpiredVersion,
					}))
				})

				It("should default version to latest supported non-preview version as shoot only specifies name", func() {
					expectedImageVersion := latestNonExpiredVersionThatSupportsCapabilities
					if !isCapabilityCloudProfile {
						expectedImageVersion = latestNonExpiredVersion
					}
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
						Version: expectedImageVersion,
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
								CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureAMD64}}},
									{Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{v1beta1constants.ArchitectureARM64}}},
								}},
						)

						if !isCapabilityCloudProfile {
							cloudProfile.Spec.MachineImages[0].Versions = append(cloudProfile.Spec.MachineImages[0].Versions,
								gardencorev1beta1.MachineImageVersion{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: suffixedVersion},
									Architectures:    []string{"amd64", "arm64"},
								},
							)
						}
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

					It("should default a major.minor machine image version to latest supported patch version", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name:    imageName1,
							Version: "2.0",
						}

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(Not(HaveOccurred()))
						Expect(shoot.Spec.Provider.Workers[0].Machine.Image.Version).To(Equal(latestNonExpiredVersionThatSupportsCapabilities))
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

					It("should reject defaulting a machine image version for worker pool with inplace update strategy if there is no machine image available in the cloud profile supporting inplace update", func() {
						shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
							Name: imageName1,
						}
						shoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)

						attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Admit(ctx, attrs, nil)

						Expect(err).To(MatchError(ContainSubstring("failed to determine latest machine image from cloud profile")))
					})
				})
			})

			Context("update Shoot", func() {
				BeforeEach(func() {
					shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
						Name:    imageName1,
						Version: nonExpiredVersion,
					}
					shoot.Spec.Provider.Workers[0].Machine.Architecture = ptr.To("amd64")

					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
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
						Version: latestNonExpiredVersionThatSupportsCapabilities,
					}

					attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
					Expect(newShoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(&core.ShootMachineImage{
						Name:    imageName1,
						Version: latestNonExpiredVersionThatSupportsCapabilities,
					}))
				})

				It("should default a version prefix of an existing worker pool to the latest supported non-preview version", func() {
					expectedImageVersion := latestNonExpiredVersionThatSupportsCapabilities
					if !isCapabilityCloudProfile {
						expectedImageVersion = latestNonExpiredVersion
					}
					newShoot := shoot.DeepCopy()
					newShoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
						Name:    imageName1,
						Version: "2",
					}

					attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(Not(HaveOccurred()))
					Expect(newShoot.Spec.Provider.Workers[0].Machine.Image.Version).To(Equal(expectedImageVersion))
				})

				It("should default a version prefix of a new image of an existing worker pool to the latest supported non-preview version", func() {
					newShoot := shoot.DeepCopy()
					newShoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
						Name:    imageName2,
						Version: "2.0",
					}

					attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).To(Not(HaveOccurred()))
					Expect(newShoot.Spec.Provider.Workers[0].Machine.Image.Version).To(Equal(latestNonExpiredVersionThatSupportsCapabilities))
				})

				It("should default version of new worker pool to latest supported non-preview version", func() {
					expectedImageVersion := latestNonExpiredVersionThatSupportsCapabilities
					if !isCapabilityCloudProfile {
						expectedImageVersion = latestNonExpiredVersion
					}
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
						Version: expectedImageVersion,
					}))
					Expect(newShoot.Spec.Provider.Workers[2].Machine.Image).To(Equal(&core.ShootMachineImage{
						Name:    imageName1,
						Version: latestNonExpiredVersion,
					}))
				})

				It("should default version of new worker pool to latest supported non-preview version (version unset)", func() {
					expectedImageVersion := latestNonExpiredVersionThatSupportsCapabilities
					if !isCapabilityCloudProfile {
						expectedImageVersion = latestNonExpiredVersion
					}

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
						Version: expectedImageVersion,
					}))
					Expect(newShoot.Spec.Provider.Workers[2].Machine.Image).To(Equal(&core.ShootMachineImage{
						Name:    imageName2,
						Version: latestNonExpiredVersion,
					}))
				})

				It("should default version of worker pool to latest supported non-preview version when machine architecture is changed", func() {
					newShoot := shoot.DeepCopy()
					newShoot.Spec.Provider.Workers[0].Machine.Type = "machine-type-3"
					newShoot.Spec.Provider.Workers[0].Machine.Image = nil
					newShoot.Spec.Provider.Workers[0].Machine.Architecture = ptr.To("arm64")

					attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), newShoot.Namespace, newShoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
					Expect(newShoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(&core.ShootMachineImage{
						Name:    imageName1,
						Version: nonExpiredVersion,
					}))
				})

				It("should use version of new worker pool as specified", func() {
					newShoot := shoot.DeepCopy()
					newWorker := newShoot.Spec.Provider.Workers[0].DeepCopy()
					newWorker.Name = "second-worker"
					newWorker.Machine.Image = &core.ShootMachineImage{
						Name:    imageName2,
						Version: nonExpiredVersion,
					}
					newShoot.Spec.Provider.Workers = append(newShoot.Spec.Provider.Workers, *newWorker)

					attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), newShoot.Namespace, newShoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
					Expect(newShoot.Spec.Provider.Workers[0]).To(Equal(shoot.Spec.Provider.Workers[0]))
					Expect(newShoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(&core.ShootMachineImage{
						Name:    imageName2,
						Version: nonExpiredVersion,
					}))
				})

				It("should default version of new image to latest supported non-preview version (version unset)", func() {
					expectedImageVersion := latestNonExpiredVersionThatSupportsCapabilities
					if !isCapabilityCloudProfile {
						expectedImageVersion = latestNonExpiredVersion
					}

					shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
						Name:    imageName1,
						Version: latestNonExpiredVersionThatSupportsCapabilities,
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
						Version: expectedImageVersion,
					}))
				})

				It("should use version of new image as specified", func() {
					shoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
						Name:    imageName1,
						Version: latestNonExpiredVersionThatSupportsCapabilities,
					}

					newShoot := shoot.DeepCopy()
					newShoot.Spec.Provider.Workers[0].Machine.Image = &core.ShootMachineImage{
						Name:    imageName2,
						Version: latestNonExpiredVersionThatSupportsCapabilities,
					}

					attrs := admission.NewAttributesRecord(newShoot, &shoot, core.Kind("Shoot").WithVersion("version"), newShoot.Namespace, newShoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
					err := admissionHandler.Admit(ctx, attrs, nil)

					Expect(err).NotTo(HaveOccurred())
					Expect(newShoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(&core.ShootMachineImage{
						Name:    imageName2,
						Version: latestNonExpiredVersionThatSupportsCapabilities,
					}))
				})
			})

		},
			Entry("Cloudprofile is using Capabilities", true),
			Entry("Cloudprofile is NOT using Capabilities", false),
		)
	})
})
