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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/resources/templates"

	"github.com/Masterminds/semver"
	"github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	fluentBitConfingDiskName      = "template-config"
	lokiName                      = "loki"
	lokiConfigDiskName            = "config"
	garden                        = "garden"
	fluentBitClusterRoleName      = "fluent-bit-read"
	simulatesShootNamespacePrefix = "shoot--logging--test-"
)

var _ = ginkgo.Describe("Seed logging testing", func() {
	var (
		f                           = framework.NewShootFramework(nil)
		fluentBit                   = &appsv1.DaemonSet{}
		fluentBitConfMap            = &corev1.ConfigMap{}
		fluentBitService            = &corev1.Service{}
		fluentBitClusterRole        = &rbacv1.ClusterRole{}
		fluentBitClusterRoleBinding = &rbacv1.ClusterRoleBinding{}
		fluentBitServiceAccount     = &corev1.ServiceAccount{}
		fluentBitPriorityClass      = &schedulingv1.PriorityClass{}
		clusterCRD                  = &apiextensionsv1.CustomResourceDefinition{}

		gardenLokiSts           = &appsv1.StatefulSet{}
		gardenLokiService       = &corev1.Service{}
		gardenLokiConfMap       = &corev1.ConfigMap{}
		gardenLokiPriorityClass = &schedulingv1.PriorityClass{}

		shootLokiService       = &corev1.Service{}
		shootLokiSts           = &appsv1.StatefulSet{}
		shootLokiPriorityClass = &schedulingv1.PriorityClass{}
		shootLokiConfMap       = &corev1.ConfigMap{}

		grafanaOperatorsIngress client.Object = &networkingv1.Ingress{}
		grafanaUsersIngress     client.Object = &networkingv1.Ingress{}
		// This shoot is used as seed for this test only
		shootClient     kubernetes.Interface
		shootLokiLabels = map[string]string{
			"app":  "loki-shoot",
			"role": "logging",
		}
		gardenLokiLabels = map[string]string{
			"app":  "loki",
			"role": "logging",
		}
	)

	framework.CBeforeEach(func(ctx context.Context) {
		var err error
		checkRequiredResources(ctx, f.SeedClient)
		shootClient, err = kubernetes.NewClientFromSecret(ctx, f.SeedClient.Client(), framework.ComputeTechnicalID(f.Project.Name, f.Shoot), gardencorev1beta1.GardenerName, kubernetes.WithClientOptions(client.Options{
			Scheme: kubernetes.SeedScheme,
		}))
		framework.ExpectNoError(err)
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: fluentBitName}, fluentBit))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: getConfigMapName(fluentBit.Spec.Template.Spec.Volumes, fluentBitConfingDiskName)}, fluentBitConfMap))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: fluentBitName}, fluentBitService))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: fluentBitClusterRoleName}, fluentBitClusterRole))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: fluentBitClusterRoleName}, fluentBitClusterRoleBinding))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: fluentBitName}, fluentBitServiceAccount))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: fluentBitName}, fluentBitPriorityClass))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: "", Name: "clusters.extensions.gardener.cloud"}, clusterCRD))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: lokiName}, gardenLokiSts))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: lokiName}, gardenLokiService))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: getConfigMapName(gardenLokiSts.Spec.Template.Spec.Volumes, lokiConfigDiskName)}, gardenLokiConfMap))
		lokiPriorityClassName := gardenLokiSts.Spec.Template.Spec.PriorityClassName
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: v1beta1constants.GardenNamespace, Name: lokiPriorityClassName}, gardenLokiPriorityClass))
		// Get the shoot logging components from the shoot running the test
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: f.ShootSeedNamespace(), Name: lokiName}, shootLokiService))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: f.ShootSeedNamespace(), Name: lokiName}, shootLokiSts))
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: f.ShootSeedNamespace(), Name: getConfigMapName(shootLokiSts.Spec.Template.Spec.Volumes, lokiConfigDiskName)}, shootLokiConfMap))
		shootLokiPriorityClassName := shootLokiSts.Spec.Template.Spec.PriorityClassName
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: f.ShootSeedNamespace(), Name: shootLokiPriorityClassName}, shootLokiPriorityClass))
		kubernetesVersion, err := semver.NewVersion(f.Shoot.Spec.Kubernetes.Version)
		framework.ExpectNoError(err)
		if versionutils.ConstraintK8sLess119.Check(kubernetesVersion) {
			grafanaOperatorsIngress = &extensionsv1beta1.Ingress{}
			grafanaUsersIngress = &extensionsv1beta1.Ingress{}
		}
		// Get the grafana-operators Ingress
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: f.ShootSeedNamespace(), Name: v1beta1constants.DeploymentNameGrafanaOperators}, grafanaOperatorsIngress))
		// Get the grafana-users Ingress
		framework.ExpectNoError(f.SeedClient.Client().Get(ctx, types.NamespacedName{Namespace: f.ShootSeedNamespace(), Name: v1beta1constants.DeploymentNameGrafanaUsers}, grafanaUsersIngress))
	}, initializationTimeout)

	f.Beta().Serial().CIt("should get container logs from loki for all namespaces", func(ctx context.Context) {
		const (
			emptyDirSize             = "500Mi"
			lokiPersistentVolumeName = "loki"
			shootLokiName            = "loki-shoots"
			userLoggerName           = "kube-apiserver"
			operatorLoggerName       = "logger"
			userLoggerRegex          = userLoggerName + "-.*"
			operatorLoggerRegex      = operatorLoggerName + "-.*"
		)
		var (
			operatorLoggerLabels = labels.SelectorFromSet(map[string]string{
				"app": operatorLoggerName,
			})

			shootLoggerLabels = labels.SelectorFromSet(map[string]string{
				"app": userLoggerName,
			})
		)

		ginkgo.By("Get Loki tenant IDs")
		userID := getXScopeOrgID(grafanaUsersIngress.GetAnnotations())
		operatorID := getXScopeOrgID(grafanaOperatorsIngress.GetAnnotations())

		ginkgo.By("Deploy the garden Namespace")
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), newGardenNamespace(v1beta1constants.GardenNamespace)))

		ginkgo.By("Deploy the Loki StatefulSet for Garden namespace")
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), gardenLokiConfMap))
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), prepareGardenLokiService(gardenLokiService)))
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), gardenLokiPriorityClass))
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), prepareGardenLokiStatefulSet(gardenLokiSts, lokiPersistentVolumeName, emptyDirSize, shootLokiLabels)))

		ginkgo.By("Deploy the Loki StatefulSet for Shoot namespaces")
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), prepareShootLokiService(shootLokiService, shootLokiName, shootLokiLabels)))
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), shootLokiPriorityClass))
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), prepareShootLokiConfigMap(shootLokiConfMap)))
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), prepareShootLokiStatefulSet(shootLokiSts, gardenLokiSts, shootLokiName, shootLokiConfMap.Name, lokiPersistentVolumeName, emptyDirSize, shootLokiLabels, gardenLokiLabels)))

		ginkgo.By("Wait until Loki StatefulSet for Garden namespace is ready")
		framework.ExpectNoError(f.WaitUntilStatefulSetIsRunning(ctx, gardenLokiSts.Name, v1beta1constants.GardenNamespace, f.ShootClient))

		ginkgo.By("Wait until Loki StatefulSet for Shoot namespaces is ready")
		framework.ExpectNoError(f.WaitUntilStatefulSetIsRunning(ctx, shootLokiSts.Name, v1beta1constants.GardenNamespace, f.ShootClient))

		ginkgo.By("Deploy the cluster CRD")
		framework.ExpectNoError(create(ctx, shootClient.Client(), prepareClusterCRD(clusterCRD)))

		ginkgo.By("Deploy the fluent-bit RBAC")
		framework.ExpectNoError(create(ctx, f.ShootClient.Client(), prepareFluentBitServiceAccount(fluentBitServiceAccount)))
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
				"LoggerName":          operatorLoggerName,
				"HelmDeployNamespace": shootNamespace.Name,
				"AppLabel":            operatorLoggerName,
				"DeltaLogsCount":      deltaLogsCount,
				"DeltaLogsDuration":   deltaLogsDuration,
				"LogsCount":           logsCount,
				"LogsDuration":        logsDuration,
			}

			// Half of the shoots will produce user and operators logs.
			// The other half will produce only operator logs
			if i&1 == 1 {
				loggerParams["LoggerName"] = userLoggerName
				loggerParams["AppLabel"] = userLoggerName
			}
			// Try to distrubute the loggers even between nodes
			if len(nodeList.Items) > 0 {
				loggerParams["NodeName"] = nodeList.Items[i%len(nodeList.Items)].Name
			}

			framework.ExpectNoError(f.RenderAndDeployTemplate(ctx, f.ShootClient, templates.LoggerAppName, loggerParams))
		}

		for i := 0; i < numberOfSimulatedClusters; i++ {
			shootNamespace := fmt.Sprintf("%s%v", simulatesShootNamespacePrefix, i)
			var l labels.Selector

			if i&1 == 1 {
				l = shootLoggerLabels
			} else {
				l = operatorLoggerLabels
			}
			ginkgo.By(fmt.Sprintf("Wait until logger application is ready in namespace %s", shootNamespace))
			framework.ExpectNoError(f.WaitUntilDeploymentsWithLabelsIsReady(ctx, l, shootNamespace, f.ShootClient))
		}

		ginkgo.By("Verify loki received all operator's logs from operator's logger application for all shoot namespaces")
		framework.ExpectNoError(WaitUntilLokiReceivesLogs(ctx, 30*time.Second, f, shootLokiLabels, operatorID, v1beta1constants.GardenNamespace, "pod_name", operatorLoggerRegex, (logsCount*numberOfSimulatedClusters)/2, numberOfSimulatedClusters/2, f.ShootClient))
		ginkgo.By("Verify loki received all operator's logs from user's logger application for all shoot namespaces")
		framework.ExpectNoError(WaitUntilLokiReceivesLogs(ctx, 30*time.Second, f, shootLokiLabels, operatorID, v1beta1constants.GardenNamespace, "pod_name", userLoggerRegex, (logsCount*numberOfSimulatedClusters)/2, numberOfSimulatedClusters/2, f.ShootClient))
		ginkgo.By("Verify loki received user logger application logs for all shoot namespaces")
		framework.ExpectNoError(WaitUntilLokiReceivesLogs(ctx, 30*time.Second, f, shootLokiLabels, userID, v1beta1constants.GardenNamespace, "pod_name", userLoggerRegex, (logsCount*numberOfSimulatedClusters)/2, numberOfSimulatedClusters/2, f.ShootClient))
		ginkgo.By("Verify loki didn't get the logs from the operator's application as user's logs for all shoot namespaces")
		framework.ExpectNoError(WaitUntilLokiReceivesLogs(ctx, 30*time.Second, f, shootLokiLabels, userID, v1beta1constants.GardenNamespace, "pod_name", operatorLoggerRegex, 0, 0, f.ShootClient))

		ginkgo.By("Verify loki received logger application logs for garden namespace")
		framework.ExpectNoError(WaitUntilLokiReceivesLogs(ctx, 30*time.Second, f, gardenLokiLabels, "", v1beta1constants.GardenNamespace, "pod_name", operatorLoggerRegex, (logsCount*numberOfSimulatedClusters)/2, numberOfSimulatedClusters/2, f.ShootClient))
		ginkgo.By("Verify loki received user application logs for garden namespace")
		framework.ExpectNoError(WaitUntilLokiReceivesLogs(ctx, 30*time.Second, f, gardenLokiLabels, "", v1beta1constants.GardenNamespace, "pod_name", userLoggerRegex, (logsCount*numberOfSimulatedClusters)/2, numberOfSimulatedClusters/2, f.ShootClient))

	}, getLogsFromLokiTimeout, framework.WithCAfterTest(func(ctx context.Context) {
		ginkgo.By("Cleaning up logger app resources")
		for i := 0; i < numberOfSimulatedClusters; i++ {
			shootNamespace := getShootNamesapce(i)
			loggerName := operatorLoggerName
			if i&1 == 1 {
				loggerName = userLoggerName

			}
			loggerDeploymentToDelete := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: shootNamespace.Name,
					Name:      loggerName,
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
			clusterCRD,
			gardenLokiSts,
			gardenLokiService,
			gardenLokiConfMap,
			gardenLokiPriorityClass,
			shootLokiService,
			shootLokiSts,
			shootLokiPriorityClass,
			shootLokiConfMap,
			newGardenNamespace(v1beta1constants.GardenNamespace),
		}
		for _, object := range objectsToDelete {
			framework.ExpectNoError(kutil.DeleteObject(ctx, f.ShootClient.Client(), object))
		}
	}, loggerDeploymentCleanupTimeout))
})

