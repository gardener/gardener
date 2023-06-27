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

package node_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	nodecontroller "github.com/gardener/gardener/pkg/nodeagent/controller/node"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
)

var _ = Describe("Nodeagent Node controller tests", func() {
	var (
		testFs   afero.Fs
		fakeDbus dbus.FakeDbus
	)
	const (
		nodeName                     = testID + "-node"
		restartSystemdUnitAnnotation = "worker.gardener.cloud/restart-systemd-services"
	)

	BeforeEach(func() {
		By("Setup manager")
		mapper, err := apiutil.NewDynamicRESTMapper(restConfig)
		Expect(err).NotTo(HaveOccurred())

		mgr, err := manager.New(restConfig, manager.Options{
			Scheme:             operatorclient.RuntimeScheme,
			MetricsBindAddress: "0",
			NewCache: cache.BuilderWithOptions(cache.Options{
				Mapper:            mapper,
				SelectorsByObject: map[client.Object]cache.ObjectSelector{},
			}),
		})
		Expect(err).NotTo(HaveOccurred())
		mgrClient = mgr.GetClient()

		By("Register controller")
		testFs = afero.NewMemMapFs()
		nodeAgentConfig := &nodeagentv1alpha1.NodeAgentConfiguration{
			TokenSecretName: nodeagentv1alpha1.NodeAgentTokenSecretName,
		}
		configBytes, err := yaml.Marshal(nodeAgentConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(afero.WriteFile(testFs, nodeagentv1alpha1.NodeAgentConfigPath, configBytes, 0644)).To(Succeed())

		originalNodeAgentToken := "original-node-agent-token"
		Expect(afero.WriteFile(testFs, nodeagentv1alpha1.NodeAgentTokenFilePath, []byte(originalNodeAgentToken), 0644)).To(Succeed())

		fakeDbus = dbus.FakeDbus{}
		reconciler := &nodecontroller.Reconciler{
			Client:   mgr.GetClient(),
			Dbus:     &fakeDbus,
			NodeName: nodeName,
		}
		Expect((reconciler.AddToManager(mgr))).To(Succeed())

		By("Create Node")
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
				Annotations: map[string]string{
					"testid": testID,
				},
			},
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Node",
			},
		}
		Expect(mgrClient.Create(ctx, node)).To(Succeed())
		DeferCleanup(func() {
			By("Delete Node")
			Expect(mgrClient.Delete(ctx, node)).To(Succeed())
		})

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

	It("should restart the node annotation's unit", func() {
		node := &corev1.Node{}
		Eventually(func(g Gomega) {
			g.Expect(mgrClient.Get(ctx, client.ObjectKey{Name: nodeName}, node)).To(Succeed())
			g.Expect(node.Name).To(Equal(nodeName))
			g.Expect(node.Annotations).To(HaveKey("testid"))
		}).Should(Succeed())

		By("Adding restart annotation to node")
		restartedServiceName := "restarted-service"
		node.Annotations[restartSystemdUnitAnnotation] = restartedServiceName
		Expect(mgrClient.Update(ctx, node)).To(Succeed())

		By("Wait for restart annotation to disappear")
		Eventually(func(g Gomega) {
			updatedNode := &corev1.Node{}
			g.Expect(mgrClient.Get(ctx, client.ObjectKey{Name: nodeName}, updatedNode)).To(Succeed())
			g.Expect(updatedNode.Annotations).NotTo(HaveKey(restartSystemdUnitAnnotation))
		}).Should(Succeed())

		By("Check that the unit was restarted")
		Expect(fakeDbus.Actions).To(HaveLen(1))
		Expect(fakeDbus.Actions[0]).To(Equal(dbus.FakeSystemdAction{
			Action:    dbus.FakeRestart,
			UnitNames: []string{restartedServiceName},
		}))
	})
})
