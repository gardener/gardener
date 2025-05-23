// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/resources/templates"
)

// Logs generator constants
const (
	deltaLogsCount            = 1
	deltaLogsDuration         = "180s"
	logsCount                 = 2000
	logsDuration              = "90s"
	numberOfSimulatedClusters = 100
)

// Test setup constants
const (
	initializationTimeout          = 5 * time.Minute
	getLogsFromValiTimeout         = 15 * time.Minute
	loggerDeploymentCleanupTimeout = 5 * time.Minute
)

// Test environment constants
const (
	fluentBitName                 = "fluent-bit"
	fluentBitConfigVolumeName     = "config"
	valiName                      = "vali"
	loggingServiceName            = "logging"
	valiConfigDiskName            = "config"
	garden                        = "garden"
	fluentBitClusterRoleName      = "fluent-operator-fluent-bit"
	simulatedShootNamespacePrefix = "shoot--logging--test-"
)

var _ = ginkgo.Describe("Seed logging testing", func() {
	var (
		shootFramework              = framework.NewShootFramework(nil)
		fluentBit                   = &appsv1.DaemonSet{}
		fluentBitSecret             = &corev1.Secret{}
		fluentBitService            = &corev1.Service{}
		fluentBitClusterRole        = &rbacv1.ClusterRole{}
		fluentBitClusterRoleBinding = &rbacv1.ClusterRoleBinding{}
		fluentBitServiceAccount     = &corev1.ServiceAccount{}
		fluentBitPriorityClass      = &schedulingv1.PriorityClass{}
		clusterCRD                  = &apiextensionsv1.CustomResourceDefinition{}

		gardenValiSts           = &appsv1.StatefulSet{}
		gardenLoggingService    = &corev1.Service{}
		gardenValiConfMap       = &corev1.ConfigMap{}
		gardenValiPriorityClass = &schedulingv1.PriorityClass{}

		shootLoggingService    = &corev1.Service{}
		shootValiSts           = &appsv1.StatefulSet{}
		shootValiPriorityClass = &schedulingv1.PriorityClass{}
		shootValiConfMap       = &corev1.ConfigMap{}

		plutonoIngress client.Object = &networkingv1.Ingress{}
		// This shoot is used as seed for this test only
		shootClient     kubernetes.Interface
		shootValiLabels = map[string]string{
			"app":  "vali-shoot",
			"role": "logging",
		}
		gardenValiLabels = map[string]string{
			"app":  "vali",
			"role": "logging",
		}
	)

	// Test environment setup
	framework.CBeforeEach(func(ctx context.Context) {
		var err error
		checkRequiredResources(ctx, shootFramework.SeedClient)

		// Create seedClient.Client for the shoots
		shootClient, err = kubernetes.NewClientFromSecret(ctx,
			shootFramework.SeedClient.Client(),
			framework.ComputeTechnicalID(shootFramework.Project.Name, shootFramework.Shoot),
			gardencorev1beta1.GardenerName,
			kubernetes.WithClientOptions(
				client.Options{
					Scheme: kubernetes.SeedScheme,
				}),
		)
		framework.ExpectNoError(err)

		// Fetch the daemon set in the garden namespace
		fluentBit, err = getFluentBitDaemonSet(ctx, shootFramework.SeedClient)
		framework.ExpectNoError(err)

		// Client for the seed cluster (gcp-ha)
		// It is used to fetch the resource definitions deployed later in the test cluster
		seedClient := shootFramework.SeedClient.Client()
		// Fetch the fluent-bit configuration
		framework.ExpectNoError(
			seedClient.Get(ctx,
				types.NamespacedName{
					Namespace: v1beta1constants.GardenNamespace,
					Name:      getSecretNameFromVolume(fluentBit.Spec.Template.Spec.Volumes, fluentBitConfigVolumeName),
				},
				fluentBitSecret),
		)

		// Fetch the fluent-bit service
		framework.ExpectNoError(
			seedClient.Get(ctx,
				types.NamespacedName{
					Namespace: v1beta1constants.GardenNamespace,
					Name:      fluentBitName},
				fluentBitService),
		)

		// Fetch the fluent-bit cluster role
		framework.ExpectNoError(
			seedClient.Get(ctx,
				types.NamespacedName{
					Namespace: "",
					Name:      fluentBitClusterRoleName},
				fluentBitClusterRole),
		)

		// Fetch the fluent-bit cluster role binding
		framework.ExpectNoError(
			seedClient.Get(ctx,
				types.NamespacedName{
					Namespace: "",
					Name:      fluentBitClusterRoleName + "-" + fluentBit.Name},
				fluentBitClusterRoleBinding),
		)

		// Fetch the fluent-bit service account
		framework.ExpectNoError(
			seedClient.Get(ctx,
				types.NamespacedName{
					Namespace: v1beta1constants.GardenNamespace,
					Name:      fluentBit.Name},
				fluentBitServiceAccount),
		)

		// Fetch the fluent-bit priority class
		fluentBitPriorityClassName := fluentBit.Spec.Template.Spec.PriorityClassName
		framework.ExpectNoError(
			seedClient.Get(ctx,
				types.NamespacedName{
					Namespace: "",
					Name:      fluentBitPriorityClassName},
				fluentBitPriorityClass),
		)

		// Fetch the Cluster CRD
		framework.ExpectNoError(
			seedClient.Get(ctx,
				types.NamespacedName{
					Namespace: "",
					Name:      "clusters.extensions.gardener.cloud"},
				clusterCRD),
		)

		// Fetch Vali StatefulSet
		framework.ExpectNoError(
			seedClient.Get(ctx,
				types.NamespacedName{
					Namespace: v1beta1constants.GardenNamespace,
					Name:      valiName},
				gardenValiSts),
		)

		// Fetch Logging service
		framework.ExpectNoError(
			seedClient.Get(ctx,
				types.NamespacedName{
					Namespace: v1beta1constants.GardenNamespace,
					Name:      loggingServiceName},
				gardenLoggingService),
		)

		// Fetch Vali configuration
		framework.ExpectNoError(
			seedClient.Get(ctx,
				types.NamespacedName{
					Namespace: v1beta1constants.GardenNamespace,
					Name:      getConfigMapName(gardenValiSts.Spec.Template.Spec.Volumes, valiConfigDiskName)},
				gardenValiConfMap),
		)

		// Fetch Vali priority class
		valiPriorityClassName := gardenValiSts.Spec.Template.Spec.PriorityClassName
		framework.ExpectNoError(
			seedClient.Get(ctx,
				types.NamespacedName{
					Namespace: v1beta1constants.GardenNamespace,
					Name:      valiPriorityClassName},
				gardenValiPriorityClass),
		)

		// Get the shoot logging components from the shoot running the test
		framework.ExpectNoError(
			seedClient.Get(ctx,
				types.NamespacedName{
					Namespace: shootFramework.ShootSeedNamespace(),
					Name:      loggingServiceName},
				shootLoggingService),
		)

		// Fetch shoot Vali Statefulset
		framework.ExpectNoError(
			seedClient.Get(ctx,
				types.NamespacedName{
					Namespace: shootFramework.ShootSeedNamespace(),
					Name:      valiName},
				shootValiSts),
		)

		// Fetch shoot Vali configuration
		framework.ExpectNoError(
			seedClient.Get(ctx,
				types.NamespacedName{
					Namespace: shootFramework.ShootSeedNamespace(),
					Name:      getConfigMapName(shootValiSts.Spec.Template.Spec.Volumes, valiConfigDiskName)},
				shootValiConfMap),
		)

		// Fetch the configMap template
		shootValiConfMap.Data["vali.yaml"] = valiYaml

		// Fetch shoot vali priority class name
		shootValiPriorityClassName := shootValiSts.Spec.Template.Spec.PriorityClassName
		framework.ExpectNoError(
			seedClient.Get(ctx,
				types.NamespacedName{
					Namespace: shootFramework.ShootSeedNamespace(),
					Name:      shootValiPriorityClassName},
				shootValiPriorityClass),
		)

		// Get the plutono Ingress
		framework.ExpectNoError(
			seedClient.Get(ctx,
				types.NamespacedName{
					Namespace: shootFramework.ShootSeedNamespace(),
					Name:      v1beta1constants.DeploymentNamePlutono},
				plutonoIngress),
		)
	}, initializationTimeout)

	// Test cases
	shootFramework.Beta().Serial().CIt("should get container logs from vali for all namespaces", func(ctx context.Context) {
		const (
			emptyDirSize             = "500Mi"
			valiPersistentVolumeName = "vali"
			shootValiName            = "vali-shoots"
			loggerRegex              = loggerName + "-.*"
		)
		var (
			loggerLabels = labels.SelectorFromSet(
				map[string]string{"app": loggerName},
			)
			// fetch target test cluster client
			client = shootFramework.ShootClient.Client()
		)

		ginkgo.By("Create the garden namespace")
		framework.ExpectNoError(
			create(ctx, client, newGardenNamespace(v1beta1constants.GardenNamespace)),
		)

		ginkgo.By("Deploy the seed vali instance")
		framework.ExpectNoError(
			create(ctx, client, gardenValiConfMap),
		)
		framework.ExpectNoError(
			create(ctx, client, prepareGardenLoggingService(gardenLoggingService)),
		)
		framework.ExpectNoError(
			create(ctx, client, gardenValiPriorityClass),
		)
		framework.ExpectNoError(
			create(ctx, client, prepareGardenValiStatefulSet(gardenValiSts, valiPersistentVolumeName, emptyDirSize, shootValiLabels)),
		)

		ginkgo.By("Deploy the shoot vali instance")
		framework.ExpectNoError(
			create(ctx, client, prepareShootLoggingService(shootLoggingService, "logging-shoot", shootValiLabels)),
		)
		framework.ExpectNoError(
			create(ctx, client, shootValiPriorityClass),
		)
		framework.ExpectNoError(
			create(ctx, client, prepareShootValiConfigMap(shootValiConfMap)),
		)

		framework.ExpectNoError(
			create(ctx, client, prepareShootValiStatefulSet(shootValiSts, gardenValiSts, shootValiName, valiPersistentVolumeName, emptyDirSize, shootValiLabels, gardenValiLabels)),
		)

		ginkgo.By("Wait until seed vali instance is ready")
		framework.ExpectNoError(
			shootFramework.WaitUntilStatefulSetIsRunning(ctx, gardenValiSts.Name, v1beta1constants.GardenNamespace, shootFramework.ShootClient),
		)

		ginkgo.By("Wait until shoot vali instance is ready")
		framework.ExpectNoError(
			shootFramework.WaitUntilStatefulSetIsRunning(ctx, shootValiSts.Name, v1beta1constants.GardenNamespace, shootFramework.ShootClient),
		)

		ginkgo.By("Deploy the Cluster CRD")
		framework.ExpectNoError(
			create(ctx, shootClient.Client(), prepareClusterCRD(clusterCRD)),
		)

		ginkgo.By("Deploy the fluent-bit RBAC")
		framework.ExpectNoError(
			create(ctx, client, prepareFluentBitServiceAccount(fluentBitServiceAccount)),
		)
		framework.ExpectNoError(
			create(ctx, client, fluentBitPriorityClass),
		)
		framework.ExpectNoError(
			create(ctx, client, fluentBitClusterRole),
		)
		framework.ExpectNoError(
			create(ctx, client, fluentBitClusterRoleBinding),
		)

		ginkgo.By("Deploy the fluent-bit")
		framework.ExpectNoError(
			create(ctx, client, fluentBitSecret),
		)
		framework.ExpectNoError(
			create(ctx, client, fluentBit),
		)

		ginkgo.By("Wait until fluent-bit is ready")
		framework.ExpectNoError(
			shootFramework.WaitUntilDaemonSetIsRunning(ctx, client, fluentBit.Name, v1beta1constants.GardenNamespace),
		)

		ginkgo.By("Deploy the simulated clusters and shoot namespaces")
		nodeList := &corev1.NodeList{}
		framework.ExpectNoError(
			client.List(ctx, nodeList),
		)

		for i := 0; i < numberOfSimulatedClusters; i++ {
			shootNamespace := getShootNamespace(i)

			// Create shoot namespace
			ginkgo.By(fmt.Sprintf("Create shoot namespace %s", shootNamespace.Name))
			framework.ExpectNoError(
				create(ctx, client, shootNamespace),
			)

			// Create Cluster resource for the shoot namespace
			cluster := getCluster(i)
			ginkgo.By(fmt.Sprintf("Deploy cluster %s", cluster.Name))
			framework.ExpectNoError(
				create(ctx, shootClient.Client(), cluster),
			)

			ginkgo.By(fmt.Sprintf("Create the logging service in shoot namespace %s", shootNamespace.Name))
			loggingShootService := getLoggingShootService(i)
			framework.ExpectNoError(
				create(ctx, client, loggingShootService),
			)

			ginkgo.By(fmt.Sprintf("Deploy the logger application in shoot namespace %s", shootNamespace.Name))
			loggerParams := map[string]any{
				"LoggerName":          loggerName,
				"HelmDeployNamespace": shootNamespace.Name,
				"AppLabel":            loggerName,
				"DeltaLogsCount":      deltaLogsCount,
				"DeltaLogsDuration":   deltaLogsDuration,
				"LogsCount":           logsCount,
				"LogsDuration":        logsDuration,
			}

			// Try to distribute the loggers even between nodes
			if len(nodeList.Items) > 0 {
				loggerParams["NodeName"] = nodeList.Items[i%len(nodeList.Items)].Name
			}

			framework.ExpectNoError(
				shootFramework.RenderAndDeployTemplate(ctx,
					shootFramework.ShootClient, templates.LoggerAppName, loggerParams),
			)
		}

		for i := 0; i < numberOfSimulatedClusters; i++ {
			shootNamespace := fmt.Sprintf("%s%v", simulatedShootNamespacePrefix, i)

			ginkgo.By(fmt.Sprintf("Wait until logger application is ready in shoot namespace %s", shootNamespace))
			framework.ExpectNoError(
				shootFramework.WaitUntilDeploymentsWithLabelsIsReady(ctx,
					loggerLabels, shootNamespace, shootFramework.ShootClient),
			)
		}

		ginkgo.By("Verify vali received all logs from all shoot namespaces")
		framework.ExpectNoError(
			WaitUntilValiReceivesLogs(ctx, 30*time.Second,
				shootFramework, shootValiLabels, v1beta1constants.GardenNamespace,
				"pod_name", loggerRegex, logsCount*numberOfSimulatedClusters,
				numberOfSimulatedClusters, shootFramework.ShootClient,
			),
		)

		ginkgo.By("Verify vali received logger application logs for garden namespace")
		framework.ExpectNoError(
			WaitUntilValiReceivesLogs(ctx, 30*time.Second,
				shootFramework, gardenValiLabels, v1beta1constants.GardenNamespace,
				"pod_name", loggerRegex, logsCount*numberOfSimulatedClusters,
				numberOfSimulatedClusters, shootFramework.ShootClient,
			),
		)

	}, getLogsFromValiTimeout, framework.WithCAfterTest(func(ctx context.Context) {
		ginkgo.By("Cleanup logger app resources")
		for i := 0; i < numberOfSimulatedClusters; i++ {
			shootNamespace := getShootNamespace(i)

			loggerDeploymentToDelete := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: shootNamespace.Name,
					Name:      loggerName,
				},
			}

			// Delete the logger application
			framework.ExpectNoError(
				kubernetesutils.DeleteObject(ctx, shootFramework.ShootClient.Client(), loggerDeploymentToDelete),
			)

			// Delete Cluster resource
			cluster := getCluster(i)
			framework.ExpectNoError(
				kubernetesutils.DeleteObject(ctx, shootClient.Client(), cluster),
			)

			// Delete logging service
			loggingShootService := getLoggingShootService(i)
			framework.ExpectNoError(
				kubernetesutils.DeleteObject(ctx, shootFramework.ShootClient.Client(), loggingShootService),
			)

			// Delete the shoot namespace
			framework.ExpectNoError(
				kubernetesutils.DeleteObject(ctx, shootFramework.ShootClient.Client(), shootNamespace),
			)
		}

		ginkgo.By("Cleanup garden namespace")
		objectsToDelete := []client.Object{
			fluentBit,
			fluentBitSecret,
			fluentBitService,
			fluentBitClusterRole,
			fluentBitClusterRoleBinding,
			fluentBitServiceAccount,
			fluentBitPriorityClass,
			clusterCRD,
			gardenValiSts,
			gardenLoggingService,
			gardenValiConfMap,
			gardenValiPriorityClass,
			shootLoggingService,
			shootValiSts,
			shootValiPriorityClass,
			shootValiConfMap,
			newGardenNamespace(v1beta1constants.GardenNamespace),
		}
		for _, object := range objectsToDelete {
			framework.ExpectNoError(kubernetesutils.DeleteObject(ctx, shootFramework.ShootClient.Client(), object))
		}
	}, loggerDeploymentCleanupTimeout))
})

