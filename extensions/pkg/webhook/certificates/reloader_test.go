// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificates

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

var _ = Describe("Reloader", func() {
	var (
		ctx context.Context

		fakeClient       client.Client
		r                *reloader
		certDir          string
		namespace        = "garden"
		serverSecretName = "test-webhook-server"
		identity         = "gardener-extension-test-webhook"
	)

	BeforeEach(func() {
		ctx = context.Background()

		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme).Build()
		certDir = GinkgoT().TempDir()

		r = &reloader{
			SyncPeriod:       DefaultSyncPeriod,
			ServerSecretName: serverSecretName,
			Namespace:        namespace,
			Identity:         identity,
			reader:           fakeClient,
			certDir:          certDir,
		}
	})

	Describe("#getServerCert", func() {
		var log logr.Logger

		BeforeEach(func() {
			log = logr.Discard()
		})

		It("should return an error if no server secret exists", func() {
			_, _, _, err := r.getServerCert(ctx, log, fakeClient)
			Expect(err).To(MatchError(ContainSubstring("couldn't find webhook server secret")))
		})

		It("should return the certificate and key from a single secret", func() {
			secret := newServerSecret("secret-1", namespace, serverSecretName, identity,
				[]byte("test-cert"), []byte("test-key"))
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			name, cert, key, err := r.getServerCert(ctx, log, fakeClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(name).To(Equal("secret-1"))
			Expect(cert).To(Equal([]byte("test-cert")))
			Expect(key).To(Equal([]byte("test-key")))
		})

		It("should return the most recently created secret when multiple exist", func() {
			oldTime := metav1.NewTime(metav1.Now().Add(-1 * time.Hour))
			newTime := metav1.Now()

			secret1 := newServerSecret("secret-old", namespace, serverSecretName, identity,
				[]byte("old-cert"), []byte("old-key"))
			secret1.CreationTimestamp = oldTime

			secret2 := newServerSecret("secret-new", namespace, serverSecretName, identity,
				[]byte("new-cert"), []byte("new-key"))
			secret2.CreationTimestamp = newTime

			// Use a new client pre-seeded with the secrets so CreationTimestamp is preserved
			scheme := runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
			preSeededClient := fakeclient.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(secret1, secret2).
				Build()

			name, cert, key, err := r.getServerCert(ctx, log, preSeededClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(name).To(Equal("secret-new"))
			Expect(cert).To(Equal([]byte("new-cert")))
			Expect(key).To(Equal([]byte("new-key")))
		})

		It("should not match secrets with a different identity", func() {
			secret := newServerSecret("secret-1", namespace, serverSecretName, "different-identity",
				[]byte("test-cert"), []byte("test-key"))
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			_, _, _, err := r.getServerCert(ctx, log, fakeClient)
			Expect(err).To(MatchError(ContainSubstring("couldn't find webhook server secret")))
		})

		It("should not match secrets with a different name label", func() {
			secret := newServerSecret("secret-1", namespace, "other-server-secret", identity,
				[]byte("test-cert"), []byte("test-key"))
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			_, _, _, err := r.getServerCert(ctx, log, fakeClient)
			Expect(err).To(MatchError(ContainSubstring("couldn't find webhook server secret")))
		})

		It("should not match secrets in a different namespace", func() {
			secret := newServerSecret("secret-1", "other-namespace", serverSecretName, identity,
				[]byte("test-cert"), []byte("test-key"))
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			_, _, _, err := r.getServerCert(ctx, log, fakeClient)
			Expect(err).To(MatchError(ContainSubstring("couldn't find webhook server secret")))
		})
	})

	Describe("#Reconcile", func() {
		It("should return an error if no server secret exists", func() {
			_, err := r.Reconcile(ctx, reconcile.Request{})
			Expect(err).To(MatchError(ContainSubstring("error retrieving server certificate")))
		})

		It("should write the certificate to disk and requeue", func() {
			secret := newServerSecret("secret-1", namespace, serverSecretName, identity,
				[]byte("test-cert"), []byte("test-key"))
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := r.Reconcile(ctx, reconcile.Request{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(DefaultSyncPeriod))

			certOnDisk, err := os.ReadFile(filepath.Join(certDir, secretsutils.DataKeyCertificate))
			Expect(err).NotTo(HaveOccurred())
			Expect(certOnDisk).To(Equal([]byte("test-cert")))

			keyOnDisk, err := os.ReadFile(filepath.Join(certDir, secretsutils.DataKeyPrivateKey))
			Expect(err).NotTo(HaveOccurred())
			Expect(keyOnDisk).To(Equal([]byte("test-key")))
		})

		It("should skip writing to disk if the secret name has not changed", func() {
			secret := newServerSecret("secret-1", namespace, serverSecretName, identity,
				[]byte("test-cert"), []byte("test-key"))
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			// First reconcile: writes to disk
			result, err := r.Reconcile(ctx, reconcile.Request{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(DefaultSyncPeriod))
			Expect(r.newestServerSecretName).To(Equal("secret-1"))

			// Overwrite cert file with different content to verify it's not written again
			Expect(os.WriteFile(filepath.Join(certDir, secretsutils.DataKeyCertificate), []byte("modified-cert"), 0600)).To(Succeed())

			// Second reconcile: should not overwrite
			result, err = r.Reconcile(ctx, reconcile.Request{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(DefaultSyncPeriod))

			certOnDisk, err := os.ReadFile(filepath.Join(certDir, secretsutils.DataKeyCertificate))
			Expect(err).NotTo(HaveOccurred())
			Expect(certOnDisk).To(Equal([]byte("modified-cert")))
		})

		It("should write updated certificate to disk when secret name changes", func() {
			secret := newServerSecret("secret-1", namespace, serverSecretName, identity,
				[]byte("test-cert-1"), []byte("test-key-1"))
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			// First reconcile
			result, err := r.Reconcile(ctx, reconcile.Request{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(DefaultSyncPeriod))
			Expect(r.newestServerSecretName).To(Equal("secret-1"))

			// Delete old secret and create a new one with a different name
			Expect(fakeClient.Delete(ctx, secret)).To(Succeed())
			secret2 := newServerSecret("secret-2", namespace, serverSecretName, identity,
				[]byte("test-cert-2"), []byte("test-key-2"))
			Expect(fakeClient.Create(ctx, secret2)).To(Succeed())

			// Second reconcile: should write new cert
			result, err = r.Reconcile(ctx, reconcile.Request{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(DefaultSyncPeriod))
			Expect(r.newestServerSecretName).To(Equal("secret-2"))

			certOnDisk, err := os.ReadFile(filepath.Join(certDir, secretsutils.DataKeyCertificate))
			Expect(err).NotTo(HaveOccurred())
			Expect(certOnDisk).To(Equal([]byte("test-cert-2")))

			keyOnDisk, err := os.ReadFile(filepath.Join(certDir, secretsutils.DataKeyPrivateKey))
			Expect(err).NotTo(HaveOccurred())
			Expect(keyOnDisk).To(Equal([]byte("test-key-2")))
		})
	})

	Describe("nonLeaderElectionRunnable", func() {
		It("should report that it does not need leader election", func() {
			runnable := nonLeaderElectionRunnable{}
			Expect(runnable.NeedLeaderElection()).To(BeFalse())
		})
	})
})

func newServerSecret(name, namespace, configName, identity string, cert, key []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				secretsmanager.LabelKeyName:            configName,
				secretsmanager.LabelKeyManagedBy:       secretsmanager.LabelValueSecretsManager,
				secretsmanager.LabelKeyManagerIdentity: identity,
			},
		},
		Data: map[string][]byte{
			secretsutils.DataKeyCertificate: cert,
			secretsutils.DataKeyPrivateKey:  key,
		},
	}
}
