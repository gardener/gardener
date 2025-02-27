// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
	"time"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	etcdcopybackupstask "github.com/gardener/gardener/pkg/component/etcd/copybackupstask"
	mocketcdcopybackupstask "github.com/gardener/gardener/pkg/component/etcd/copybackupstask/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/test"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("EtcdCopyBackupsTask", func() {
	var (
		ctx              context.Context
		ctrl             *gomock.Controller
		c                *mockclient.MockClient
		reader           *mockclient.MockReader
		kubernetesClient kubernetes.Interface

		botanist        *Botanist
		namespace       = "shoot--foo--bar"
		shootName       = "bar"
		projectName     = "foo"
		seedName        = "seed"
		backupEntryName = "backup-entry"
	)

	BeforeEach(func() {
		ctx = context.TODO()
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		reader = mockclient.NewMockReader(ctrl)
		kubernetesClient = fakekubernetes.NewClientSetBuilder().
			WithClient(c).
			WithAPIReader(reader).
			Build()

		botanist = &Botanist{Operation: &operation.Operation{}}
		botanist.SeedClientSet = kubernetesClient
		botanist.Seed = &seedpkg.Seed{}
		botanist.Shoot = &shootpkg.Shoot{
			ControlPlaneNamespace: namespace,
			BackupEntryName:       backupEntryName,
		}
		botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: seedName,
			},
			Spec: gardencorev1beta1.SeedSpec{
				Backup: &gardencorev1beta1.SeedBackup{
					Provider: "gcp",
				},
			},
		})
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: projectName,
			},
		})
	})

	Describe("#DefaultEtcdCopyBackupsTask", func() {
		It("should create a new EtcdCopyBackupsTask deploy waiter", func() {
			etcdCopyBackupsTask := botanist.DefaultEtcdCopyBackupsTask()
			Expect(etcdCopyBackupsTask).NotTo(BeNil())
		})

		It("should create a new EtcdCopyBackupsTask with correct values", func() {
			validator := &newEtcdCopyBackupsTaskValidator{
				expectedClient: Equal(c),
				expectedLogger: BeAssignableToTypeOf(logr.Logger{}),
				expectedValues: Equal(&etcdcopybackupstask.Values{
					Name:      botanist.Shoot.GetInfo().Name,
					Namespace: botanist.Shoot.ControlPlaneNamespace,
					WaitForFinalSnapshot: &druidv1alpha1.WaitForFinalSnapshotSpec{
						Enabled: true,
						Timeout: &metav1.Duration{Duration: etcdcopybackupstask.DefaultTimeout},
					},
				}),
				expectedWaitInterval:        Equal(etcdcopybackupstask.DefaultInterval),
				expectedWaitSevereThreshold: Equal(etcdcopybackupstask.DefaultSevereThreshold),
				expectedWaitTimeout:         Equal(etcdcopybackupstask.DefaultTimeout),
			}

			defer test.WithVars(&NewEtcdCopyBackupsTask, validator.NewEtcdCopyBackupsTask)()
			NewEtcdCopyBackupsTask = validator.NewEtcdCopyBackupsTask

			etcdCopyBackupsTask := botanist.DefaultEtcdCopyBackupsTask()
			Expect(etcdCopyBackupsTask).NotTo(BeNil())
		})
	})

	Describe("#DeployEtcdCopyBackupsTask", func() {
		var (
			etcdCopyBackupsTask    *mocketcdcopybackupstask.MockInterface
			etcdBackupSecret       *corev1.Secret
			sourceEtcdBackupSecret *corev1.Secret
			sourceBackupEntry      *extensionsv1alpha1.BackupEntry

			secretGroupResource      = schema.GroupResource{Resource: "Secrets"}
			backupEntryGroupResource = schema.GroupResource{Resource: "BackupEntries"}
			fakeErr                  = errors.New("fake err")
		)

		BeforeEach(func() {
			etcdCopyBackupsTask = mocketcdcopybackupstask.NewMockInterface(ctrl)
			botanist.Shoot.Components = &shootpkg.Components{
				ControlPlane: &shootpkg.ControlPlane{
					EtcdCopyBackupsTask: etcdCopyBackupsTask,
				},
			}

			etcdBackupSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "etcd-backup",
					Namespace: namespace,
				},
			}
			sourceEtcdBackupSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "source-etcd-backup",
					Namespace: namespace,
				},
			}
			sourceBackupEntry = &extensionsv1alpha1.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "source-" + backupEntryName,
				},
				Spec: extensionsv1alpha1.BackupEntrySpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						Type: "aws",
					},
				},
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should properly deploy EtcdCopyBackupsTask resource", func() {
			etcdCopyBackupsTask.EXPECT().Destroy(ctx)
			etcdCopyBackupsTask.EXPECT().WaitCleanup(ctx)
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(sourceBackupEntry), gomock.AssignableToTypeOf(sourceBackupEntry))
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(sourceEtcdBackupSecret), gomock.AssignableToTypeOf(sourceEtcdBackupSecret))
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(etcdBackupSecret), gomock.AssignableToTypeOf(etcdBackupSecret))
			etcdCopyBackupsTask.EXPECT().SetSourceStore(gomock.AssignableToTypeOf(druidv1alpha1.StoreSpec{}))
			etcdCopyBackupsTask.EXPECT().SetTargetStore(gomock.AssignableToTypeOf(druidv1alpha1.StoreSpec{}))
			etcdCopyBackupsTask.EXPECT().Deploy(ctx)
			Expect(botanist.DeployEtcdCopyBackupsTask(ctx)).To(Succeed())
		})

		It("should return an error if removal of old EtcdCopyBackupsTask resource fails", func() {
			etcdCopyBackupsTask.EXPECT().Destroy(ctx).Return(fakeErr)
			Expect(botanist.DeployEtcdCopyBackupsTask(ctx)).To(HaveOccurred())
		})

		It("should return an error if waiting to remove old EtcdCopyBackupsTask fails", func() {
			etcdCopyBackupsTask.EXPECT().Destroy(ctx)
			etcdCopyBackupsTask.EXPECT().WaitCleanup(ctx).Return(fakeErr)
			Expect(botanist.DeployEtcdCopyBackupsTask(ctx)).To(HaveOccurred())
		})

		It("should return an error if the etcd backup secret is not found", func() {
			etcdCopyBackupsTask.EXPECT().Destroy(ctx)
			etcdCopyBackupsTask.EXPECT().WaitCleanup(ctx)
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(sourceBackupEntry), gomock.AssignableToTypeOf(sourceBackupEntry))
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(sourceEtcdBackupSecret), gomock.AssignableToTypeOf(sourceEtcdBackupSecret)).Return(apierrors.NewNotFound(secretGroupResource, sourceEtcdBackupSecret.Name))
			Expect(botanist.DeployEtcdCopyBackupsTask(ctx)).To(HaveOccurred())
		})

		It("should return an error if the source backup entry is not found", func() {
			etcdCopyBackupsTask.EXPECT().Destroy(ctx)
			etcdCopyBackupsTask.EXPECT().WaitCleanup(ctx)
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(sourceBackupEntry), gomock.AssignableToTypeOf(sourceBackupEntry)).Return(apierrors.NewNotFound(backupEntryGroupResource, etcdBackupSecret.Name))
			Expect(botanist.DeployEtcdCopyBackupsTask(ctx)).To(HaveOccurred())
		})

		It("should return an error if the source etcd backup secret is not found", func() {
			etcdCopyBackupsTask.EXPECT().Destroy(ctx)
			etcdCopyBackupsTask.EXPECT().WaitCleanup(ctx)
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(sourceBackupEntry), gomock.AssignableToTypeOf(sourceBackupEntry))
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(sourceEtcdBackupSecret), gomock.AssignableToTypeOf(sourceEtcdBackupSecret))
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(etcdBackupSecret), gomock.AssignableToTypeOf(etcdBackupSecret)).Return(apierrors.NewNotFound(secretGroupResource, etcdBackupSecret.Name))
			Expect(botanist.DeployEtcdCopyBackupsTask(ctx)).To(HaveOccurred())
		})

		It("should return an error if the etcd copy backup task component Deploy fails", func() {
			etcdCopyBackupsTask.EXPECT().Destroy(ctx)
			etcdCopyBackupsTask.EXPECT().WaitCleanup(ctx)
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(sourceBackupEntry), gomock.AssignableToTypeOf(sourceBackupEntry))
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(sourceEtcdBackupSecret), gomock.AssignableToTypeOf(sourceEtcdBackupSecret))
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(etcdBackupSecret), gomock.AssignableToTypeOf(etcdBackupSecret))
			etcdCopyBackupsTask.EXPECT().SetSourceStore(gomock.AssignableToTypeOf(druidv1alpha1.StoreSpec{}))
			etcdCopyBackupsTask.EXPECT().SetTargetStore(gomock.AssignableToTypeOf(druidv1alpha1.StoreSpec{}))
			etcdCopyBackupsTask.EXPECT().Deploy(ctx).Return(fakeErr)
			Expect(botanist.DeployEtcdCopyBackupsTask(ctx)).To(MatchError(fakeErr))
		})
	})
})

type newEtcdCopyBackupsTaskValidator struct {
	etcdcopybackupstask.Interface

	expectedClient              gomegatypes.GomegaMatcher
	expectedLogger              gomegatypes.GomegaMatcher
	expectedValues              gomegatypes.GomegaMatcher
	expectedWaitInterval        gomegatypes.GomegaMatcher
	expectedWaitSevereThreshold gomegatypes.GomegaMatcher
	expectedWaitTimeout         gomegatypes.GomegaMatcher
}

func (v *newEtcdCopyBackupsTaskValidator) NewEtcdCopyBackupsTask(
	logger logr.Logger,
	client client.Client,
	values *etcdcopybackupstask.Values,
	waitInterval time.Duration,
	waitSevereThreshold time.Duration,
	waitTimeout time.Duration,
) etcdcopybackupstask.Interface {
	Expect(client).To(v.expectedClient)
	Expect(logger).To(v.expectedLogger)
	Expect(values).To(v.expectedValues)
	Expect(waitInterval).To(v.expectedWaitInterval)
	Expect(waitSevereThreshold).To(v.expectedWaitSevereThreshold)
	Expect(waitTimeout).To(v.expectedWaitTimeout)

	return v
}