func prepareGardenLoggingService(service *corev1.Service) *corev1.Service {
	// Remove the cluster IP because it could be already in use
	service.Spec.ClusterIP = ""
	service.Spec.ClusterIPs = nil
	return service
}

func prepareGardenValiStatefulSet(gardenValiSts *appsv1.StatefulSet, valiPersistentVolumeName, emptyDirSize string, antiAffinityLabels map[string]string) *appsv1.StatefulSet {
	// Remove the Vali PVC as it is no needed for the test
	gardenValiSts.Spec.VolumeClaimTemplates = nil
	// Instead use an empty dir volume
	gardenValiSts.Spec.Template.Spec.Volumes = append(gardenValiSts.Spec.Template.Spec.Volumes, newEmptyDirVolume(valiPersistentVolumeName, emptyDirSize))
	// Spread the vali instances on separate nodes to let deploying of loggers afterwards to be relatively equal through the nodes
	gardenValiSts.Spec.Template.Spec.Affinity = &corev1.Affinity{PodAntiAffinity: newPodAntiAffinity(antiAffinityLabels)}

	capValiContainerResources(gardenValiSts)

	return gardenValiSts
}

func prepareShootLoggingService(shootLoggingService *corev1.Service, name string, selector map[string]string) *corev1.Service {
	shootLoggingService.Name = name // "vali-shoots"
	shootLoggingService.Namespace = v1beta1constants.GardenNamespace
	shootLoggingService.Spec.Selector = selector
	shootLoggingService.Spec.ClusterIP = ""
	shootLoggingService.Spec.ClusterIPs = nil
	return shootLoggingService
}

