// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist_test

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mockkubernetes "github.com/gardener/gardener/pkg/mock/gardener/client/kubernetes"
	mocketcd "github.com/gardener/gardener/pkg/mock/gardener/operation/botanist/controlplane/etcd"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/etcd"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	"github.com/golang/mock/gomock"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Etcd", func() {
	var (
		ctrl             *gomock.Controller
		kubernetesClient *mockkubernetes.MockInterface
		c                *mockclient.MockClient
		botanist         *Botanist

		ctx                   = context.TODO()
		fakeErr               = fmt.Errorf("fake err")
		namespace             = "shoot--foo--bar"
		role                  = "test"
		class                 = etcd.ClassImportant
		maintenanceTimeWindow = gardencorev1beta1.MaintenanceTimeWindow{
			Begin: "123456+0000",
			End:   "162543+0000",
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		kubernetesClient = mockkubernetes.NewMockInterface(ctrl)
		c = mockclient.NewMockClient(ctrl)
		botanist = &Botanist{Operation: &operation.Operation{}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultEtcd", func() {
		var hvpaEnabled = true

		BeforeEach(func() {
			botanist.K8sSeedClient = kubernetesClient
			botanist.Seed = &seedpkg.Seed{
				Info: &gardencorev1beta1.Seed{},
			}
			botanist.Shoot = &shootpkg.Shoot{
				Info: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Maintenance: &gardencorev1beta1.Maintenance{
							TimeWindow: &maintenanceTimeWindow,
						},
					},
				},
				SeedNamespace: namespace,
			}
		})

		Context("no shooted seed", func() {
			BeforeEach(func() {
				botanist.ShootedSeed = nil
			})

			It("should successfully create an etcd interface (normal class)", func() {
				defer test.WithFeatureGate(gardenletfeatures.FeatureGate, features.HVPA, hvpaEnabled)()
				kubernetesClient.EXPECT().Client()

				validator := &newEtcdValidator{
					expectedClient:                  BeNil(),
					expectedNamespace:               Equal(namespace),
					expectedRole:                    Equal(role),
					expectedClass:                   Equal(class),
					expectedRetainReplicas:          BeFalse(),
					expectedStorageCapacity:         Equal("10Gi"),
					expectedDefragmentationSchedule: Equal(pointer.StringPtr("34 12 */3 * *")),
					expectedHVPAConfig: Equal(&etcd.HVPAConfig{
						Enabled:               hvpaEnabled,
						MaintenanceTimeWindow: maintenanceTimeWindow,
					}),
				}

				oldNewEtcd := NewEtcd
				defer func() { NewEtcd = oldNewEtcd }()
				NewEtcd = validator.NewEtcd

				etcd, err := botanist.DefaultEtcd(role, class)
				Expect(etcd).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
			})

			It("should successfully create an etcd interface (important class)", func() {
				class := etcd.ClassImportant

				defer test.WithFeatureGate(gardenletfeatures.FeatureGate, features.HVPA, hvpaEnabled)()
				kubernetesClient.EXPECT().Client()

				validator := &newEtcdValidator{
					expectedClient:                  BeNil(),
					expectedNamespace:               Equal(namespace),
					expectedRole:                    Equal(role),
					expectedClass:                   Equal(class),
					expectedRetainReplicas:          BeFalse(),
					expectedStorageCapacity:         Equal("10Gi"),
					expectedDefragmentationSchedule: Equal(pointer.StringPtr("34 12 */3 * *")),
					expectedHVPAConfig: Equal(&etcd.HVPAConfig{
						Enabled:               hvpaEnabled,
						MaintenanceTimeWindow: maintenanceTimeWindow,
					}),
				}

				oldNewEtcd := NewEtcd
				defer func() { NewEtcd = oldNewEtcd }()
				NewEtcd = validator.NewEtcd

				etcd, err := botanist.DefaultEtcd(role, class)
				Expect(etcd).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("no HVPAShootedSeed feature gate", func() {
			hvpaForShootedSeedEnabled := false

			BeforeEach(func() {
				botanist.ShootedSeed = &gardencorev1beta1helper.ShootedSeed{}
			})

			It("should successfully create an etcd interface (normal class)", func() {
				defer test.WithFeatureGate(gardenletfeatures.FeatureGate, features.HVPAForShootedSeed, hvpaForShootedSeedEnabled)()
				kubernetesClient.EXPECT().Client()

				validator := &newEtcdValidator{
					expectedClient:                  BeNil(),
					expectedNamespace:               Equal(namespace),
					expectedRole:                    Equal(role),
					expectedClass:                   Equal(class),
					expectedRetainReplicas:          BeFalse(),
					expectedStorageCapacity:         Equal("10Gi"),
					expectedDefragmentationSchedule: Equal(pointer.StringPtr("34 12 * * *")),
					expectedHVPAConfig: Equal(&etcd.HVPAConfig{
						Enabled:               hvpaForShootedSeedEnabled,
						MaintenanceTimeWindow: maintenanceTimeWindow,
					}),
				}

				oldNewEtcd := NewEtcd
				defer func() { NewEtcd = oldNewEtcd }()
				NewEtcd = validator.NewEtcd

				etcd, err := botanist.DefaultEtcd(role, class)
				Expect(etcd).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
			})

			It("should successfully create an etcd interface (important class)", func() {
				class := etcd.ClassImportant

				defer test.WithFeatureGate(gardenletfeatures.FeatureGate, features.HVPAForShootedSeed, hvpaForShootedSeedEnabled)()
				kubernetesClient.EXPECT().Client()

				validator := &newEtcdValidator{
					expectedClient:                  BeNil(),
					expectedNamespace:               Equal(namespace),
					expectedRole:                    Equal(role),
					expectedClass:                   Equal(class),
					expectedRetainReplicas:          BeFalse(),
					expectedStorageCapacity:         Equal("10Gi"),
					expectedDefragmentationSchedule: Equal(pointer.StringPtr("34 12 * * *")),
					expectedHVPAConfig: Equal(&etcd.HVPAConfig{
						Enabled:               hvpaForShootedSeedEnabled,
						MaintenanceTimeWindow: maintenanceTimeWindow,
					}),
				}

				oldNewEtcd := NewEtcd
				defer func() { NewEtcd = oldNewEtcd }()
				NewEtcd = validator.NewEtcd

				etcd, err := botanist.DefaultEtcd(role, class)
				Expect(etcd).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
			})
		})

		It("should return an error because the maintenance time window cannot be parsed", func() {
			defer test.WithFeatureGate(gardenletfeatures.FeatureGate, features.HVPA, true)()
			botanist.Shoot.Info.Spec.Maintenance.TimeWindow = &gardencorev1beta1.MaintenanceTimeWindow{
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
			etcdMain, etcdEvents *mocketcd.MockEtcd

			secretNameCA     = "ca-etcd"
			secretNameServer = "etcd-server-cert"
			secretNameClient = "etcd-client-tls"
			checksumCA       = "1234"
			checksumServer   = "5678"
			checksumClient   = "9012"
			shootUID         = types.UID("uuid")
		)

		BeforeEach(func() {
			etcdMain, etcdEvents = mocketcd.NewMockEtcd(ctrl), mocketcd.NewMockEtcd(ctrl)

			botanist.K8sSeedClient = kubernetesClient
			botanist.CheckSums = map[string]string{
				secretNameCA:     checksumCA,
				secretNameServer: checksumServer,
				secretNameClient: checksumClient,
			}
			botanist.Seed = &seedpkg.Seed{
				Info: &gardencorev1beta1.Seed{},
			}
			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						EtcdMain:   etcdMain,
						EtcdEvents: etcdEvents,
					},
				},
				Info: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Maintenance: &gardencorev1beta1.Maintenance{
							TimeWindow: &maintenanceTimeWindow,
						},
					},
					Status: gardencorev1beta1.ShootStatus{
						TechnicalID: namespace,
						UID:         shootUID,
					},
				},
				SeedNamespace: namespace,
			}

			etcdMain.EXPECT().SetSecrets(etcd.Secrets{
				CA:     component.Secret{Name: secretNameCA, Checksum: checksumCA},
				Server: component.Secret{Name: secretNameServer, Checksum: checksumServer},
				Client: component.Secret{Name: secretNameClient, Checksum: checksumClient},
			})
			etcdEvents.EXPECT().SetSecrets(etcd.Secrets{
				CA:     component.Secret{Name: secretNameCA, Checksum: checksumCA},
				Server: component.Secret{Name: secretNameServer, Checksum: checksumServer},
				Client: component.Secret{Name: secretNameClient, Checksum: checksumClient},
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
				botanist.Seed.Info.Spec.Backup = nil
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
			)

			BeforeEach(func() {
				botanist.Seed.Info.Spec.Backup = &gardencorev1beta1.SeedBackup{
					Provider: backupProvider,
				}
			})

			It("should set the secrets and deploy", func() {
				kubernetesClient.EXPECT().Client().Return(c)
				c.EXPECT().Get(ctx, kutil.Key(namespace, "etcd-backup"), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					backupSecret.DeepCopyInto(obj.(*corev1.Secret))
					return nil
				})
				etcdMain.EXPECT().SetBackupConfig(&etcd.BackupConfig{
					Provider:             backupProvider,
					SecretRefName:        "etcd-backup",
					Prefix:               namespace + "--" + string(shootUID),
					Container:            bucketName,
					FullSnapshotSchedule: "1 12 * * *",
				})

				etcdMain.EXPECT().Deploy(ctx)
				etcdEvents.EXPECT().Deploy(ctx)
				Expect(botanist.DeployEtcd(ctx)).To(Succeed())
			})

			It("should fail when reading the backup secret fails", func() {
				kubernetesClient.EXPECT().Client().Return(c)
				c.EXPECT().Get(ctx, kutil.Key(namespace, "etcd-backup"), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr)

				Expect(botanist.DeployEtcd(ctx)).To(MatchError(fakeErr))
			})

			It("should fail when the backup schedule cannot be determined", func() {
				kubernetesClient.EXPECT().Client().Return(c)
				c.EXPECT().Get(ctx, kutil.Key(namespace, "etcd-backup"), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					backupSecret.DeepCopyInto(obj.(*corev1.Secret))
					return nil
				})
				botanist.Shoot.Info.Spec.Maintenance.TimeWindow = &gardencorev1beta1.MaintenanceTimeWindow{
					Begin: "foobar",
					End:   "barfoo",
				}

				Expect(botanist.DeployEtcd(ctx)).To(HaveOccurred())
			})
		})
	})

	Describe("#ScaleETCDTo*", func() {
		var (
			etcdEvents = &druidv1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "etcd-events",
					Namespace: namespace,
				},
			}
			etcdMain = &druidv1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "etcd-main",
					Namespace: namespace,
				},
			}
		)

		BeforeEach(func() {
			botanist.K8sSeedClient = kubernetesClient
			botanist.Shoot = &shootpkg.Shoot{SeedNamespace: namespace}
		})

		Describe("#ScaleETCDToZero", func() {
			var patch = client.RawPatch(types.MergePatchType, []byte(`{"spec":{"replicas":0}}`))

			It("should scale both etcds to 0", func() {
				kubernetesClient.EXPECT().DirectClient().Return(c).Times(2)
				c.EXPECT().Patch(ctx, etcdEvents, patch)
				c.EXPECT().Patch(ctx, etcdMain, patch)

				Expect(botanist.ScaleETCDToZero(ctx)).To(Succeed())
			})

			It("should return the error when scaling etcd-events fails", func() {
				kubernetesClient.EXPECT().DirectClient().Return(c)
				c.EXPECT().Patch(ctx, etcdEvents, patch).Return(fakeErr)

				Expect(botanist.ScaleETCDToZero(ctx)).To(MatchError(fakeErr))
			})

			It("should return the error when scaling etcd-main fails", func() {
				kubernetesClient.EXPECT().DirectClient().Return(c).Times(2)
				c.EXPECT().Patch(ctx, etcdEvents, patch)
				c.EXPECT().Patch(ctx, etcdMain, patch).Return(fakeErr)

				Expect(botanist.ScaleETCDToZero(ctx)).To(MatchError(fakeErr))
			})
		})

		Describe("#ScaleETCDToOne", func() {
			var patch = client.RawPatch(types.MergePatchType, []byte(`{"spec":{"replicas":1}}`))

			It("should scale both etcds to 1", func() {
				kubernetesClient.EXPECT().DirectClient().Return(c).Times(2)
				c.EXPECT().Patch(ctx, etcdEvents, patch)
				c.EXPECT().Patch(ctx, etcdMain, patch)

				Expect(botanist.ScaleETCDToOne(ctx)).To(Succeed())
			})

			It("should return the error when scaling etcd-events fails", func() {
				kubernetesClient.EXPECT().DirectClient().Return(c)
				c.EXPECT().Patch(ctx, etcdEvents, patch).Return(fakeErr)

				Expect(botanist.ScaleETCDToOne(ctx)).To(MatchError(fakeErr))
			})

			It("should return the error when scaling etcd-main fails", func() {
				kubernetesClient.EXPECT().DirectClient().Return(c).Times(2)
				c.EXPECT().Patch(ctx, etcdEvents, patch)
				c.EXPECT().Patch(ctx, etcdMain, patch).Return(fakeErr)

				Expect(botanist.ScaleETCDToOne(ctx)).To(MatchError(fakeErr))
			})
		})
	})
})

