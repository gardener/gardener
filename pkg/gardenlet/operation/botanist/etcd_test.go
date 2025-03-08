// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	mocketcd "github.com/gardener/gardener/pkg/component/etcd/etcd/mock"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Etcd", func() {
	var (
		ctrl             *gomock.Controller
		kubernetesClient kubernetes.Interface
		c                *mockclient.MockClient
		fakeClient       client.Client
		sm               secretsmanager.Interface
		botanist         *Botanist

		ctx                   = context.Background()
		fakeErr               = errors.New("fake err")
		namespace             = "shoot--foo--bar"
		role                  = "test"
		class                 = etcd.ClassImportant
		maintenanceTimeWindow = gardencorev1beta1.MaintenanceTimeWindow{
			Begin: "123456+0000",
			End:   "162543+0000",
		}

		validator *newEtcdValidator
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		kubernetesClient = fakekubernetes.NewClientSetBuilder().
			WithClient(c).
			Build()
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		sm = fakesecretsmanager.New(fakeClient, namespace)
		botanist = &Botanist{Operation: &operation.Operation{}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultEtcd", func() {
		BeforeEach(func() {
			botanist.SecretsManager = sm
			botanist.SeedClientSet = kubernetesClient
			botanist.Seed = &seedpkg.Seed{}
			botanist.Shoot = &shootpkg.Shoot{
				ControlPlaneNamespace: namespace,
			}
			botanist.Seed.SetInfo(&gardencorev1beta1.Seed{})
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.27.2",
					},
					Maintenance: &gardencorev1beta1.Maintenance{
						TimeWindow: &maintenanceTimeWindow,
					},
				},
			})

			validator = &newEtcdValidator{
				expectedClient:                  Equal(c),
				expectedLogger:                  BeAssignableToTypeOf(logr.Logger{}),
				expectedNamespace:               Equal(namespace),
				expectedSecretsManager:          Equal(sm),
				expectedRole:                    Equal(role),
				expectedClass:                   Equal(class),
				expectedReplicas:                PointTo(Equal(int32(1))),
				expectedStorageCapacity:         Equal("10Gi"),
				expectedDefragmentationSchedule: Equal(ptr.To("34 12 */3 * *")),
				expectedMaintenanceTimeWindow:   Equal(maintenanceTimeWindow),
				expectedHighAvailabilityEnabled: Equal(v1beta1helper.IsHAControlPlaneConfigured(botanist.Shoot.GetInfo())),
			}
		})

		Context("no ManagedSeed", func() {
			BeforeEach(func() {
				botanist.ManagedSeed = nil
			})

			for _, etcdClass := range []etcd.Class{etcd.ClassNormal, etcd.ClassImportant} {
				for _, shootPurpose := range []gardencorev1beta1.ShootPurpose{gardencorev1beta1.ShootPurposeEvaluation, gardencorev1beta1.ShootPurposeProduction, gardencorev1beta1.ShootPurposeInfrastructure} {
					var (
						class   = etcdClass
						purpose = shootPurpose
					)
					It(fmt.Sprintf("should successfully create an etcd interface: class = %q, purpose = %q", class, purpose), func() {
						botanist.Shoot.Purpose = purpose

						validator.expectedClass = Equal(class)

						oldNewEtcd := NewEtcd
						defer func() { NewEtcd = oldNewEtcd }()
						NewEtcd = validator.NewEtcd

						etcd, err := botanist.DefaultEtcd(role, class)
						Expect(etcd).NotTo(BeNil())
						Expect(err).NotTo(HaveOccurred())
					})
				}
			}
		})

		Context("with ManagedSeed", func() {
			BeforeEach(func() {
				botanist.ManagedSeed = &seedmanagementv1alpha1.ManagedSeed{}
				validator.expectedDefragmentationSchedule = Equal(ptr.To("34 12 * * *"))
			})

			It("should successfully create an etcd interface (normal class)", func() {

				oldNewEtcd := NewEtcd
				defer func() { NewEtcd = oldNewEtcd }()
				NewEtcd = validator.NewEtcd

				etcd, err := botanist.DefaultEtcd(role, class)
				Expect(etcd).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
			})

			It("should successfully create an etcd interface (important class)", func() {
				class := etcd.ClassImportant

				oldNewEtcd := NewEtcd
				defer func() { NewEtcd = oldNewEtcd }()
				NewEtcd = validator.NewEtcd

				etcd, err := botanist.DefaultEtcd(role, class)
				Expect(etcd).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("with minAllowed configuration", func() {
			var (
				minAllowedETCDMain   corev1.ResourceList
				minAllowedETCDEvents corev1.ResourceList
			)

			BeforeEach(func() {
				minAllowedETCDMain = corev1.ResourceList{"cpu": resource.MustParse("500m"), "memory": resource.MustParse("1Gi")}
				minAllowedETCDEvents = corev1.ResourceList{"cpu": resource.MustParse("100m")}

				botanist.Shoot.GetInfo().Spec.Kubernetes.ETCD = &gardencorev1beta1.ETCD{
					Main: &gardencorev1beta1.ETCDConfig{
						Autoscaling: &gardencorev1beta1.ControlPlaneAutoscaling{
							MinAllowed: minAllowedETCDMain,
						},
					},
					Events: &gardencorev1beta1.ETCDConfig{
						Autoscaling: &gardencorev1beta1.ControlPlaneAutoscaling{
							MinAllowed: minAllowedETCDEvents,
						},
					},
				}
			})

			It("should successfully create an etcd-main interface", func() {
				validator.expectedRole = Equal("main")
				validator.expectedAutoscalingConfiguration = Equal(etcd.AutoscalingConfig{MinAllowed: minAllowedETCDMain})

				oldNewEtcd := NewEtcd
				defer func() { NewEtcd = oldNewEtcd }()
				NewEtcd = validator.NewEtcd

				etcd, err := botanist.DefaultEtcd("main", class)
				Expect(etcd).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
			})

			It("should successfully create an etcd-events interface", func() {
				validator.expectedRole = Equal("events")
				validator.expectedAutoscalingConfiguration = Equal(etcd.AutoscalingConfig{MinAllowed: minAllowedETCDEvents})

				oldNewEtcd := NewEtcd
				defer func() { NewEtcd = oldNewEtcd }()
				NewEtcd = validator.NewEtcd

				etcd, err := botanist.DefaultEtcd("events", class)
				Expect(etcd).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
			})
		})

		It("should return an error because the maintenance time window cannot be parsed", func() {
			botanist.Shoot.GetInfo().Spec.Maintenance.TimeWindow = &gardencorev1beta1.MaintenanceTimeWindow{
				Begin: "foobar",
				End:   "barfoo",
			}

			etcd, err := botanist.DefaultEtcd(role, class)
			Expect(etcd).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#DeployEtcd", func() {
		var (
			etcdMain, etcdEvents *mocketcd.MockInterface
			shootUID             = types.UID("uuid")
		)

		BeforeEach(func() {
			etcdMain, etcdEvents = mocketcd.NewMockInterface(ctrl), mocketcd.NewMockInterface(ctrl)

			botanist.SeedClientSet = kubernetesClient
			botanist.Seed = &seedpkg.Seed{}
			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						EtcdMain:   etcdMain,
						EtcdEvents: etcdEvents,
					},
				},
				ControlPlaneNamespace: namespace,
				BackupEntryName:       namespace + "--" + string(shootUID),
				InternalClusterDomain: "internal.example.com",
			}
			botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					ClusterIdentity: ptr.To("seed-identity"),
				},
			})
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Maintenance: &gardencorev1beta1.Maintenance{
						TimeWindow: &maintenanceTimeWindow,
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					TechnicalID: namespace,
					UID:         shootUID,
				},
			})
		})

		It("should fail when the deploy function fails for etcd-main", func() {
			etcdMain.EXPECT().Deploy(ctx).Return(fakeErr)
			etcdEvents.EXPECT().Deploy(ctx)

			err := botanist.DeployEtcd(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(err.(*multierror.Error).Errors).To(ConsistOf(Equal(fakeErr)))
		})

		It("should fail when the deploy function fails for etcd-events", func() {
			etcdMain.EXPECT().Deploy(ctx)
			etcdEvents.EXPECT().Deploy(ctx).Return(fakeErr)

			err := botanist.DeployEtcd(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(err.(*multierror.Error).Errors).To(ConsistOf(Equal(fakeErr)))
		})

		Context("w/o backup", func() {
			BeforeEach(func() {
				botanist.Seed.GetInfo().Spec.Backup = nil
			})

			It("should set the secrets and deploy", func() {
				etcdMain.EXPECT().Deploy(ctx)
				etcdEvents.EXPECT().Deploy(ctx)

				Expect(botanist.DeployEtcd(ctx)).To(Succeed())
			})
		})

		Context("w/ backup", func() {
			var (
				backupProvider = "prov"
				bucketName     = "container"
				backupSecret   = &corev1.Secret{
					Data: map[string][]byte{
						"bucketName": []byte(bucketName),
					},
				}
				backupLeaderElectionConfig = &gardenletconfigv1alpha1.ETCDBackupLeaderElection{
					ReelectionPeriod: &metav1.Duration{Duration: 2 * time.Second},
				}

				expectGetBackupSecret = func() {
					c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "etcd-backup"}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
						func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
							backupSecret.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						},
					)
				}
				expectSetBackupConfig = func() {
					etcdMain.EXPECT().SetBackupConfig(&etcd.BackupConfig{
						Provider:             backupProvider,
						SecretRefName:        "etcd-backup",
						Prefix:               namespace + "--" + string(shootUID),
						Container:            bucketName,
						FullSnapshotSchedule: "1 12 * * *",
						LeaderElection:       backupLeaderElectionConfig,
					})
				}
			)

			BeforeEach(func() {
				botanist.Seed.GetInfo().Spec.Backup = &gardencorev1beta1.SeedBackup{
					Provider: backupProvider,
				}
				botanist.Config = &gardenletconfigv1alpha1.GardenletConfiguration{
					ETCDConfig: &gardenletconfigv1alpha1.ETCDConfig{
						BackupLeaderElection: backupLeaderElectionConfig,
					},
				}
			})

			It("should set secrets and deploy", func() {
				botanist.Shoot.GetInfo().Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
					HighAvailability: &gardencorev1beta1.HighAvailability{
						FailureTolerance: gardencorev1beta1.FailureTolerance{
							Type: gardencorev1beta1.FailureToleranceTypeNode,
						},
					},
				}

				expectGetBackupSecret()
				expectSetBackupConfig()
				etcdMain.EXPECT().Deploy(ctx)
				etcdEvents.EXPECT().Deploy(ctx)

				Expect(botanist.DeployEtcd(ctx)).To(Succeed())
			})

			It("should fail when reading the backup secret fails", func() {
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "etcd-backup"}, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr)

				Expect(botanist.DeployEtcd(ctx)).To(MatchError(fakeErr))
			})

			It("should fail when the backup schedule cannot be determined", func() {
				botanist.Shoot.GetInfo().Spec.Maintenance.TimeWindow = &gardencorev1beta1.MaintenanceTimeWindow{
					Begin: "foobar",
					End:   "barfoo",
				}
				expectGetBackupSecret()

				Expect(botanist.DeployEtcd(ctx)).To(HaveOccurred())
			})

			Context("cpm restore phase", func() {
				BeforeEach(func() {
					botanist.Shoot.GetInfo().Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
						HighAvailability: &gardencorev1beta1.HighAvailability{
							FailureTolerance: gardencorev1beta1.FailureTolerance{
								Type: gardencorev1beta1.FailureToleranceTypeNode,
							},
						},
					}
					botanist.Shoot.GetInfo().Status.LastOperation = &gardencorev1beta1.LastOperation{
						Type: gardencorev1beta1.LastOperationTypeRestore,
					}

					expectSetBackupConfig()
					expectGetBackupSecret()
				})

				It("should properly restore multi-node etcd from backup if etcd main does not exist yet", func() {
					// Expect the checks for whether multi-node etcd has to be restored
					etcdMain.EXPECT().Get(ctx).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, ""))

					for _, etcd := range []*mocketcd.MockInterface{etcdMain, etcdEvents} {
						gomock.InOrder(
							etcd.EXPECT().GetReplicas().Return(ptr.To[int32](3)),
							etcd.EXPECT().SetReplicas(ptr.To[int32](1)),
							etcd.EXPECT().Deploy(ctx),
							etcd.EXPECT().SetReplicas(ptr.To[int32](3)),
						)
					}

					Expect(botanist.DeployEtcd(ctx)).To(Succeed())
				})

				It("should properly restore multi-node etcd from backup if it is deployed with 1 replica", func() {
					etcdMain.EXPECT().Get(ctx).DoAndReturn(func(_ context.Context) (*druidv1alpha1.Etcd, error) {
						return &druidv1alpha1.Etcd{
							Spec: druidv1alpha1.EtcdSpec{
								Replicas: 1,
							},
						}, nil
					})

					for _, etcd := range []*mocketcd.MockInterface{etcdMain, etcdEvents} {
						gomock.InOrder(
							etcd.EXPECT().GetReplicas().Return(ptr.To[int32](3)),
							etcd.EXPECT().SetReplicas(ptr.To[int32](1)),
							etcd.EXPECT().Deploy(ctx),
							etcd.EXPECT().SetReplicas(ptr.To[int32](3)),
						)
					}

					Expect(botanist.DeployEtcd(ctx)).To(Succeed())
				})

				It("should not try to restore multi-node etcd from backup if it has already been scaled up", func() {
					etcdMain.EXPECT().Get(ctx).DoAndReturn(func(_ context.Context) (*druidv1alpha1.Etcd, error) {
						return &druidv1alpha1.Etcd{
							Spec: druidv1alpha1.EtcdSpec{
								Replicas: 3,
							},
						}, nil
					})
					etcdMain.EXPECT().Deploy(ctx)
					etcdEvents.EXPECT().Deploy(ctx)

					Expect(botanist.DeployEtcd(ctx)).To(Succeed())
				})

				It("should not try to restore multi-node etcd from backup if it has already been scaled down and the shoot is hibernated", func() {
					botanist.Shoot.HibernationEnabled = true

					etcdMain.EXPECT().Get(ctx).DoAndReturn(func(_ context.Context) (*druidv1alpha1.Etcd, error) {
						return &druidv1alpha1.Etcd{
							Spec: druidv1alpha1.EtcdSpec{
								Replicas: 0,
							},
						}, nil
					})
					etcdMain.EXPECT().Deploy(ctx)
					etcdEvents.EXPECT().Deploy(ctx)

					Expect(botanist.DeployEtcd(ctx)).To(Succeed())
				})
			})
		})
	})

	Describe("#DestroyEtcd", func() {
		var (
			etcdMain, etcdEvents *mocketcd.MockInterface
		)

		BeforeEach(func() {
			etcdMain, etcdEvents = mocketcd.NewMockInterface(ctrl), mocketcd.NewMockInterface(ctrl)

			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						EtcdMain:   etcdMain,
						EtcdEvents: etcdEvents,
					},
				},
			}
		})

		It("should fail when the destroy function fails for etcd-main", func() {
			etcdMain.EXPECT().Destroy(ctx).Return(fakeErr)
			etcdEvents.EXPECT().Destroy(ctx)

			err := botanist.DestroyEtcd(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(err.(*multierror.Error).Errors).To(ConsistOf(Equal(fakeErr)))
		})

		It("should fail when the destroy function fails for etcd-events", func() {
			etcdMain.EXPECT().Destroy(ctx)
			etcdEvents.EXPECT().Destroy(ctx).Return(fakeErr)

			err := botanist.DestroyEtcd(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(err.(*multierror.Error).Errors).To(ConsistOf(Equal(fakeErr)))
		})

		It("should succeed when both etcd-main and etcd-events destroy is successful", func() {
			etcdMain.EXPECT().Destroy(ctx)
			etcdEvents.EXPECT().Destroy(ctx)

			Expect(botanist.DestroyEtcd(ctx)).To(Succeed())
		})
	})
})

