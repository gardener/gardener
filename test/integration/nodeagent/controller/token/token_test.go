// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/controller/token"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
)

var _ = Describe("Nodeagent token controller tests", func() {
	var (
		testFs                 afero.Fs
		originalNodeAgentToken string
	)

	BeforeEach(func() {
		By("Setup manager")
		mapper, err := apiutil.NewDynamicRESTMapper(restConfig)
		Expect(err).NotTo(HaveOccurred())

		mgr, err := manager.New(restConfig, manager.Options{
			Scheme:             operatorclient.RuntimeScheme,
			MetricsBindAddress: "0",
			NewCache: cache.BuilderWithOptions(cache.Options{
				Mapper: mapper,
			}),
		})
		Expect(err).NotTo(HaveOccurred())
		mgrClient = mgr.GetClient()

		By("Register controller")
		testFs = afero.NewMemMapFs()
		nodeAgentConfig := &nodeagentv1alpha1.NodeAgentConfiguration{
			TokenSecretName: v1alpha1.NodeAgentTokenSecretName,
		}
		configBytes, err := yaml.Marshal(nodeAgentConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(afero.WriteFile(testFs, nodeagentv1alpha1.NodeAgentConfigPath, configBytes, 0644)).To(Succeed())

		originalNodeAgentToken = "original-node-agent-token"
		Expect(afero.WriteFile(testFs, nodeagentv1alpha1.NodeAgentTokenFilePath, []byte(originalNodeAgentToken), 0644)).To(Succeed())

		tokenReconciler := &token.Reconciler{
			Client: mgr.GetClient(),
			Fs:     testFs,
			Config: nodeAgentConfig,
		}
		Expect((tokenReconciler.AddToManager(mgr))).To(Succeed())

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

	It("should update the local token file when secret changes", func() {
		gotToken, err := afero.ReadFile(testFs, nodeagentv1alpha1.NodeAgentTokenFilePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(gotToken)).To(Equal(originalNodeAgentToken))

		updatedTestToken := "updated-test-token"
		Expect(afero.WriteFile(testFs, nodeagentv1alpha1.NodeAgentTokenFilePath, []byte(updatedTestToken), 0644)).To(Succeed())

		tokenSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1alpha1.NodeAgentTokenSecretName,
				Namespace: metav1.NamespaceSystem,
			},
			Data: map[string][]byte{
				nodeagentv1alpha1.NodeAgentTokenSecretKey: []byte(updatedTestToken),
			},
		}
		Expect(testClient.Create(ctx, tokenSecret)).To(Succeed())

		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, tokenSecret)).To(Succeed())
		})

		Eventually(func(g Gomega) {
			gotToken, err := afero.ReadFile(testFs, nodeagentv1alpha1.NodeAgentTokenFilePath)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(string(gotToken)).To(Equal(updatedTestToken))
		}).Should(Succeed())
	})
})
