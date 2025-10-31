// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericactuator_test

import (
	"context"
	"maps"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener/extensions/pkg/controller/backupentry"
	"github.com/gardener/gardener/extensions/pkg/controller/backupentry/genericactuator"
	extensionsmockgenericactuator "github.com/gardener/gardener/extensions/pkg/controller/backupentry/genericactuator/mock"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
)

const (
	providerSecretName      = "backupprovider"
	providerSecretNamespace = "garden"
	shootTechnicalID        = "shoot--foo--bar"
	shootUID                = "asd234-asd-34"
	bucketName              = "test-bucket"
)

var _ = Describe("Actuator", func() {
	var (
		ctx = context.Background()
		log logr.Logger

		ctrl *gomock.Controller
		mgr  *mockmanager.MockManager

		backupEntry              *extensionsv1alpha1.BackupEntry
		backupProviderSecretData map[string][]byte
		backupEntrySecret        *corev1.Secret
		etcdBackupSecretData     map[string][]byte
		etcdBackupSecretKey      client.ObjectKey
		etcdBackupSecret         *corev1.Secret
		seedNamespace            *corev1.Namespace

		fakeClient client.Client
		a          backupentry.Actuator
	)

	BeforeEach(func() {
		logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
		log = logf.Log.WithName("test")

		ctrl = gomock.NewController(GinkgoT())

		backupEntry = &extensionsv1alpha1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: shootTechnicalID + "--" + shootUID,
			},
			Spec: extensionsv1alpha1.BackupEntrySpec{
				BucketName: bucketName,
				SecretRef: corev1.SecretReference{
					Name:      providerSecretName,
					Namespace: providerSecretNamespace,
				},
			},
		}
		backupProviderSecretData = map[string][]byte{
			"foo":        []byte("bar"),
			"bucketName": []byte(bucketName),
		}
		backupEntrySecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      providerSecretName,
				Namespace: providerSecretNamespace,
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}
		etcdBackupSecretData = map[string][]byte{
			"bucketName": []byte(bucketName),
			"foo":        []byte("bar"),
		}
		etcdBackupSecretKey = client.ObjectKey{Namespace: shootTechnicalID, Name: "etcd-backup"}
		etcdBackupSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "etcd-backup",
				Namespace: shootTechnicalID,
			},
			Data: etcdBackupSecretData,
		}
		seedNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: shootTechnicalID,
			},
		}

		mgr = mockmanager.NewMockManager(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("#Reconcile", func() {
		var backupEntryDelegate *extensionsmockgenericactuator.MockBackupEntryDelegate

		BeforeEach(func() {
			backupEntryDelegate = extensionsmockgenericactuator.NewMockBackupEntryDelegate(ctrl)
		})

		Context("seed namespace exist", func() {
			It("should create etcd-backup secret if it does not exist", func() {
				fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(seedNamespace, backupEntrySecret).Build()
				mgr.EXPECT().GetClient().Return(fakeClient)
				backupEntryDelegate.EXPECT().GetETCDSecretData(ctx, gomock.AssignableToTypeOf(logr.Logger{}), backupEntry, backupProviderSecretData).Return(etcdBackupSecretData, nil)

				a = genericactuator.NewActuator(mgr, backupEntryDelegate)
				Expect(a.Reconcile(ctx, log, backupEntry)).To(Succeed())

				actual := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, etcdBackupSecretKey, actual)).To(Succeed())
				Expect(actual.Annotations).To(HaveKeyWithValue("backup.gardener.cloud/created-by", backupEntry.Name))
				Expect(actual.Data).To(Equal(etcdBackupSecretData))
			})

			It("should update etcd-backup secret if it already exists", func() {
				existingEtcdBackupSecret := etcdBackupSecret.DeepCopy()
				existingEtcdBackupSecret.Data = map[string][]byte{
					"new-key": []byte("new-value"),
				}
				fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(seedNamespace, backupEntrySecret, existingEtcdBackupSecret).Build()
				mgr.EXPECT().GetClient().Return(fakeClient)
				backupEntryDelegate.EXPECT().GetETCDSecretData(ctx, gomock.AssignableToTypeOf(logr.Logger{}), backupEntry, backupProviderSecretData).Return(etcdBackupSecretData, nil)

				a = genericactuator.NewActuator(mgr, backupEntryDelegate)
				Expect(a.Reconcile(ctx, log, backupEntry)).To(Succeed())

				actual := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, etcdBackupSecretKey, actual)).To(Succeed())
				Expect(actual.Annotations).To(HaveKeyWithValue("backup.gardener.cloud/created-by", backupEntry.Name))
				Expect(actual.Data).To(Equal(etcdBackupSecretData))
			})

			It("should remove all unknown annotations and labels from the etcd-backup secret", func() {
				existingEtcdBackupSecret := etcdBackupSecret.DeepCopy()
				testKeyValue := map[string]string{"key1": "value1", "key2:": "value2"}
				existingEtcdBackupSecret.Annotations = testKeyValue
				existingEtcdBackupSecret.Labels = testKeyValue

				fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(seedNamespace, backupEntrySecret, existingEtcdBackupSecret).Build()
				mgr.EXPECT().GetClient().Return(fakeClient)
				backupEntryDelegate.EXPECT().GetETCDSecretData(ctx, gomock.AssignableToTypeOf(logr.Logger{}), backupEntry, backupProviderSecretData).Return(etcdBackupSecretData, nil)

				a = genericactuator.NewActuator(mgr, backupEntryDelegate)
				Expect(a.Reconcile(ctx, log, backupEntry)).To(Succeed())

				actual := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, etcdBackupSecretKey, actual)).To(Succeed())
				Expect(actual.Annotations).To(Equal(map[string]string{"backup.gardener.cloud/created-by": backupEntry.Name}))
				Expect(actual.Labels).To(BeEmpty())
			})

			Context("#WorkloadIdentity", func() {
				var existingEtcdBackupSecret *corev1.Secret

				BeforeEach(func() {
					backupEntrySecret.Annotations = map[string]string{
						"workloadidentity.security.gardener.cloud/context-object": `{"kind":"BackupEntry","apiVersion":"core.gardener.cloud/v1beta1","name":"bar","namespace":"test","uid":""}`,
						"workloadidentity.security.gardener.cloud/namespace":      "test-namespace",
						"workloadidentity.security.gardener.cloud/name":           "test-workload-identity",
					}
					backupEntrySecret.Labels = map[string]string{
						"security.gardener.cloud/purpose":                   "workload-identity-token-requestor",
						"workloadidentity.security.gardener.cloud/provider": "test",
					}
					backupEntrySecret.Data["config"] = []byte("config")
					backupEntrySecret.Data["to-be-deleted1"] = []byte("value")
					backupEntrySecret.Data["token"] = []byte("to-be-replaced")

					existingEtcdBackupSecret = etcdBackupSecret.DeepCopy()
					existingEtcdBackupSecret.Data = map[string][]byte{
						"to-be-deleted2": []byte("value"),
						"token":          []byte("to-be-preserved"),
					}
					existingEtcdBackupSecret.Annotations = map[string]string{
						"workloadidentity.security.gardener.cloud/token-renew-timestamp": "renew",
						"to-be-deleted": "to-be-deleted",
					}
					existingEtcdBackupSecret.Labels = map[string]string{
						"to-be-deleted": "to-be-deleted",
					}

					fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(seedNamespace, backupEntrySecret, existingEtcdBackupSecret).Build()
					mgr.EXPECT().GetClient().Return(fakeClient)
					passedData := map[string][]byte{
						"bucketName": []byte(bucketName),
						"config":     backupEntrySecret.Data["config"],
					}
					expectedData := maps.Clone(passedData)
					expectedData["mock"] = []byte("true")
					backupEntryDelegate.EXPECT().GetETCDSecretData(ctx, gomock.AssignableToTypeOf(logr.Logger{}), backupEntry, passedData).Return(expectedData, nil)
					a = genericactuator.NewActuator(mgr, backupEntryDelegate)
				})

				It("should update already existing secret with workload identity details", func() {
					Expect(a.Reconcile(ctx, log, backupEntry)).To(Succeed())

					actual := &corev1.Secret{}
					Expect(fakeClient.Get(ctx, etcdBackupSecretKey, actual)).To(Succeed())

					Expect(actual.Annotations).To(HaveLen(5))
					Expect(actual.Annotations).To(HaveKeyWithValue("backup.gardener.cloud/created-by", backupEntry.Name))
					Expect(actual.Annotations).To(HaveKeyWithValue("workloadidentity.security.gardener.cloud/name", "test-workload-identity"))
					Expect(actual.Annotations).To(HaveKeyWithValue("workloadidentity.security.gardener.cloud/namespace", "test-namespace"))
					Expect(actual.Annotations).To(HaveKeyWithValue("workloadidentity.security.gardener.cloud/context-object", `{"kind":"BackupEntry","apiVersion":"core.gardener.cloud/v1beta1","name":"bar","namespace":"test","uid":""}`))
					Expect(actual.Annotations).To(HaveKeyWithValue("workloadidentity.security.gardener.cloud/token-renew-timestamp", "renew"))
					Expect(actual.Annotations).ToNot(HaveKey("to-be-deleted"))

					Expect(actual.Labels).To(HaveLen(2))
					Expect(actual.Labels).To(HaveKeyWithValue("security.gardener.cloud/purpose", "workload-identity-token-requestor"))
					Expect(actual.Labels).To(HaveKeyWithValue("workloadidentity.security.gardener.cloud/provider", "test"))
					Expect(actual.Labels).ToNot(HaveKey("to-be-deleted"))

					Expect(actual.Data).To(HaveLen(4))
					Expect(actual.Data).To(HaveKeyWithValue("config", []byte("config")))
					Expect(actual.Data).To(HaveKeyWithValue("token", []byte("to-be-preserved")))
					Expect(actual.Data).To(HaveKeyWithValue("mock", []byte("true")))
					Expect(actual.Data).To(HaveKeyWithValue("bucketName", []byte(bucketName)))
					Expect(actual.Data).ToNot(HaveKey("to-be-deleted1"))
					Expect(actual.Data).ToNot(HaveKey("to-be-deleted2"))
				})

				It("should fail to reconcile the secret due to missing WorkloadIdentity name ", func() {
					delete(backupEntrySecret.Annotations, "workloadidentity.security.gardener.cloud/name")
					Expect(fakeClient.Update(ctx, backupEntrySecret)).To(Succeed())

					err := a.Reconcile(ctx, log, backupEntry)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("BackupEntry is set to use workload identity but WorkloadIdentity's name is missing"))
				})

				It("should fail to reconcile the secret due to missing WorkloadIdentity namespace ", func() {
					delete(backupEntrySecret.Annotations, "workloadidentity.security.gardener.cloud/namespace")
					Expect(fakeClient.Update(ctx, backupEntrySecret)).To(Succeed())

					err := a.Reconcile(ctx, log, backupEntry)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("BackupEntry is set to use workload identity but WorkloadIdentity's namespace is missing"))
				})

				It("should fail to reconcile the secret due to missing WorkloadIdentity provider type ", func() {
					delete(backupEntrySecret.Labels, "workloadidentity.security.gardener.cloud/provider")
					Expect(fakeClient.Update(ctx, backupEntrySecret)).To(Succeed())

					err := a.Reconcile(ctx, log, backupEntry)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("BackupEntry is set to use workload identity but WorkloadIdentity's provider type missing"))
				})

				It("should not set token data key when the etcd secret ", func() {
					delete(existingEtcdBackupSecret.Data, "token")
					Expect(fakeClient.Update(ctx, existingEtcdBackupSecret)).To(Succeed())

					Expect(a.Reconcile(ctx, log, backupEntry)).To(Succeed())

					actual := &corev1.Secret{}
					Expect(fakeClient.Get(ctx, etcdBackupSecretKey, actual)).To(Succeed())
					Expect(actual.Data).ToNot(HaveKey("token"))
				})
			})
		})

		Context("seed namespace does not exist", func() {
			It("should not create etcd-backup secret", func() {
				fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(backupEntrySecret).Build()
				mgr.EXPECT().GetClient().Return(fakeClient)

				a = genericactuator.NewActuator(mgr, backupEntryDelegate)
				Expect(a.Reconcile(ctx, log, backupEntry)).To(Succeed())

				Expect(fakeClient.Get(ctx, etcdBackupSecretKey, &corev1.Secret{})).To(BeNotFoundError())
			})
		})
	})

	Context("#Delete", func() {
		var backupEntryDelegate *extensionsmockgenericactuator.MockBackupEntryDelegate

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(seedNamespace, backupEntrySecret).Build()
			mgr.EXPECT().GetClient().Return(fakeClient)

			backupEntryDelegate = extensionsmockgenericactuator.NewMockBackupEntryDelegate(ctrl)
			backupEntryDelegate.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(logr.Logger{}), backupEntry).Return(nil)
		})

		It("should not delete etcd-backup secret if has annotation with different BackupEntry name", func() {
			etcdBackupSecret.Annotations = map[string]string{
				"backup.gardener.cloud/created-by": "foo",
			}
			Expect(fakeClient.Create(ctx, etcdBackupSecret)).To(Succeed())

			a = genericactuator.NewActuator(mgr, backupEntryDelegate)
			Expect(a.Delete(ctx, log, backupEntry)).To(Succeed())

			actual := &corev1.Secret{}
			Expect(fakeClient.Get(ctx, etcdBackupSecretKey, actual)).To(Succeed())
			Expect(actual.Annotations).To(HaveKeyWithValue("backup.gardener.cloud/created-by", "foo"))
			Expect(actual.Data).To(Equal(etcdBackupSecretData))
		})

		It("should delete etcd-backup secret if it has annotation with same BackupEntry name", func() {
			etcdBackupSecret.Annotations = map[string]string{
				"backup.gardener.cloud/created-by": backupEntry.Name,
			}
			Expect(fakeClient.Create(ctx, etcdBackupSecret)).To(Succeed())

			a = genericactuator.NewActuator(mgr, backupEntryDelegate)
			Expect(a.Delete(ctx, log, backupEntry)).To(Succeed())

			Expect(fakeClient.Get(ctx, etcdBackupSecretKey, &corev1.Secret{})).To(BeNotFoundError())
		})

		It("should delete etcd-backup secret if it has no annotations", func() {
			Expect(fakeClient.Create(ctx, etcdBackupSecret)).To(Succeed())

			a = genericactuator.NewActuator(mgr, backupEntryDelegate)
			Expect(a.Delete(ctx, log, backupEntry)).To(Succeed())

			Expect(fakeClient.Get(ctx, etcdBackupSecretKey, &corev1.Secret{})).To(BeNotFoundError())
		})
	})
})
