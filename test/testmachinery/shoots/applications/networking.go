// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

/**
	Overview
		- Tests the communication between all nodes of the shoot

	AfterSuite
		- Cleanup Workload in Shoot

	Test: Create a nginx daemonset and test if it is reachable from each node.
	Expected Output
		- nginx's are reachable from each node
 **/

package applications

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/resources/templates"
)

const (
	networkTestTimeout = 30 * time.Minute
	cleanupTimeout     = 2 * time.Minute
)

var _ = ginkgo.Describe("Shoot network testing", func() {

	f := framework.NewShootFramework(&framework.ShootConfig{
		CreateTestNamespace: true,
	})

	var (
		name = "net-test"
	)

	f.Default().CIt("should reach all webservers on all nodes", func(ctx context.Context) {
		templateParams := map[string]string{
			"name":      name,
			"namespace": f.Namespace,
		}
		ginkgo.By("Deploy the net test daemon set")
		framework.ExpectNoError(f.RenderAndDeployTemplate(ctx, f.ShootClient, "network-nginx-serviceaccount.yaml.tpl", templateParams))
		framework.ExpectNoError(f.RenderAndDeployTemplate(ctx, f.ShootClient, templates.NginxDaemonSetName, templateParams))

		err := f.WaitUntilDaemonSetIsRunning(ctx, f.ShootClient.Client(), name, f.Namespace)
		framework.ExpectNoError(err)

		pods := &corev1.PodList{}
		err = f.ShootClient.Client().List(ctx, pods, client.InNamespace(f.Namespace), client.MatchingLabels{"app": "net-nginx"})
		framework.ExpectNoError(err)

		// check if all webservers can be reached from all nodes
		ginkgo.By("Check connectivity to webservers")
		var allErrs error
		for _, from := range pods.Items {
			for _, to := range pods.Items {
				ginkgo.By(fmt.Sprintf("Testing %s to %s", from.GetName(), to.GetName()))
				stdout, _, err := f.ShootClient.PodExecutor().Execute(ctx, from.Namespace, from.Name, "pause",
					"curl", "-L", to.Status.PodIP+":80", "--fail", "-m", "10",
				)
				if err != nil {
					allErrs = multierror.Append(allErrs, fmt.Errorf("%s to %s: %w", from.GetName(), to.GetName(), err))
					continue
				}
				data, err := io.ReadAll(stdout)
				if err != nil {
					allErrs = multierror.Append(allErrs, fmt.Errorf("cannot to read the command output: %w", err))
					continue
				}
				f.Logger.Info("Executing curl command from one pod to another", "from", from.GetName(), "to", to.GetName(), "data", data)
			}
		}
		framework.ExpectNoError(allErrs)
	}, networkTestTimeout, framework.WithCAfterTest(func(ctx context.Context) {
		ginkgo.By("Cleanup network test daemonset")
		err := f.ShootClient.Client().Delete(ctx, &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: f.Namespace}})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				framework.ExpectNoError(err)
			}
		}
	}, cleanupTimeout))

})