func prepareShootValiConfigMap(confMap *corev1.ConfigMap) *corev1.ConfigMap {
	confMap.Namespace = v1beta1constants.GardenNamespace
	return confMap
}

func prepareShootValiStatefulSet(shootValiSts, gardenValiSts *appsv1.StatefulSet, name, valiPersistentVolumeName, emptyDirSize string, newLabels, antiAffinityLabels map[string]string) *appsv1.StatefulSet {
	// Extract the containers related only to the seed logging stack
	var containers []corev1.Container
	for _, gardenCon := range gardenValiSts.Spec.Template.Spec.Containers {
		for _, shootCon := range shootValiSts.Spec.Template.Spec.Containers {
			if shootCon.Name == gardenCon.Name {
				containers = append(containers, shootCon)
			}
		}
	}
	shootValiSts.Spec.Template.Spec.Containers = containers
	// Extract the volumes related only to the seed logging stack
	var alreadyHasEmptyDirAsValiPVC bool
	var volumes []corev1.Volume
	for _, gardenVolume := range gardenValiSts.Spec.Template.Spec.Volumes {
		for _, shootVolume := range shootValiSts.Spec.Template.Spec.Volumes {
			if shootVolume.Name == gardenVolume.Name {
				volumes = append(volumes, shootVolume)
				if gardenVolume.Name == valiPersistentVolumeName && gardenVolume.EmptyDir != nil {
					alreadyHasEmptyDirAsValiPVC = true
				}
			}
		}
	}
	shootValiSts.Spec.Template.Spec.Volumes = volumes

	// Remove the Vali PVC as it is no needed for the test
	shootValiSts.Spec.VolumeClaimTemplates = nil
	// Instead use an empty dir volume
	if !alreadyHasEmptyDirAsValiPVC {
		shootValiSts.Spec.Template.Spec.Volumes = append(shootValiSts.Spec.Template.Spec.Volumes, newEmptyDirVolume(valiPersistentVolumeName, emptyDirSize))
	}
	// Spread the vali instances on separate nodes to let deploying of loggers afterwards to be relatively equal through the nodes
	shootValiSts.Spec.Template.Spec.Affinity = &corev1.Affinity{PodAntiAffinity: newPodAntiAffinity(antiAffinityLabels)}
	// Rename the shoot Vali because it has the same name as the garden one
	shootValiSts.Name = name // "vali-shoots"
	// Change the labels and the selectors because the shoot Vali has the same ones as garden Vali
	shootValiSts.Labels = newLabels
	shootValiSts.Spec.Selector.MatchLabels = newLabels
	shootValiSts.Spec.Template.Labels = newLabels
	// Move the shoot Vali in the garden namespace
	shootValiSts.Namespace = v1beta1constants.GardenNamespace

	capValiContainerResources(shootValiSts)

	return shootValiSts
}

