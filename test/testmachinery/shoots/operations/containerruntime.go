// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operations

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/test/framework"
)

var _ = Describe("Shoot container runtime testing", func() {
	f := framework.NewShootFramework(nil)

	f.Default().Serial().CIt("verify containerd configuration and runtime setup", func(ctx context.Context) {
		var shoot = f.Shoot

		if v1beta1helper.IsWorkerless(shoot) {
			Skip("at least one worker pool is required in the test shoot")
		}

		// check the node labels to contain containerd label
		nodeList, err := framework.GetAllNodes(ctx, f.ShootClient)
		framework.ExpectNoError(err)

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
