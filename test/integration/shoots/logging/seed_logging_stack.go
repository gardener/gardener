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

package logging

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/resources/templates"

	"github.com/onsi/ginkgo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

const (
	deltaLogsCount            = 1
	deltaLogsDuration         = "180s"
	logsCount                 = 2000
	logsDuration              = "90s"
	numberOfSimulatedClusters = 100

	initializationTimeout          = 5 * time.Minute
	getLogsFromLokiTimeout         = 15 * time.Minute
	loggerDeploymentCleanupTimeout = 5 * time.Minute

	fluentBitName                 = "fluent-bit"
	lokiName                      = "loki"
	garden                        = "garden"
	loggerDeploymentName          = "logger"
	logger                        = "logger-.*"
	fluentBitClusterRoleName      = "fluent-bit-read"
	simulatesShootNamespacePrefix = "shoot--logging--test-"
)

var _ = ginkgo.Describe("Seed logging testing", func() {
	var (
		f                           = framework.NewShootFramework(nil)
		gardenNamespace             = &corev1.Namespace{}
		fluentBit                   = &appsv1.DaemonSet{}
		fluentBitConfMap            = &corev1.ConfigMap{}
		fluentBitService            = &corev1.Service{}
		fluentBitClusterRole        = &rbacv1.ClusterRole{}
		fluentBitClusterRoleBinding = &rbacv1.ClusterRoleBinding{}
		fluentBitServiceAccount     = &corev1.ServiceAccount{}
		fluentBitPriorityClass      = &schedulingv1.PriorityClass{}
		clusterCRD                  = &apiextensionsv1.CustomResourceDefinition{}
		lokiSts                     = &appsv1.StatefulSet{}
		lokiServiceAccount          = &corev1.ServiceAccount{}
		lokiService                 = &corev1.Service{}
		lokiConfMap                 = &corev1.ConfigMap{}
		lokiPriorityClass           = &schedulingv1.PriorityClass{}
		// This shoot is used as seed for this integration test only
		shootClient     kubernetes.Interface
		shootLokiLabels = map[string]string{
			"app":  "loki-shoot",
			"role": "logging",
		}
		gardenLokiLabels = map[string]string{
			"app":  "loki",
			"role": "logging",
		}
		lokiShootService *corev1.Service
		lokiShootSts     *appsv1.StatefulSet
	)

	framework.CBeforeEach(func(ctx context.Context) {
		var err error
		checkRequiredResources(ctx, f.SeedClient)
		shootClient, err = kubernetes.NewClientFromSecret(ctx, f.SeedClient.Client(), framework.ComputeTechnicalID(f.Project.Name, f.Shoot), gardencorev1beta1.GardenerName, kubernetes.WithClientOptions(client.Options{
			Scheme: kubernetes.SeedScheme,
		}))
		framework.ExpectNoError(err)
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: fluentBitName}, fluentBit))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: getConfigMapName(fluentBit.Spec.Template.Spec.Volumes, "template-config")}, fluentBitConfMap))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: fluentBitName}, fluentBitService))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: fluentBitClusterRoleName}, fluentBitClusterRole))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: fluentBitClusterRoleName}, fluentBitClusterRoleBinding))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: fluentBitName}, fluentBitServiceAccount))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: fluentBitName}, fluentBitPriorityClass))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: "", Name: "clusters.extensions.gardener.cloud"}, clusterCRD))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: lokiName}, lokiSts))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: lokiName}, lokiServiceAccount))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: lokiName}, lokiService))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: getConfigMapName(lokiSts.Spec.Template.Spec.Volumes, "config")}, lokiConfMap))
		lokiPriorityClassName := lokiSts.Spec.Template.Spec.PriorityClassName
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: lokiPriorityClassName}, lokiPriorityClass))
	}, initializationTimeout)

	f.Beta().Serial().CIt("should get container logs from loki for all namespaces", func(ctx context.Context) {
		var (
			emptyDirSize             = "500Mi"
			lokiPersistentVolumeName = "loki"
		)
		// Dedicated Loki for all Shoot logs
		lokiShootService = lokiService.DeepCopy()
		lokiShootService.Name = "loki-shoots"
		lokiShootService.Spec.Selector = shootLokiLabels
		lokiShootService.Spec.ClusterIP = ""
		lokiShootSts = lokiSts.DeepCopy()
		lokiShootSts.Name = "loki-shoots"
		lokiShootSts.Labels = shootLokiLabels
		lokiShootSts.Spec.Selector.MatchLabels = shootLokiLabels
		lokiShootSts.Spec.Template.Labels = shootLokiLabels

		ginkgo.By("Deploy the garden Namespace")
		gardenNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: v1beta1constants.GardenNamespace,
			},
		}
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), gardenNamespace))

		ginkgo.By("Deploy the Loki StatefulSet")
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), lokiServiceAccount))
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), lokiConfMap))
		// Remove the cluster IP because it could be already in use
		lokiService.Spec.ClusterIP = ""
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), lokiService))
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), lokiPriorityClass))
		// Remove the Loki PVC as it is no needed for the test
		lokiSts.Spec.VolumeClaimTemplates = nil
		// Instead use an empty dir volume
		lokiSts.Spec.Template.Spec.Volumes = append(lokiSts.Spec.Template.Spec.Volumes, newEmptyDirVolume(lokiPersistentVolumeName, emptyDirSize))
		// Spread the loki instances on separate nodes to let deploying of loggers afterwards to be relatively equal through the nodes
		lokiSts.Spec.Template.Spec.Affinity = &corev1.Affinity{PodAntiAffinity: newPodAntiAffinity(shootLokiLabels)}
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), lokiSts))

		ginkgo.By("Deploy the Loki StatefulSet for Shoot namespaces")
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), lokiShootService))
		// The same procedure as above for the Loki used for the simulated shoots
		lokiShootSts.Spec.VolumeClaimTemplates = nil
		lokiShootSts.Spec.Template.Spec.Volumes = append(lokiShootSts.Spec.Template.Spec.Volumes, newEmptyDirVolume(lokiPersistentVolumeName, emptyDirSize))
		lokiShootSts.Spec.Template.Spec.Affinity = &corev1.Affinity{PodAntiAffinity: newPodAntiAffinity(gardenLokiLabels)}
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), lokiShootSts))

		ginkgo.By("Wait until Loki StatefulSet for Garden namespace is ready")
		framework.ExpectNoError(f.WaitUntilStatefulSetIsRunning(ctx, lokiSts.Name, v1beta1constants.GardenNamespace, f.ShootClient))

		ginkgo.By("Wait until Loki StatefulSet for Shoot namespaces is ready")
		framework.ExpectNoError(f.WaitUntilStatefulSetIsRunning(ctx, lokiShootSts.Name, v1beta1constants.GardenNamespace, f.ShootClient))

		ginkgo.By("Deploy the cluster CRD")
		clusterCRD.Spec.PreserveUnknownFields = false
		for version := range clusterCRD.Spec.Versions {
			clusterCRD.Spec.Versions[version].Schema.OpenAPIV3Schema.XPreserveUnknownFields = pointer.Bool(true)
		}
		framework.ExpectNoError(create(ctx, shootClient.Client(), clusterCRD))

		ginkgo.By("Deploy the fluent-bit RBAC")
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), fluentBitServiceAccount))
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), fluentBitPriorityClass))
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), fluentBitClusterRole))
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), fluentBitClusterRoleBinding))
		framework.ExpectNoError(f.RenderAndDeployTemplate(ctx, f.ShootClient, "fluent-bit-psp-clusterrolebinding.yaml", nil))

		ginkgo.By("Deploy the fluent-bit DaemonSet")
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), fluentBitConfMap))
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), fluentBit))

		ginkgo.By("Wait until fluent-bit DaemonSet is ready")
		framework.ExpectNoError(f.WaitUntilDaemonSetIsRunning(ctx, f.ShootClient.Client(), fluentBitName, v1beta1constants.GardenNamespace))

		ginkgo.By("Deploy the simulated cluster and shoot controlplane namespaces")
		nodeList := &corev1.NodeList{}
		framework.ExpectNoError(f.ShootClient.Client().List(ctx, nodeList))

		for i := 0; i < numberOfSimulatedClusters; i++ {
			shootNamespace := getShootNamesapce(i)
			ginkgo.By(fmt.Sprintf("Deploy namespace %s", shootNamespace.Name))
			framework.ExpectNoError(create(ctx, f.ShootClient.Client(), shootNamespace))

			cluster := getCluster(i)
			ginkgo.By(fmt.Sprintf("Deploy cluster %s", cluster.Name))
			framework.ExpectNoError(create(ctx, shootClient.Client(), cluster))

			ginkgo.By(fmt.Sprintf("Deploy the loki service in namespace %s", shootNamespace.Name))
			lokiShootService := getLokiShootService(i)
			framework.ExpectNoError(create(ctx, f.ShootClient.Client(), lokiShootService))

			ginkgo.By(fmt.Sprintf("Deploy the logger application in namespace %s", shootNamespace.Name))
			loggerParams := map[string]interface{}{
				"LoggerName":          loggerDeploymentName,
				"HelmDeployNamespace": shootNamespace.Name,
				"AppLabel":            loggerDeploymentName,
				"DeltaLogsCount":      deltaLogsCount,
				"DeltaLogsDuration":   deltaLogsDuration,
				"LogsCount":           logsCount,
				"LogsDuration":        logsDuration,
			}

			if len(nodeList.Items) > 0 {
				loggerParams["NodeName"] = nodeList.Items[i%len(nodeList.Items)].Name
			}

			framework.ExpectNoError(f.RenderAndDeployTemplate(ctx, f.ShootClient, templates.LoggerAppName, loggerParams))
		}

		loggerLabels := labels.SelectorFromSet(map[string]string{
			"app": "logger",
		})
		for i := 0; i < numberOfSimulatedClusters; i++ {
			shootNamespace := fmt.Sprintf("%s%v", simulatesShootNamespacePrefix, i)
			ginkgo.By(fmt.Sprintf("Wait until logger application is ready in namespace %s", shootNamespace))
			framework.ExpectNoError(f.WaitUntilDeploymentsWithLabelsIsReady(ctx, loggerLabels, shootNamespace, f.ShootClient))
		}

		ginkgo.By("Verify loki received logger application logs for all shoot namespaces")
		framework.ExpectNoError(WaitUntilLokiReceivesLogs(ctx, 30*time.Second, f, shootLokiLabels, "", v1beta1constants.GardenNamespace, "pod_name", logger, logsCount*numberOfSimulatedClusters, numberOfSimulatedClusters, f.ShootClient))

		ginkgo.By("Verify loki received logger application logs for garden namespace")
		framework.ExpectNoError(WaitUntilLokiReceivesLogs(ctx, 30*time.Second, f, gardenLokiLabels, "", v1beta1constants.GardenNamespace, "pod_name", logger, logsCount*numberOfSimulatedClusters, numberOfSimulatedClusters, f.ShootClient))

	}, getLogsFromLokiTimeout, framework.WithCAfterTest(func(ctx context.Context) {
		ginkgo.By("Cleaning up logger app resources")
		for i := 0; i < numberOfSimulatedClusters; i++ {
			shootNamespace := getShootNamesapce(i)
			loggerDeploymentToDelete := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: shootNamespace.Name,
					Name:      "logger",
				},
			}
			framework.ExpectNoError(kutil.DeleteObject(ctx, f.ShootClient.Client(), loggerDeploymentToDelete))

			cluster := getCluster(i)
			framework.ExpectNoError(kutil.DeleteObject(ctx, shootClient.Client(), cluster))

			lokiShootService := getLokiShootService(i)
			framework.ExpectNoError(kutil.DeleteObject(ctx, f.ShootClient.Client(), lokiShootService))

			framework.ExpectNoError(kutil.DeleteObject(ctx, f.ShootClient.Client(), shootNamespace))
		}

		ginkgo.By("Cleaning up garden namespace")
		objectsToDelete := []client.Object{
			fluentBit,
			fluentBitConfMap,
			fluentBitService,
			fluentBitClusterRole,
			fluentBitClusterRoleBinding,
			fluentBitServiceAccount,
			fluentBitPriorityClass,
			lokiSts,
			lokiServiceAccount,
			lokiService,
			lokiConfMap,
			lokiPriorityClass,
			clusterCRD,
			lokiShootService,
			lokiShootSts,
			gardenNamespace,
		}
		for _, object := range objectsToDelete {
			framework.ExpectNoError(kutil.DeleteObject(ctx, f.ShootClient.Client(), object))
		}
	}, loggerDeploymentCleanupTimeout))
})