type newEtcdValidator struct {
	etcd.Interface

	expectedClient                   gomegatypes.GomegaMatcher
	expectedLogger                   gomegatypes.GomegaMatcher
	expectedNamespace                gomegatypes.GomegaMatcher
	expectedSecretsManager           gomegatypes.GomegaMatcher
	expectedRole                     gomegatypes.GomegaMatcher
	expectedClass                    gomegatypes.GomegaMatcher
	expectedReplicas                 gomegatypes.GomegaMatcher
	expectedStorageCapacity          gomegatypes.GomegaMatcher
	expectedDefragmentationSchedule  gomegatypes.GomegaMatcher
	expectedHighAvailabilityEnabled  gomegatypes.GomegaMatcher
	expectedMaintenanceTimeWindow    gomegatypes.GomegaMatcher
	expectedAutoscalingConfiguration gomegatypes.GomegaMatcher
}

func (v *newEtcdValidator) NewEtcd(
	log logr.Logger,
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values etcd.Values,
) etcd.Interface {
	Expect(log).To(v.expectedLogger)
	Expect(client).To(v.expectedClient)
	Expect(namespace).To(v.expectedNamespace)
	Expect(secretsManager).To(v.expectedSecretsManager)
	Expect(values.Role).To(v.expectedRole)
	Expect(values.Class).To(v.expectedClass)
	Expect(values.Replicas).To(v.expectedReplicas)
	Expect(values.StorageCapacity).To(v.expectedStorageCapacity)
	Expect(values.DefragmentationSchedule).To(v.expectedDefragmentationSchedule)
	Expect(values.HighAvailabilityEnabled).To(v.expectedHighAvailabilityEnabled)

	if v.expectedAutoscalingConfiguration != nil {
		Expect(values.Autoscaling).To(v.expectedAutoscalingConfiguration)
	} else {
		Expect(values.Autoscaling).To(Equal(etcd.AutoscalingConfig{}))
	}

	return v
}
