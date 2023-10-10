// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package token_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/controller/token"
)

var _ = Describe("Token controller tests", func() {
	var (
		testFS afero.Fs

		accessToken = []byte("access-token")
		secret      *corev1.Secret
	)

	BeforeEach(func() {
		By("Setup manager")
		mgr, err := manager.New(restConfig, manager.Options{
			Metrics: metricsserver.Options{BindAddress: "0"},
			Cache: cache.Options{
				DefaultNamespaces: map[string]cache.Config{metav1.NamespaceSystem: {}},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		By("Register controller")
		testFS = afero.NewMemMapFs()
		Expect((&token.Reconciler{
			FS:                    testFS,
			AccessTokenSecretName: testRunID,
		}).AddToManager(mgr)).To(Succeed())

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

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testRunID,
				Namespace: metav1.NamespaceSystem,
			},
			Data: map[string][]byte{resourcesv1alpha1.DataKeyToken: accessToken},
		}
	})

	JustBeforeEach(func() {
		By("Create access token secret")
		Expect(testClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, secret)).To(Succeed())
		})
	})

	It("should write the token to the local file system", func() {
		Eventually(func(g Gomega) []byte {
			tokenOnDisk, err := afero.ReadFile(testFS, nodeagentv1alpha1.TokenFilePath)
			g.Expect(err).NotTo(HaveOccurred())
			return tokenOnDisk
		}).Should(Equal(accessToken))
	})

	It("should update the token on the local file system when it changes", func() {
		Eventually(func(g Gomega) []byte {
			tokenOnDisk, err := afero.ReadFile(testFS, nodeagentv1alpha1.TokenFilePath)
			g.Expect(err).NotTo(HaveOccurred())
			return tokenOnDisk
		}).Should(Equal(accessToken))

		By("Update token in secret data")
		newToken := []byte("new-token")
		patch := client.MergeFrom(secret.DeepCopy())
		secret.Data[resourcesv1alpha1.DataKeyToken] = newToken
		Expect(testClient.Patch(ctx, secret, patch)).To(Succeed())

		Eventually(func(g Gomega) []byte {
			tokenOnDisk, err := afero.ReadFile(testFS, nodeagentv1alpha1.TokenFilePath)
			g.Expect(err).NotTo(HaveOccurred())
			return tokenOnDisk
		}).Should(Equal(newToken))
	})

	Context("unrelated secret", func() {
		BeforeEach(func() {
			secret.Name = "some-other-secret"
		})

		It("should do nothing because the secret is unrelated", func() {
			Consistently(func(g Gomega) error {
				tokenOnDisk, err := afero.ReadFile(testFS, nodeagentv1alpha1.TokenFilePath)
				g.Expect(tokenOnDisk).To(BeEmpty())
				return err
			}).Should(MatchError(ContainSubstring("file does not exist")))
		})
	})
})
