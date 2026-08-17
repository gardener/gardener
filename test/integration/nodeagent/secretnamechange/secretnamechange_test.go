// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretnamechange_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodeagenthelper "github.com/gardener/gardener/pkg/api/config/nodeagent/v1alpha1/helper"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/nodeagent"
)

const (
	configDir     = "/var/lib/gardener-node-agent"
	initialSecret = "cloud-config-initial"
	newSecret     = "cloud-config-new"
)

var _ = Describe("SecretNameChange controller tests", func() {
	var node *corev1.Node

	BeforeEach(func() {
		cancelCalled.Store(false)

		By("Write initial config to fake filesystem")
		config := &nodeagentconfigv1alpha1.NodeAgentConfiguration{
			Controllers: nodeagentconfigv1alpha1.ControllerConfiguration{
				OperatingSystemConfig: nodeagentconfigv1alpha1.OperatingSystemConfigControllerConfig{
					SecretName: initialSecret,
				},
			},
		}
		configRaw, err := runtime.Encode(nodeagent.Codec, config)
		Expect(err).NotTo(HaveOccurred())
		Expect(testFS.WriteFile(nodeagenthelper.GetConfigFilePath(configDir), configRaw, 0600)).To(Succeed())

		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: testRunID,
				Labels: map[string]string{
					testID: testRunID,
					v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName: initialSecret,
				},
			},
		}

		By("Create Node")
		Expect(testClient.Create(ctx, node)).To(Succeed())
		DeferCleanup(func() {
			By("Delete Node")
			Expect(testClient.Delete(ctx, node)).To(Succeed())
		})
	})

	It("should do nothing when the secret name label matches the config", func() {
		Consistently(func() bool {
			return cancelCalled.Load()
		}).Should(BeFalse())
	})

	It("should overwrite config and call cancel when the secret name label changes", func() {
		By("Update node label to new secret name")
		patch := client.MergeFrom(node.DeepCopy())
		node.Labels[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName] = newSecret
		Expect(testClient.Patch(ctx, node, patch)).To(Succeed())

		By("Wait for cancel to be called")
		Eventually(func() bool {
			return cancelCalled.Load()
		}).Should(BeTrue())

		By("Verify the config on disk was updated")
		configRaw, err := testFS.ReadFile(nodeagenthelper.GetConfigFilePath(configDir))
		Expect(err).NotTo(HaveOccurred())
		config := &nodeagentconfigv1alpha1.NodeAgentConfiguration{}
		Expect(runtime.DecodeInto(nodeagent.Codec, configRaw, config)).To(Succeed())
		Expect(config.Controllers.OperatingSystemConfig.SecretName).To(Equal(newSecret))
	})
})
