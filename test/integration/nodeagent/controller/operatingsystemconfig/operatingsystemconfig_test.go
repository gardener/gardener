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

package operatingsystemconfig_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/executor"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	osc "github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Nodeagent Operating System Config controller tests", func() {
	var (
		testFs          afero.Fs
		fakeDbus        dbus.FakeDbus
		triggerChannels []chan event.GenericEvent
	)
	const (
		nodeName               = testID + "-node"
		nodeAgentOSCSecretName = testID + "-nodeagent-osc-secret"
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
			OSCSecretName:   nodeAgentOSCSecretName,
		}
		configBytes, err := yaml.Marshal(nodeAgentConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(afero.WriteFile(testFs, nodeagentv1alpha1.NodeAgentConfigPath, configBytes, 0644)).To(Succeed())

		originalNodeAgentToken := "original-node-agent-token"
		Expect(afero.WriteFile(testFs, nodeagentv1alpha1.NodeAgentTokenFilePath, []byte(originalNodeAgentToken), 0644)).To(Succeed())

		fakeDbus = dbus.FakeDbus{}
		triggerChannels = []chan event.GenericEvent{
			make(chan event.GenericEvent, 1),
		}
		reconciler := &osc.Reconciler{
			Client:          mgr.GetClient(),
			Fs:              testFs,
			Config:          nodeAgentConfig,
			TriggerChannels: triggerChannels,
			Dbus:            &fakeDbus,
			NodeName:        nodeName,
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

	It("should update the node agent operating system config", func() {
		By("Create node agent operating system config secret")
		fileExample := extensionsv1alpha1.File{
			Path: "/example/file",
			Content: extensionsv1alpha1.FileContent{
				Inline: &extensionsv1alpha1.FileContentInline{
					Data: "example-contents",
				},
			},
		}
		unitEnabled := extensionsv1alpha1.Unit{
			Name:    "enabled-unit",
			Enable:  pointer.BoolPtr(true),
			Command: pointer.String("start"),
			Content: pointer.String("# ENABLED"),
		}
		unitDisabled := extensionsv1alpha1.Unit{
			Name:    "disabled-unit",
			Enable:  pointer.BoolPtr(false),
			Command: pointer.String("stop"),
			Content: pointer.String("# Disabled"),
		}
		osc := extensionsv1alpha1.OperatingSystemConfig{
			Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
				Files: []extensionsv1alpha1.File{
					fileExample,
				},
				Units: []extensionsv1alpha1.Unit{
					unitEnabled,
					unitDisabled,
				},
			},
		}
		var oscRaw []byte
		oscRaw, err := yaml.Marshal(osc)
		Expect(err).NotTo(HaveOccurred())

		oscSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodeAgentOSCSecretName,
				Namespace: metav1.NamespaceSystem,
			},
			Data: map[string][]byte{
				nodeagentv1alpha1.NodeAgentOSCSecretKey: oscRaw,
			},
		}
		Expect(mgrClient.Create(ctx, oscSecret)).To(Succeed())

		By("Wait for node annotation to be updated")
		Eventually(func(g Gomega) {
			updatedNode := &corev1.Node{}
			g.Expect(mgrClient.Get(ctx, client.ObjectKey{Name: nodeName}, updatedNode)).To(Succeed())
			oscCheckSum := utils.ComputeSHA256Hex(oscRaw)
			g.Expect(updatedNode.Annotations).To(HaveKeyWithValue(executor.AnnotationKeyChecksum, oscCheckSum))
		}).Should(Succeed())

		By("Check that the node agent operating system config has been applied correctly")
		exampleFile, err := afero.ReadFile(testFs, fileExample.Path)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(exampleFile)).To(Equal(fileExample.Content.Inline.Data))

		Expect(fakeDbus.Actions).To(HaveLen(5))
		Expect(fakeDbus.Actions[0]).To(Equal(dbus.FakeSystemdAction{
			Action:    dbus.FakeEnable,
			UnitNames: []string{unitEnabled.Name},
		}))
		Expect(fakeDbus.Actions[1]).To(Equal(dbus.FakeSystemdAction{
			Action:    dbus.FakeDisable,
			UnitNames: []string{unitDisabled.Name},
		}))
		Expect(fakeDbus.Actions[2]).To(Equal(dbus.FakeSystemdAction{
			Action: dbus.FakeDeamonReload,
		}))
		Expect(fakeDbus.Actions[3]).To(Equal(dbus.FakeSystemdAction{
			Action:    dbus.FakeRestart,
			UnitNames: []string{unitEnabled.Name},
		}))
		Expect(fakeDbus.Actions[4]).To(Equal(dbus.FakeSystemdAction{
			Action:    dbus.FakeStop,
			UnitNames: []string{unitDisabled.Name},
		}))

		By("Check that all trigger channels have been notified")
		for _, triggerChannel := range triggerChannels {
			Eventually(triggerChannel).Should(Receive())
		}
	})
})
