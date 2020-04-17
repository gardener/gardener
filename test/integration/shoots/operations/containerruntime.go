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

package operations

import (
	"context"
	"fmt"
	"io/ioutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/test/framework"
	"github.com/onsi/ginkgo"
	g "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Shoot container runtime testing", func() {

	f := framework.NewShootFramework(nil)

	f.Beta().Serial().CIt("should add worker pool with containerd", func(ctx context.Context) {
		shoot := f.Shoot

		if len(shoot.Spec.Provider.Workers) == 0 {
			ginkgo.Skip("at least one worker pool is required in the test shoot.")
		}

		containerdWorker := shoot.Spec.Provider.Workers[0].DeepCopy()

		allowedCharacters := "0123456789abcdefghijklmnopqrstuvwxyz"
		id, err := gardenerutils.GenerateRandomStringFromCharset(3, allowedCharacters)
		framework.ExpectNoError(err)

		containerdWorker.Name = fmt.Sprintf("test-%s", id)
		containerdWorker.Maximum = 1
		containerdWorker.Minimum = 1
		containerdWorker.CRI = &gardencorev1beta1.CRI{
			Name:              extensionsv1alpha1.CRINameContainerD,
			ContainerRuntimes: nil,
		}

		shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, *containerdWorker)

		ginkgo.By("adding containerd worker pool")

		defer func(ctx context.Context, workerPoolName string) {
			ginkgo.By("removing containerd worker pool after test execution")
			err := f.UpdateShoot(ctx, func(s *gardencorev1beta1.Shoot) error {
				var workers []gardencorev1beta1.Worker
				for _, worker := range s.Spec.Provider.Workers {
					if worker.Name == workerPoolName {
						continue
					}
					workers = append(workers, worker)
				}
				s.Spec.Provider.Workers = workers
				return nil
			})
			framework.ExpectNoError(err)
		}(ctx, containerdWorker.Name)

		err = f.UpdateShoot(ctx, func(s *gardencorev1beta1.Shoot) error {
			s.Spec.Provider.Workers = shoot.Spec.Provider.Workers
			return nil
		})
		framework.ExpectNoError(err)

		// check the node labels of the worker pool to contain containerd label
		nodeList, err := framework.GetAllNodesInWorkerPool(ctx, f.ShootClient, &containerdWorker.Name)
		framework.ExpectNoError(err)
		g.Expect(len(nodeList.Items)).To(g.Equal(int(containerdWorker.Minimum)))

		for _, node := range nodeList.Items {
			value, found := node.Labels[extensionsv1alpha1.CRINameWorkerLabel]
			g.Expect(found).To(g.BeTrue())
			g.Expect(value).To(g.Equal(extensionsv1alpha1.CRINameContainerD))
		}

		// deploy root pod
		rootPod, err := framework.DeployRootPod(ctx, f.ShootClient.Client(), "kube-system", &nodeList.Items[0].Name)
		framework.ExpectNoError(err)

		// wait until pod is running
		framework.ExpectNoError(err)
		err = f.WaitUntilPodIsRunning(ctx, rootPod.Name, rootPod.Namespace, f.ShootClient)
		framework.ExpectNoError(err)

		// check the configuration on the host
		initializerServiceCommand := fmt.Sprintf("systemctl is-active %s", "containerd-initializer")
		executeCommand(ctx, f.ShootClient, rootPod.Namespace, rootPod.Name, rootPod.Spec.Containers[0].Name, initializerServiceCommand, "active")

		containerdServiceCommand := fmt.Sprintf("systemctl is-active %s", "containerd")
		executeCommand(ctx, f.ShootClient, rootPod.Namespace, rootPod.Name, rootPod.Spec.Containers[0].Name, containerdServiceCommand, "active")

		// check that config.toml is configured
		checkConfigurationCommand := "cat /etc/systemd/system/containerd.service.d/11-exec_config.conf | grep 'usr/bin/containerd --config=/etc/containerd/config.toml' |  echo $?"
		executeCommand(ctx, f.ShootClient, rootPod.Namespace, rootPod.Name, rootPod.Spec.Containers[0].Name, checkConfigurationCommand, "0")

		// check that config.toml exists
		checkConfigCommand := "[ -f /etc/containerd/config.toml ] && echo 'found' || echo 'Not found'"
		executeCommand(ctx, f.ShootClient, rootPod.Namespace, rootPod.Name, rootPod.Spec.Containers[0].Name, checkConfigCommand, "found")
	}, scaleWorkerTimeout)
})

// executeCommand executes a command on the host and checks the returned result
func executeCommand(ctx context.Context, c kubernetes.Interface, namespace, name, containerName, command, expected string) {
	command = fmt.Sprintf("chroot /hostroot %s", command)
	reader, err := kubernetes.NewPodExecutor(c.RESTConfig()).Execute(ctx, namespace, name, containerName, command)
	framework.ExpectNoError(err)
	response, err := ioutil.ReadAll(reader)
	framework.ExpectNoError(err)
	g.Expect(response).ToNot(g.BeNil())
	g.Expect(string(response)).To(g.Equal(fmt.Sprintf("%s\n", expected)))
}