func prepareGardenLokiService(service *corev1.Service) *corev1.Service {
	// Remove the cluster IP because it could be already in use
	service.Spec.ClusterIP = ""
	service.Spec.ClusterIPs = nil
	return service
}

func prepareGardenLokiStatefulSet(gardenLokiSts *appsv1.StatefulSet, lokiPersistentVolumeName, emptyDirSize string, antiAffinityLabels map[string]string) *appsv1.StatefulSet {
	// Remove the Loki PVC as it is no needed for the test
	gardenLokiSts.Spec.VolumeClaimTemplates = nil
	// Instead use an empty dir volume
	gardenLokiSts.Spec.Template.Spec.Volumes = append(gardenLokiSts.Spec.Template.Spec.Volumes, newEmptyDirVolume(lokiPersistentVolumeName, emptyDirSize))
	// Spread the loki instances on separate nodes to let deploying of loggers afterwards to be relatively equal through the nodes
	gardenLokiSts.Spec.Template.Spec.Affinity = &corev1.Affinity{PodAntiAffinity: newPodAntiAffinity(antiAffinityLabels)}
	return gardenLokiSts
}

func prepareShootLokiService(shootLokiService *corev1.Service, name string, selector map[string]string) *corev1.Service {
	shootLokiService.Name = name // "loki-shoots"
	shootLokiService.Namespace = v1beta1constants.GardenNamespace
	shootLokiService.Spec.Selector = selector
	shootLokiService.Spec.ClusterIP = ""
	shootLokiService.Spec.ClusterIPs = nil
	return shootLokiService
}