type newEtcdValidator struct {
	etcd.Etcd

	expectedClient                  gomegatypes.GomegaMatcher
	expectedNamespace               gomegatypes.GomegaMatcher
	expectedRole                    gomegatypes.GomegaMatcher
	expectedClass                   gomegatypes.GomegaMatcher
	expectedRetainReplicas          gomegatypes.GomegaMatcher
	expectedStorageCapacity         gomegatypes.GomegaMatcher
	expectedDefragmentationSchedule gomegatypes.GomegaMatcher
	expectedHVPAConfig              gomegatypes.GomegaMatcher
}

func (v *newEtcdValidator) NewEtcd(
	client client.Client,
	namespace string,
	role string,
	class etcd.Class,
	retainReplicas bool,
	storageCapacity string,
	defragmentationSchedule *string,
) etcd.Etcd {
	Expect(client).To(v.expectedClient)
	Expect(namespace).To(v.expectedNamespace)
	Expect(role).To(v.expectedRole)
	Expect(class).To(v.expectedClass)
	Expect(retainReplicas).To(v.expectedRetainReplicas)
	Expect(storageCapacity).To(v.expectedStorageCapacity)
	Expect(defragmentationSchedule).To(v.expectedDefragmentationSchedule)

	return v
}

func (v *newEtcdValidator) SetHVPAConfig(config *etcd.HVPAConfig) {
	Expect(config).To(v.expectedHVPAConfig)
}
