// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

/**
	Overview
		- Tests that there is no segfault in journalctl under load

	Test: deploy load to a node
	Expected Output
		- No segfault in the journalctl logs

 **/

package operatingsystem

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
	g "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/resources/templates"
)

var _ = ginkgo.Describe("Operating system testing", func() {

	f := framework.NewShootFramework(&framework.ShootConfig{
		CreateTestNamespace: true,
	})

	ginkgo.Context("OperatingSystem load", func() {

		const deploymentName = "os-loadtest"

		var rootPodExecutor framework.RootPodExecutor

		f.Default().Serial().CIt("should not segfault", func(ctx context.Context) {
			// choose random node
			nodes := &corev1.NodeList{}
			err := f.ShootClient.Client().List(ctx, nodes)
			framework.ExpectNoError(err)

			if len(nodes.Items) == 0 {
				ginkgo.Fail("at least one node is needed")
			}

			err = f.RenderAndDeployTemplate(ctx, f.ShootClient, templates.SimpleLoadDeploymentName, map[string]string{
				"name":      deploymentName,
				"namespace": f.Namespace,
				"nodeName":  nodes.Items[0].Name,
			})
			framework.ExpectNoError(err)

			err = f.WaitUntilDeploymentIsReady(ctx, deploymentName, f.Namespace, f.ShootClient)
			framework.ExpectNoError(err)

			ginkgo.By("Wait 10 seconds for the deployment to generate load")
			time.Sleep(10 * time.Second)

			// deploy root pod on the node with the load
			rootPodExecutor = framework.NewRootPodExecutor(f.Logger, f.ShootClient, &nodes.Items[0].Name, f.Namespace)

			response, err := rootPodExecutor.Execute(ctx, "journalctl", "--no-pager")
			framework.ExpectNoError(err)
			g.Expect(response).ToNot(g.BeNil())

			ginkgo.By("Expect no segfault")

			journalctlValidation := framework.TextValidation{"segfault": "expect no systemctl segfault"}
			err = journalctlValidation.ValidateAsDenylist(response)
			framework.ExpectNoError(err)

			ginkgo.By("Expect systemctl to respond")
			_, err = rootPodExecutor.Execute(ctx, "systemctl")
			framework.ExpectNoError(err)
		}, 30*time.Minute)

		framework.CAfterEach(func(ctx context.Context) {
			err := rootPodExecutor.Clean(ctx)
			framework.ExpectNoError(err)

			deployment := &appsv1.Deployment{}
			deployment.Name = deploymentName
			deployment.Namespace = f.Namespace
			err = framework.DeleteAndWaitForResource(ctx, f.ShootClient, deployment, 5*time.Minute)
			framework.ExpectNoError(err)
		}, 5*time.Minute)
	})

})
