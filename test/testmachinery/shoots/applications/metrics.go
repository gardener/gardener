// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

/**
	Overview
		- Tests that runtime metrics are available

	AfterSuite
		- Cleanup Workload in Shoot

	Test: Create arbitrary deployment
	Expected Output
		- Metrics are available
 **/

package applications

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/resources/templates"
)

const (
	metricsTestTimeout = 10 * time.Minute
)

var _ = ginkgo.Describe("Shoot application metrics testing", func() {

	f := framework.NewShootFramework(&framework.ShootConfig{
		CreateTestNamespace: true,
	})

	var (
		name = "metrics-test"
	)

	f.Default().CIt("should read runtime metrics", func(ctx context.Context) {
		templateParams := map[string]string{
			"name":      name,
			"namespace": f.Namespace,
		}
		err := f.RenderAndDeployTemplate(ctx, f.ShootClient, templates.SimpleLoadDeploymentName, templateParams)
		framework.ExpectNoError(err)

		err = f.WaitUntilDeploymentIsReady(ctx, name, f.Namespace, f.ShootClient)
		framework.ExpectNoError(err)

		pods := &corev1.PodList{}
		err = f.ShootClient.Client().List(ctx, pods, client.InNamespace(f.Namespace), client.MatchingLabels{"app": "load"})
		framework.ExpectNoError(err)

		if len(pods.Items) == 0 {
			ginkgo.Fail("at least one pod is needed")
		}
		podName := pods.Items[0].Name

		ginkgo.By("Check runtime metrics")
		framework.ExpectNoError(
			retry.Until(ctx, 30*time.Second, func(ctx context.Context) (bool, error) {
				podMetrics := &metricsv1beta1.PodMetrics{}
				if err := f.ShootClient.Client().Get(ctx, kubernetesutils.Key(f.Namespace, podName), podMetrics); err != nil {
					if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) || apierrors.IsServiceUnavailable(err) {
						f.Logger.Error(err, "No metrics for pod available yet", "pod", client.ObjectKeyFromObject(podMetrics))
						return retry.MinorError(err)
					}
					return retry.SevereError(err)
				}

				if len(podMetrics.Containers) == 0 {
					return retry.MinorError(errors.New("no metrics recorded yet"))
				}

				for _, container := range podMetrics.Containers {
					if container.Usage.Cpu() == nil {
						return retry.MinorError(fmt.Errorf("no CPU metrics recorded yet for container %q", container.Name))
					}
					if container.Usage.Memory() == nil {
						return retry.MinorError(fmt.Errorf("no Memory metrics recorded yet for container %q", container.Name))
					}
				}
				return retry.Ok()
			}),
		)
	}, metricsTestTimeout, framework.WithCAfterTest(func(ctx context.Context) {
		ginkgo.By("Cleanup metrics test deployment")
		err := f.ShootClient.Client().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: f.Namespace}})
		framework.ExpectNoError(client.IgnoreNotFound(err))
	}, cleanupTimeout))

})
