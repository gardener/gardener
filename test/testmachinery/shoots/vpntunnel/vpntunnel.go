// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpntunnel

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/resources/templates"
	"github.com/gardener/gardener/test/utils/access"
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
		loggerParams := map[string]any{
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

		ginkgo.By("Request ServiceAccount token with cluster-admin privileges")
		serviceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.SecretNameGardener,
				Namespace: metav1.NamespaceSystem,
			},
		}
		token, err := framework.CreateTokenForServiceAccount(ctx, f.ShootClient, serviceAccount, ptr.To[int64](3600))
		framework.ExpectNoError(err)

		ginkgo.By("Get the pods matching the logging-pod label")
		pods := &corev1.PodList{}
		err = f.ShootClient.Client().List(ctx, pods, client.InNamespace(namespace), client.MatchingLabels{"app": loggerAppLabel})
		framework.ExpectNoError(err)

		ginkgo.By("Check until we get all logs from logging-pod")
		for _, pod := range pods.Items {
			log := f.Logger.WithValues("pod", client.ObjectKeyFromObject(&pod))

			i := 0
			for ; i < maxIterations; i++ {
				f.Logger.Info("Using address", "iteration", i+1, "address", f.Shoot.Status.AdvertisedAddresses[0].Name)
				stdout, _, err := f.ShootClient.PodExecutor().Execute(
					ctx,
					pod.Namespace,
					pod.Name,
					"pause",
					fmt.Sprintf("curl -k -v -XGET  -H \"Accept: application/json, */*\" -H \"Authorization: Bearer %s\" \"%s/api/v1/namespaces/%s/pods/%s/log?container=logger\"", token, f.Shoot.Status.AdvertisedAddresses[0].URL, pod.Namespace, pod.Name),
				)
				if apierrors.IsNotFound(err) {
					log.Error(err, "Aborting as pod was not found anymore")
					break
				}
				framework.ExpectNoError(err)
				scanner := bufio.NewScanner(stdout)
				counter := 0
				for scanner.Scan() {
					counter++
				}
				err = scanner.Err()
				framework.ExpectNoError(err)
				log.Info("Got lines from pod", "iteration", i+1, "count", counter)
				if counter >= logsCount {
					break
				}
				time.Sleep(1 * time.Second)
			}
			Expect(i).To(BeNumerically("<", maxIterations))
		}

	}, testTimeout, framework.WithCAfterTest(func(ctx context.Context) {
		ginkgo.By("Cleanup logging-pod resources")
		loggerDeploymentToDelete := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: namespace,
			},
		}
		err := kubernetesutils.DeleteObject(ctx, f.ShootClient.Client(), loggerDeploymentToDelete)
		framework.ExpectNoError(err)
	}, cleanupTimeout))

	f.Beta().CIt("should copy data to pod", func(ctx context.Context) {
		ginkgo.By("Request kubeconfig with cluster-admin privileges")
		kubeconfig, err := access.RequestAdminKubeconfigForShoot(ctx, f.GardenClient, f.Shoot, ptr.To[int64](3600))
		framework.ExpectNoError(err)

		testSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: copyDeployment, Namespace: namespace}}
		_, err = controllerutils.GetAndCreateOrMergePatch(ctx, f.ShootClient.Client(), testSecret, func() error {
			testSecret.Type = corev1.SecretTypeOpaque
			testSecret.Data = map[string][]byte{
				"kubeconfig": kubeconfig,
			}
			return nil
		})
		framework.ExpectNoError(err)

		ginkgo.By("Deploy the source and target pod")
		params := map[string]any{
			"Name":        copyDeployment,
			"Namespace":   namespace,
			"AppLabel":    copyLabel,
			"SizeInMB":    500,
			"KubeVersion": f.Shoot.Spec.Kubernetes.Version,
			// Here it is assumed that all worker pool of shoot have same architecture.
			"Architecture": f.Shoot.Spec.Provider.Workers[0].Machine.Architecture,
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

		for _, pod := range pods.Items {
			log := f.Logger.WithValues("pod", client.ObjectKeyFromObject(&pod))

			ginkgo.By("Copy data to target-container in pod " + pod.Name)
			stdout, _, err := f.ShootClient.PodExecutor().Execute(ctx, pod.Namespace, pod.Name, "source-container",
				"/data/kubectl", "cp", "/data/data", pod.Namespace+"/"+pod.Name+":/data/data", "-c", "target-container",
			)
			if apierrors.IsNotFound(err) {
				log.Error(err, "Aborting as pod was not found anymore")
				break
			}
			framework.ExpectNoError(err)
			output, err := io.ReadAll(stdout)
			framework.ExpectNoError(err)
			log.Info("Got output from 'kubectl cp' command", "output", string(output))
		}
	}, testTimeout, framework.WithCAfterTest(func(ctx context.Context) {
		ginkgo.By("Cleanup copy resources")
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
		err := kubernetesutils.DeleteObjects(ctx, f.ShootClient.Client(), deploymentToDelete, secretToDelete)
		framework.ExpectNoError(err)
	}, cleanupTimeout))
})