func capValiContainerResources(valiSts *appsv1.StatefulSet) {
	for i, container := range valiSts.Spec.Template.Spec.Containers {
		if container.Name == "vali" {
			valiSts.Spec.Template.Spec.Containers[i].Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("300Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("3Gi"),
				},
			}
			return
		}
	}
}

func prepareClusterCRD(crd *apiextensionsv1.CustomResourceDefinition) *apiextensionsv1.CustomResourceDefinition {
	crd.Spec.PreserveUnknownFields = false
	for version := range crd.Spec.Versions {
		crd.Spec.Versions[version].Schema.OpenAPIV3Schema.XPreserveUnknownFields = ptr.To(true)
	}
	return crd
}

func prepareFluentBitServiceAccount(serviceAccount *corev1.ServiceAccount) *corev1.ServiceAccount {
	serviceAccount.AutomountServiceAccountToken = ptr.To(true)
	return serviceAccount
}

func getFluentBitDaemonSet(ctx context.Context, k8sSeedClient kubernetes.Interface) (*appsv1.DaemonSet, error) {
	daemonSetList := &appsv1.DaemonSetList{}
	err := k8sSeedClient.Client().List(ctx,
		daemonSetList,
		client.InNamespace(garden),
		client.MatchingLabels{
			v1beta1constants.LabelApp:   v1beta1constants.DaemonSetNameFluentBit,
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleLogging,
		})
	if err != nil {
		return nil, err
	}
	if len(daemonSetList.Items) == 0 {
		return nil, fmt.Errorf("fluent-bit daemonset not found")
	}

	return daemonSetList.Items[0].DeepCopy(), nil
}
