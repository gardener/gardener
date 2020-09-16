// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
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
	"io/ioutil"
	"time"

	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/resources/templates"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/onsi/ginkgo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	networkTestTimeout = 1800 * time.Second
	cleanupTimeout     = 2 * time.Minute
)

var _ = ginkgo.Describe("Shoot network testing", func() {

	f := framework.NewShootFramework(&framework.ShootConfig{
		CreateTestNamespace: true,
	})

	var (
		name = "net-test"
	)

	f.Beta().CIt("should reach all webservers on all nodes", func(ctx context.Context) {
		templateParams := map[string]string{
			"name":      name,
			"namespace": f.Namespace,
		}
		ginkgo.By("Deploy the net test daemon set")
		err := f.RenderAndDeployTemplate(ctx, f.ShootClient, templates.NginxDaemonSetName, templateParams)
		framework.ExpectNoError(err)

		err = f.WaitUntilDaemonSetIsRunning(ctx, f.ShootClient.DirectClient(), name, f.Namespace)
		framework.ExpectNoError(err)

		pods := &corev1.PodList{}
		err = f.ShootClient.DirectClient().List(ctx, pods, client.InNamespace(f.Namespace), client.MatchingLabels{"app": "net-nginx"})
		framework.ExpectNoError(err)

		podExecutor := framework.NewPodExecutor(f.ShootClient)

		// check if all webservers can be reached from all nodes
		ginkgo.By("test connectivity to webservers")
		var res error
		for _, from := range pods.Items {
			for _, to := range pods.Items {
				ginkgo.By(fmt.Sprintf("Testing %s to %s", from.GetName(), to.GetName()))
				reader, err := podExecutor.Execute(ctx, from.Namespace, from.Name, "net-curl", fmt.Sprintf("curl -L %s:80 --fail -m 10", to.Status.PodIP))
				if err != nil {
					res = multierror.Append(res, errors.Wrapf(err, "%s to %s", from.GetName(), to.GetName()))
					continue
				}
				data, err := ioutil.ReadAll(reader)
				if err != nil {
					f.Logger.Error(err)
					continue
				}
				f.Logger.Infof("%s to %s: %s", from.GetName(), to.GetName(), data)
			}
		}
		framework.ExpectNoError(err)
	}, networkTestTimeout, framework.WithCAfterTest(func(ctx context.Context) {
		ginkgo.By("cleanup network test daemonset")
		err := f.ShootClient.DirectClient().Delete(ctx, &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: f.Namespace}})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				framework.ExpectNoError(err)
			}
		}
	}, cleanupTimeout))

})
