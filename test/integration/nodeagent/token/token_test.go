// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package token_test

import (
	"context"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/controller/token"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Token controller tests", func() {
	var (
		testFS afero.Afero

		accessToken1, accessToken2 = []byte("access-token-1"), []byte("access-token-2")
		path1, path2               = "/some/path", "/some/other/path"
		secret1, secret2           *corev1.Secret
		syncPeriod                 = time.Second

		channel chan event.TypedGenericEvent[*corev1.Secret]
	)

	Context("requeued secret", func() {
		BeforeEach(func() {
			secret1Name, err := utils.GenerateRandomString(64)
			Expect(err).NotTo(HaveOccurred())
			secret2Name, err := utils.GenerateRandomString(64)
			Expect(err).NotTo(HaveOccurred())

			secret1 = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      strings.ToLower(secret1Name),
					Namespace: metav1.NamespaceSystem,
				},
				Data: map[string][]byte{resourcesv1alpha1.DataKeyToken: accessToken1},
			}
			secret2 = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      strings.ToLower(secret2Name),
					Namespace: metav1.NamespaceSystem,
				},
				Data: map[string][]byte{resourcesv1alpha1.DataKeyToken: accessToken2},
			}

			By("Setup manager")
			mgr, err := manager.New(restConfig, manager.Options{
				Metrics: metricsserver.Options{BindAddress: "0"},
				Cache: cache.Options{
					DefaultNamespaces: map[string]cache.Config{metav1.NamespaceSystem: {}},
				},
				Controller: controllerconfig.Controller{
					SkipNameValidation: ptr.To(true),
				},
			})
			Expect(err).NotTo(HaveOccurred())

			channel = make(chan event.TypedGenericEvent[*corev1.Secret])
			By("Register controller")
			testFS = afero.Afero{Fs: afero.NewMemMapFs()}
			Expect((&token.Reconciler{
				FS: testFS,
				Config: nodeagentconfigv1alpha1.TokenControllerConfig{
					SyncConfigs: []nodeagentconfigv1alpha1.TokenSecretSyncConfig{
						{
							SecretName: secret1.Name,
							Path:       path1,
						},
						{
							SecretName: secret2.Name,
							Path:       path2,
						},
					},
					SyncPeriod: &metav1.Duration{Duration: syncPeriod},
				},
			}).AddToManager(mgr, channel)).To(Succeed())

			By("Start manager")
			mgrContext, mgrCancel := context.WithCancel(ctx)

			go func() {
				defer GinkgoRecover()
				Expect(mgr.Start(mgrContext)).To(Succeed())
			}()

			DeferCleanup(func() {
				By("Stop manager")
				mgrCancel()
			})
		})

		JustBeforeEach(func() {
			By("Create access token secrets")
			Expect(testClient.Create(ctx, secret1)).To(Succeed())
			Expect(testClient.Create(ctx, secret2)).To(Succeed())
			DeferCleanup(func() {
				Expect(testClient.Delete(ctx, secret1)).To(Succeed())
				Expect(testClient.Delete(ctx, secret2)).To(Succeed())
			})
		})

		It("should write the tokens to the local file system", func() {
			Eventually(func(g Gomega) {
				token1OnDisk, err := afero.ReadFile(testFS, path1)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token1OnDisk).To(Equal(accessToken1))

				token2OnDisk, err := afero.ReadFile(testFS, path2)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token2OnDisk).To(Equal(accessToken2))
			}).Should(Succeed())
		})

		It("should update the tokens on the local file system after the sync period", func() {
			Eventually(func(g Gomega) {
				token1OnDisk, err := afero.ReadFile(testFS, path1)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token1OnDisk).To(Equal(accessToken1))

				token2OnDisk, err := afero.ReadFile(testFS, path2)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token2OnDisk).To(Equal(accessToken2))
			}).Should(Succeed())

			By("Update tokens in secret data")
			newToken1 := []byte("new-token1")
			patch := client.MergeFrom(secret1.DeepCopy())
			secret1.Data[resourcesv1alpha1.DataKeyToken] = newToken1
			Expect(testClient.Patch(ctx, secret1, patch)).To(Succeed())

			newToken2 := []byte("new-token1")
			patch = client.MergeFrom(secret2.DeepCopy())
			secret2.Data[resourcesv1alpha1.DataKeyToken] = newToken2
			Expect(testClient.Patch(ctx, secret2, patch)).To(Succeed())

			Eventually(func(g Gomega) {
				token1OnDisk, err := afero.ReadFile(testFS, path1)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token1OnDisk).To(Equal(newToken1))

				token2OnDisk, err := afero.ReadFile(testFS, path2)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token2OnDisk).To(Equal(newToken2))
			}).WithTimeout(2 * syncPeriod).Should(Succeed())
		})

		Context("unrelated secret", func() {
			BeforeEach(func() {
				secret1.Name = "some-other-secret"
				secret2.Name = "yet-another-secret"
			})

			It("should do nothing because the secret is unrelated", func() {
				Consistently(func(g Gomega) {
					token1OnDisk, err := afero.ReadFile(testFS, path1)
					g.Expect(token1OnDisk).To(BeEmpty())
					g.Expect(err).To(MatchError(ContainSubstring("file does not exist")))

					token2OnDisk, err := afero.ReadFile(testFS, path2)
					g.Expect(token2OnDisk).To(BeEmpty())
					g.Expect(err).To(MatchError(ContainSubstring("file does not exist")))
				}).Should(Succeed())
			})
		})
	})

	Context("source channel", func() {
		BeforeEach(func() {
			secret1Name, err := utils.GenerateRandomString(64)
			Expect(err).NotTo(HaveOccurred())
			secret2Name, err := utils.GenerateRandomString(64)
			Expect(err).NotTo(HaveOccurred())

			secret1 = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      strings.ToLower(secret1Name),
					Namespace: metav1.NamespaceSystem,
				},
				Data: map[string][]byte{resourcesv1alpha1.DataKeyToken: accessToken1},
			}
			secret2 = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      strings.ToLower(secret2Name),
					Namespace: metav1.NamespaceSystem,
				},
				Data: map[string][]byte{resourcesv1alpha1.DataKeyToken: accessToken2},
			}

			By("Setup manager")
			mgr, err := manager.New(restConfig, manager.Options{
				Metrics: metricsserver.Options{BindAddress: "0"},
				Cache: cache.Options{
					DefaultNamespaces: map[string]cache.Config{metav1.NamespaceSystem: {}},
				},
				Controller: controllerconfig.Controller{
					SkipNameValidation: ptr.To(true),
				},
			})
			Expect(err).NotTo(HaveOccurred())

			channel = make(chan event.TypedGenericEvent[*corev1.Secret])
			By("Register controller")
			testFS = afero.Afero{Fs: afero.NewMemMapFs()}
			Expect((&token.Reconciler{
				FS: testFS,
				Config: nodeagentconfigv1alpha1.TokenControllerConfig{
					SyncConfigs: []nodeagentconfigv1alpha1.TokenSecretSyncConfig{
						{
							SecretName: secret1.Name,
							Path:       path1,
						},
						{
							SecretName: secret2.Name,
							Path:       path2,
						},
					},
					// Use a higher time period to make sure the update is not due to sync period
					SyncPeriod: &metav1.Duration{Duration: time.Minute},
				},
			}).AddToManager(mgr, channel)).To(Succeed())

			By("Start manager")
			mgrContext, mgrCancel := context.WithCancel(ctx)

			go func() {
				defer GinkgoRecover()
				Expect(mgr.Start(mgrContext)).To(Succeed())
			}()

			DeferCleanup(func() {
				By("Stop manager")
				mgrCancel()
			})

			By("Create access token secrets")
			Expect(testClient.Create(ctx, secret1)).To(Succeed())
			Expect(testClient.Create(ctx, secret2)).To(Succeed())
			DeferCleanup(func() {
				Expect(testClient.Delete(ctx, secret1)).To(Succeed())
				Expect(testClient.Delete(ctx, secret2)).To(Succeed())
			})
		})

		It("should update the tokens on the local file system because of an event", func() {
			Eventually(func(g Gomega) {
				token1OnDisk, err := afero.ReadFile(testFS, path1)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token1OnDisk).To(Equal(accessToken1))

				token2OnDisk, err := afero.ReadFile(testFS, path2)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token2OnDisk).To(Equal(accessToken2))
			}).Should(Succeed())

			By("Update tokens in secret data")
			newToken1 := []byte("new-token1")
			patch := client.MergeFrom(secret1.DeepCopy())
			secret1.Data[resourcesv1alpha1.DataKeyToken] = newToken1
			Expect(testClient.Patch(ctx, secret1, patch)).To(Succeed())

			newToken2 := []byte("new-token1")
			patch = client.MergeFrom(secret2.DeepCopy())
			secret2.Data[resourcesv1alpha1.DataKeyToken] = newToken2
			Expect(testClient.Patch(ctx, secret2, patch)).To(Succeed())

			channel <- event.TypedGenericEvent[*corev1.Secret]{Object: secret1}
			channel <- event.TypedGenericEvent[*corev1.Secret]{Object: secret2}

			Eventually(func(g Gomega) {
				token1OnDisk, err := afero.ReadFile(testFS, path1)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token1OnDisk).To(Equal(newToken1))

				token2OnDisk, err := afero.ReadFile(testFS, path2)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token2OnDisk).To(Equal(newToken2))
			}).WithTimeout(2 * syncPeriod).Should(Succeed())
		})
	})
})
