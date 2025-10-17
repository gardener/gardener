// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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

	Describe("#Admit", func() {
		var (
			ctx context.Context

			userInfo = &user.DefaultInfo{Name: "foo"}

			shoot core.Shoot

			admissionHandler *MutateShoot
		)

		BeforeEach(func() {
			ctx = context.Background()

			shoot = core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: "garden-my-project",
				},
			}

			var err error
			admissionHandler, err = New()
			Expect(err).NotTo(HaveOccurred())
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

		Context("created-by annotation", func() {
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
	})
})
