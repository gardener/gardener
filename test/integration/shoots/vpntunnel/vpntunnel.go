// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package vpntunnel

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gardener/gardener/pkg/controllerutils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/resources/templates"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	deploymentName = "logging-pod"
	namespace      = metav1.NamespaceDefault
	logsCount      = 100000
	logsDuration   = "1s"
	loggerAppLabel = "vpnTunnelTesting"
	testTimeout    = 5 * time.Minute
	cleanupTimeout = 5 * time.Minute
	maxIterations  = 300
	copyDeployment = "copy-pod"
	copyLabel      = "vpnTunnelCopyTesting"
)

var _ = ginkgo.Describe("Shoot vpn tunnel testing", func() {

	f := framework.NewShootFramework(nil)

	f.Beta().CIt("should get container logs from logging-pod", func(ctx context.Context) {

		ginkgo.By("Deploy the logging-pod")
		loggerParams := map[string]interface{}{
			"LoggerName":          deploymentName,
			"HelmDeployNamespace": namespace,
			"AppLabel":            loggerAppLabel,
			"LogsCount":           logsCount,
			"LogsDuration":        logsDuration,
		}

		err := f.RenderAndDeployTemplate(ctx, f.ShootClient, templates.VPNTunnelDeploymentName, loggerParams)
		framework.ExpectNoError(err)

		ginkgo.By("Wait until logging-pod application is ready")
		loggerLabels := labels.SelectorFromSet(labels.Set(map[string]string{
			"app": loggerAppLabel,
		}))

		err = f.WaitUntilDeploymentsWithLabelsIsReady(ctx, loggerLabels, namespace, f.ShootClient)
		framework.ExpectNoError(err)

		ginkgo.By("Get kubeconfig and extract token")
		data, err := framework.GetObjectFromSecret(ctx, f.SeedClient, f.ShootSeedNamespace(), "kubecfg", framework.KubeconfigSecretKeyName)
		framework.ExpectNoError(err)
		lines := strings.Split(string(data), "\n")
		var token string
		for _, line := range lines {
			index := strings.Index(line, "token:")
			if index >= 0 {
				token = strings.TrimSpace(line[index+len("token:"):])
			}
		}

		ginkgo.By("Get the pods matching the logging-pod label")
		pods := &corev1.PodList{}
		err = f.ShootClient.Client().List(ctx, pods, client.InNamespace(namespace), client.MatchingLabels{"app": loggerAppLabel})
		framework.ExpectNoError(err)

		ginkgo.By("Check until we get all logs from logging-pod")
		podExecutor := framework.NewPodExecutor(f.ShootClient)
		for _, pod := range pods.Items {
			i := 0
			for ; i < maxIterations; i++ {
				f.Logger.Infof("Using %s address for %d. iteration", f.Shoot.Status.AdvertisedAddresses[0].Name, i+1)
				reader, err := podExecutor.Execute(ctx, pod.Namespace, pod.Name, "net-curl", fmt.Sprintf("curl -k -v -XGET  -H \"Accept: application/json, */*\" -H \"Authorization: Bearer %s\" \"%s/api/v1/namespaces/%s/pods/%s/log?container=logger\"", token, f.Shoot.Status.AdvertisedAddresses[0].URL, pod.Namespace, pod.Name))
				if apierrors.IsNotFound(err) {
					f.Logger.Infof("Aborting as pod %s was not found anymore: %s", pod.Name, err)
					break
				}
				framework.ExpectNoError(err)
				scanner := bufio.NewScanner(reader)
				counter := 0
				for scanner.Scan() {
					counter++
				}
				err = scanner.Err()
				framework.ExpectNoError(err)
				f.Logger.Infof("Got %d lines from pod %s in %d. iteration", counter, pod.Name, i+1)
				if counter >= logsCount {
					break
				}
				time.Sleep(1 * time.Second)
			}
			Expect(i < maxIterations).To(Equal(true))
		}

	}, testTimeout, framework.WithCAfterTest(func(ctx context.Context) {
		ginkgo.By("Cleaning up logging-pod resources")
		loggerDeploymentToDelete := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: namespace,
			},
		}
		err := kutil.DeleteObject(ctx, f.ShootClient.Client(), loggerDeploymentToDelete)
		framework.ExpectNoError(err)
	}, cleanupTimeout))

	f.Beta().CIt("should copy data to pod", func(ctx context.Context) {

		ginkgo.By("Get kubeconfig from shoot controlplane")
		kubeCfgSecret := &corev1.Secret{}
		err := f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: f.ShootSeedNamespace(), Name: "kubecfg"}, kubeCfgSecret)
		framework.ExpectNoError(err)

		testSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: copyDeployment, Namespace: namespace}}
		_, err = controllerutils.GetAndCreateOrMergePatch(ctx, f.ShootClient.Client(), testSecret, func() error {
			testSecret.Type = corev1.SecretTypeOpaque
			testSecret.Data = kubeCfgSecret.Data
			return nil
		})
		framework.ExpectNoError(err)

		ginkgo.By("Deploy the source and target pod")
		params := map[string]interface{}{
			"Name":        copyDeployment,
			"Namespace":   namespace,
			"AppLabel":    copyLabel,
			"SizeInMB":    500,
			"KubeVersion": f.Shoot.Spec.Kubernetes.Version,
		}

		err = f.RenderAndDeployTemplate(ctx, f.ShootClient, templates.VPNTunnelCopyDeploymentName, params)
		framework.ExpectNoError(err)

		ginkgo.By("Wait until pod is ready")
		labels := labels.SelectorFromSet(labels.Set(map[string]string{
			"app": copyLabel,
		}))

		err = f.WaitUntilDeploymentsWithLabelsIsReady(ctx, labels, namespace, f.ShootClient)
		framework.ExpectNoError(err)

		ginkgo.By("Get the pods matching the label")
		pods := &corev1.PodList{}
		err = f.ShootClient.Client().List(ctx, pods, client.InNamespace(namespace), client.MatchingLabels{"app": copyLabel})
		framework.ExpectNoError(err)

		podExecutor := framework.NewPodExecutor(f.ShootClient)
		for _, pod := range pods.Items {
			ginkgo.By(fmt.Sprintf("Copy data to target-container in pod %s", pod.Name))
			reader, err := podExecutor.Execute(ctx, pod.Namespace, pod.Name, "source-container", fmt.Sprintf("/data/kubectl cp /data/data %s/%s:/data/data -c target-container", pod.Namespace, pod.Name))
			if apierrors.IsNotFound(err) {
				f.Logger.Infof("Aborting as pod %s was not found anymore: %s", pod.Name, err)
				break
			}
			framework.ExpectNoError(err)
			output, err := io.ReadAll(reader)
			framework.ExpectNoError(err)
			f.Logger.Infof("Got output from 'kubectl cp': %s", string(output))
		}
	}, testTimeout, framework.WithCAfterTest(func(ctx context.Context) {
		ginkgo.By("Cleaning up copy resources")
		deploymentToDelete := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      copyDeployment,
				Namespace: namespace,
			},
		}
		secretToDelete := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      copyDeployment,
				Namespace: namespace,
			},
		}
		err := kutil.DeleteObjects(ctx, f.ShootClient.Client(), deploymentToDelete, secretToDelete)
		framework.ExpectNoError(err)
	}, cleanupTimeout))
})