func prepareShootLokiConfigMap(confMap *corev1.ConfigMap) *corev1.ConfigMap {
	confMap.Namespace = v1beta1constants.GardenNamespace
	return confMap
}

func prepareShootLokiStatefulSet(shootLokiSts, gardenLokiSts *appsv1.StatefulSet, name, configMapNAme, lokiPersistentVolumeName, emptyDirSize string, newLabels, antiAffinityLabels map[string]string) *appsv1.StatefulSet {
	// Extract the containers related only to the seed logging stack
	var containers []corev1.Container
	for _, gardenCon := range gardenLokiSts.Spec.Template.Spec.Containers {
		for _, shootCon := range shootLokiSts.Spec.Template.Spec.Containers {
			if shootCon.Name == gardenCon.Name {
				containers = append(containers, shootCon)
			}
		}
	}
	shootLokiSts.Spec.Template.Spec.Containers = containers
	// Extract the volumes related only to the seed logging stack
	var alreadyHasEmptyDirAsLokiPVC bool
	var volumes []corev1.Volume
	for _, gardenVolume := range gardenLokiSts.Spec.Template.Spec.Volumes {
		for _, shootVolume := range shootLokiSts.Spec.Template.Spec.Volumes {
			if shootVolume.Name == gardenVolume.Name {
				volumes = append(volumes, shootVolume)
				if gardenVolume.Name == lokiPersistentVolumeName && gardenVolume.EmptyDir != nil {
					alreadyHasEmptyDirAsLokiPVC = true
				}
			}
		}
	}
	shootLokiSts.Spec.Template.Spec.Volumes = volumes

	// Remove the Loki PVC as it is no needed for the test
	shootLokiSts.Spec.VolumeClaimTemplates = nil
	// Instead use an empty dir volume
	if !alreadyHasEmptyDirAsLokiPVC {
		shootLokiSts.Spec.Template.Spec.Volumes = append(shootLokiSts.Spec.Template.Spec.Volumes, newEmptyDirVolume(lokiPersistentVolumeName, emptyDirSize))
	}
	// Spread the loki instances on separate nodes to let deploying of loggers afterwards to be relatively equal through the nodes
	shootLokiSts.Spec.Template.Spec.Affinity = &corev1.Affinity{PodAntiAffinity: newPodAntiAffinity(antiAffinityLabels)}
	// Rename the shoot Loki because it has the same name as the garden one
	shootLokiSts.Name = name // "loki-shoots"
	// Change the labels and the selectors because the shoot Loki has the same ones as garden Loki
	shootLokiSts.Labels = newLabels
	shootLokiSts.Spec.Selector.MatchLabels = newLabels
	shootLokiSts.Spec.Template.Labels = newLabels
	// Move the shoot Loki in the garden namespace
	shootLokiSts.Namespace = v1beta1constants.GardenNamespace
	return shootLokiSts
}

func prepareClusterCRD(crd *apiextensionsv1.CustomResourceDefinition) *apiextensionsv1.CustomResourceDefinition {
	crd.Spec.PreserveUnknownFields = false
	for version := range crd.Spec.Versions {
		crd.Spec.Versions[version].Schema.OpenAPIV3Schema.XPreserveUnknownFields = pointer.Bool(true)
	}
	return crd
}

func prepareFluentBitServiceAccount(serviceAccount *corev1.ServiceAccount) *corev1.ServiceAccount {
	serviceAccount.AutomountServiceAccountToken = pointer.BoolPtr(true)
	return serviceAccount
}
