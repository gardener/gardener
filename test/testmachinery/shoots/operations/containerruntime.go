// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operations

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/test/framework"
)

var _ = Describe("Shoot container runtime testing", func() {
	f := framework.NewShootFramework(nil)

	f.Default().Serial().CIt("should add worker pool with containerd", func(ctx context.Context) {
		var (
			shoot       = f.Shoot
			worker      = shoot.Spec.Provider.Workers[0]
			workerImage = worker.Machine.Image
		)

		if v1beta1helper.IsWorkerless(shoot) {
			Skip("at least one worker pool is required in the test shoot")
		}

		if !supportsContainerD(f.CloudProfile.Spec.MachineImages, workerImage) {
			message := fmt.Sprintf("machine image '%s@%s' does not support containerd", workerImage.Name, *workerImage.Version)
			Skip(message)
		}

		containerdWorker := worker.DeepCopy()

		allowedCharacters := "0123456789abcdefghijklmnopqrstuvwxyz"
		id, err := utils.GenerateRandomStringFromCharset(3, allowedCharacters)
		framework.ExpectNoError(err)

		containerdWorker.Name = "test-" + id
		containerdWorker.Maximum = 1
		containerdWorker.Minimum = 1
		containerdWorker.CRI = &gardencorev1beta1.CRI{
			Name:              gardencorev1beta1.CRINameContainerD,
			ContainerRuntimes: nil,
		}

		shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, *containerdWorker)

		By("Add containerd worker pool")

		defer func(ctx context.Context, workerPoolName string) {
			By("Remove containerd worker pool after test execution")
			err := f.UpdateShoot(ctx, func(s *gardencorev1beta1.Shoot) error {
				var workers []gardencorev1beta1.Worker
				for _, current := range s.Spec.Provider.Workers {
					if current.Name == workerPoolName {
						continue
					}
					workers = append(workers, current)
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
		Expect(nodeList.Items).To(HaveLen(int(containerdWorker.Minimum)))

		for _, node := range nodeList.Items {
			value, found := node.Labels[extensionsv1alpha1.CRINameWorkerLabel]
			Expect(found).To(BeTrue())
			Expect(value).To(Equal(string(extensionsv1alpha1.CRINameContainerD)))
		}

		// deploy root pod
		rootPodExecutor := framework.NewRootPodExecutor(f.Logger, f.ShootClient, &nodeList.Items[0].Name, "kube-system")

		// check the configuration on the host
		containerdServiceCommand := []string{"systemctl", "is-active", "containerd"}
		executeCommand(ctx, f, rootPodExecutor, containerdServiceCommand, "active")

		// check that config.toml is configured
		checkConfigurationCommand := []string{"sh", "-c", "cat /etc/systemd/system/containerd.service.d/11-exec_config.conf | grep 'usr/bin/containerd --config=/etc/containerd/config.toml' | echo $?"}
		executeCommand(ctx, f, rootPodExecutor, checkConfigurationCommand, "0")

		// check that config.toml exists
		checkConfigCommand := []string{"sh", "-c", "[ -f /etc/containerd/config.toml ] && echo 'found' || echo 'Not found'"}
		executeCommand(ctx, f, rootPodExecutor, checkConfigCommand, "found")
	}, scaleWorkerTimeout)
})

// executeCommand executes a command on the host and checks the returned result
func executeCommand(ctx context.Context, f *framework.ShootFramework, rootPodExecutor framework.RootPodExecutor, command []string, expected string) {
	response, err := rootPodExecutor.Execute(ctx, command...)
	if err != nil {
		f.Logger.Error(err, "Error executing command", "command", command, "response", response)
	}
	framework.ExpectNoError(err)
	Expect(response).ToNot(BeNil())
	Expect(string(response)).To(Equal(fmt.Sprintf("%s\n", expected)))
}

func supportsContainerD(cloudProfileImages []gardencorev1beta1.MachineImage, workerImage *gardencorev1beta1.ShootMachineImage) bool {
	var (
		cloudProfileImage *gardencorev1beta1.MachineImage
		machineVersion    *gardencorev1beta1.MachineImageVersion
	)

	for _, c := range cloudProfileImages {
		current := c
		if current.Name == workerImage.Name {
			cloudProfileImage = &current
			break
		}
	}
	if cloudProfileImage == nil {
		return false
	}

	for _, v := range cloudProfileImage.Versions {
		version := v
		if version.Version == *workerImage.Version {
			machineVersion = &version
			break
		}
	}
	if machineVersion == nil {
		return false
	}

	for _, cri := range machineVersion.CRI {
		if cri.Name == gardencorev1beta1.CRINameContainerD {
			return true
		}
	}

	return false
}
