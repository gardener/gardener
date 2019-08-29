// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/migration"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var chartPathControlPlane = filepath.Join(common.ChartPath, "seed-controlplane", "charts")

// DeployNamespace creates a namespace in the Seed cluster which is used to deploy all the control plane
// components for the Shoot cluster. Moreover, the cloud provider configuration and all the secrets will be
// stored as ConfigMaps/Secrets.
func (b *Botanist) DeployNamespace(ctx context.Context) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: b.Shoot.SeedNamespace,
		},
	}

	if err := kutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), namespace, func() error {
		namespace.Annotations = getShootAnnotations(b.Shoot.Info.Annotations, b.Shoot.Info.Status.UID)
		namespace.Labels = map[string]string{
			common.GardenRole:                          common.GardenRoleShoot,
			gardencorev1alpha1.GardenRole:              common.GardenRoleShoot,
			common.ShootHibernated:                     strconv.FormatBool(b.Shoot.HibernationEnabled),
			gardencorev1alpha1.LabelBackupProvider:     string(b.Seed.CloudProvider),
			gardencorev1alpha1.LabelSeedProvider:       string(b.Seed.CloudProvider),
			gardencorev1alpha1.LabelShootProvider:      string(b.Shoot.CloudProvider),
			gardencorev1alpha1.LabelNetworkingProvider: string(b.Shoot.Info.Spec.Networking.Type),
		}
		return nil
	}); err != nil {
		return err
	}

	b.SeedNamespaceObject = namespace
	return nil
}

func getShootAnnotations(annotations map[string]string, uid types.UID) map[string]string {
	shootAnnotations := map[string]string{
		common.ShootUID: string(uid),
	}
	for key, value := range annotations {
		if strings.HasPrefix(key, common.AnnotateSeedNamespacePrefix) {
			shootAnnotations[key] = value
		}
	}
	return shootAnnotations
}

// DeployBackupNamespace creates a namespace in the Seed cluster from info in shoot object, which is used to deploy all the backup infrastructure
// realted resources for shoot cluster. Moreover, the terraform configuration and all the secrets will be
// stored as ConfigMaps/Secrets.
func (b *Botanist) DeployBackupNamespace(ctx context.Context) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: common.GenerateBackupNamespaceName(b.BackupInfrastructure.Name),
		},
	}

	return kutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), namespace, func() error {
		namespace.Labels = map[string]string{
			common.GardenRole: common.GardenRoleBackup,
		}
		return nil
	})
}

// DeleteNamespace deletes the namespace in the Seed cluster which holds the control plane components. The built-in
// garbage collection in Kubernetes will automatically delete all resources which belong to this namespace. This
// comprises volumes and load balancers as well.
func (b *Botanist) DeleteNamespace(ctx context.Context) error {
	return b.deleteNamespace(ctx, b.Shoot.SeedNamespace)
}

// DeleteBackupNamespace deletes the namespace in the Seed cluster which holds the backup infrastructure state. The built-in
// garbage collection in Kubernetes will automatically delete all resources which belong to this namespace.
func (b *Botanist) DeleteBackupNamespace(ctx context.Context) error {
	return b.deleteNamespace(ctx, common.GenerateBackupNamespaceName(b.BackupInfrastructure.Name))
}

func (b *Botanist) deleteNamespace(ctx context.Context, name string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	err := b.K8sSeedClient.Client().Delete(ctx, namespace, kubernetes.DefaultDeleteOptionFuncs...)
	if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
		return nil
	}
	return err
}

// DeleteKubeAPIServer deletes the kube-apiserver deployment in the Seed cluster which holds the Shoot's control plane.
func (b *Botanist) DeleteKubeAPIServer(ctx context.Context) error {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gardencorev1alpha1.DeploymentNameKubeAPIServer,
			Namespace: b.Shoot.SeedNamespace,
		},
	}
	return client.IgnoreNotFound(b.K8sSeedClient.Client().Delete(ctx, deploy, kubernetes.DefaultDeleteOptionFuncs...))
}

// DeleteBackupInfrastructure deletes the sets deletionTimestamp on the backupInfrastructure resource in the Garden namespace
// which is responsible for actual deletion of cloud resource for Shoot's backup infrastructure.
func (b *Botanist) DeleteBackupInfrastructure() error {
	err := b.K8sGardenClient.Garden().GardenV1beta1().BackupInfrastructures(b.Shoot.Info.Namespace).Delete(common.GenerateBackupInfrastructureName(b.Shoot.SeedNamespace, b.Shoot.Info.Status.UID), nil)
	return client.IgnoreNotFound(err)
}

// DeployClusterAutoscaler deploys the cluster-autoscaler into the Shoot namespace in the Seed cluster. It is responsible
// for automatically scaling the worker pools of the Shoot.
func (b *Botanist) DeployClusterAutoscaler(ctx context.Context) error {
	if !b.Shoot.WantsClusterAutoscaler {
		return b.DeleteClusterAutoscaler(ctx)
	}

	var workerPools []map[string]interface{}
	for _, worker := range b.Shoot.MachineDeployments {
		// Skip worker pools for which min=0. Auto scaler cannot handle worker pools having a min count of 0.
		if worker.Minimum == 0 {
			continue
		}

		workerPools = append(workerPools, map[string]interface{}{
			"name": worker.Name,
			"min":  worker.Minimum,
			"max":  worker.Maximum,
		})
	}

	defaultValues := map[string]interface{}{
		"podAnnotations": map[string]interface{}{
			"checksum/secret-cluster-autoscaler": b.CheckSums[gardencorev1alpha1.DeploymentNameClusterAutoscaler],
		},
		"namespace": map[string]interface{}{
			"uid": b.SeedNamespaceObject.UID,
		},
		"replicas":    b.Shoot.GetReplicas(1),
		"workerPools": workerPools,
	}

	if clusterAutoscalerConfig := b.Shoot.Info.Spec.Kubernetes.ClusterAutoscaler; clusterAutoscalerConfig != nil {
		if val := clusterAutoscalerConfig.ScaleDownUtilizationThreshold; val != nil {
			defaultValues["scaleDownUtilizationThreshold"] = *val
		}
		if val := clusterAutoscalerConfig.ScaleDownUnneededTime; val != nil {
			defaultValues["scaleDownUnneededTime"] = *val
		}
		if val := clusterAutoscalerConfig.ScaleDownDelayAfterAdd; val != nil {
			defaultValues["scaleDownDelayAfterAdd"] = *val
		}
		if val := clusterAutoscalerConfig.ScaleDownDelayAfterFailure; val != nil {
			defaultValues["scaleDownDelayAfterFailure"] = *val
		}
		if val := clusterAutoscalerConfig.ScaleDownDelayAfterDelete; val != nil {
			defaultValues["scaleDownDelayAfterDelete"] = *val
		}
		if val := clusterAutoscalerConfig.ScanInterval; val != nil {
			defaultValues["scanInterval"] = *val
		}
	}

	values, err := b.InjectSeedShootImages(defaultValues, common.ClusterAutoscalerImageName)
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, gardencorev1alpha1.DeploymentNameClusterAutoscaler), b.Shoot.SeedNamespace, gardencorev1alpha1.DeploymentNameClusterAutoscaler, nil, values)
}

// DeleteClusterAutoscaler deletes the cluster-autoscaler deployment in the Seed cluster which holds the Shoot's control plane.
func (b *Botanist) DeleteClusterAutoscaler(ctx context.Context) error {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gardencorev1alpha1.DeploymentNameClusterAutoscaler,
			Namespace: b.Shoot.SeedNamespace,
		},
	}
	return client.IgnoreNotFound(b.K8sSeedClient.Client().Delete(ctx, deploy, kubernetes.DefaultDeleteOptionFuncs...))
}

// DeployDependencyWatchdog deploys the dependency watchdog to the Shoot namespace in the Seed.
func (b *Botanist) DeployDependencyWatchdog(ctx context.Context) error {
	dependencyWatchdogConfig := map[string]interface{}{
		"replicas": b.Shoot.GetReplicas(1),
	}

	dependencyWatchdog, err := b.InjectSeedSeedImages(dependencyWatchdogConfig, gardencorev1alpha1.DeploymentNameDependencyWatchdog)
	if err != nil {
		return nil
	}
	return b.ChartApplierSeed.ApplyChart(ctx, filepath.Join(chartPathControlPlane, gardencorev1alpha1.DeploymentNameDependencyWatchdog), b.Shoot.SeedNamespace, gardencorev1alpha1.DeploymentNameDependencyWatchdog, nil, dependencyWatchdog)
}

// WakeUpControlPlane scales the replicas to 1 for the following deployments which are needed in case of shoot deletion:
// * etcd-events
// * etcd-main
// * kube-apiserver
// * kube-controller-manager
func (b *Botanist) WakeUpControlPlane(ctx context.Context) error {
	client := b.K8sSeedClient.Client()

	for _, statefulset := range []string{gardencorev1alpha1.StatefulSetNameETCDEvents, gardencorev1alpha1.StatefulSetNameETCDMain} {
		if err := kubernetes.ScaleStatefulSet(ctx, client, kutil.Key(b.Shoot.SeedNamespace, statefulset), 1); err != nil {
			return err
		}
	}
	if err := b.WaitUntilEtcdReady(ctx); err != nil {
		return err
	}

	if err := kubernetes.ScaleDeployment(ctx, client, kutil.Key(b.Shoot.SeedNamespace, gardencorev1alpha1.DeploymentNameKubeAPIServer), 1); err != nil {
		return err
	}
	if err := b.WaitUntilKubeAPIServerReady(ctx); err != nil {
		return err
	}

	for _, deployment := range []string{
		gardencorev1alpha1.DeploymentNameKubeControllerManager,
		gardencorev1alpha1.DeploymentNameGardenerResourceManager,
	} {
		if err := kubernetes.ScaleDeployment(ctx, client, kutil.Key(b.Shoot.SeedNamespace, deployment), 1); err != nil {
			return err
		}
	}

	return nil
}

// HibernateControlPlane hibernates the entire control plane if the shoot shall be hibernated.
func (b *Botanist) HibernateControlPlane(ctx context.Context) error {
	c := b.K8sSeedClient.Client()

	// If a shoot is hibernated we only want to scale down the entire control plane if no nodes exist anymore. The node-lifecycle-controller
	// inside KCM is responsible for deleting Node objects of terminated/non-existing VMs, so let's wait for that before scaling down.
	if b.K8sShootClient != nil {
		ctxWithTimeOut, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		if err := b.WaitUntilNodesDeleted(ctxWithTimeOut); err != nil {
			return err
		}
	}

	deployments := []string{
		gardencorev1alpha1.DeploymentNameGardenerResourceManager,
		gardencorev1alpha1.DeploymentNameKubeControllerManager,
		gardencorev1alpha1.DeploymentNameKubeAPIServer,
	}
	for _, deployment := range deployments {
		if err := kubernetes.ScaleDeployment(ctx, c, kutil.Key(b.Shoot.SeedNamespace, deployment), 0); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	if err := c.Delete(ctx, &autoscalingv2beta1.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: gardencorev1alpha1.DeploymentNameKubeAPIServer, Namespace: b.Shoot.SeedNamespace}}, kubernetes.DefaultDeleteOptionFuncs...); client.IgnoreNotFound(err) != nil {
		return err
	}

	for _, statefulset := range []string{gardencorev1alpha1.StatefulSetNameETCDEvents, gardencorev1alpha1.StatefulSetNameETCDMain} {
		if err := kubernetes.ScaleStatefulSet(ctx, c, kutil.Key(b.Shoot.SeedNamespace, statefulset), 0); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}

// ControlPlaneDefaultTimeout is the default timeout and defines how long Gardener should wait
// for a successful reconciliation of a control plane resource.
const ControlPlaneDefaultTimeout = 3 * time.Minute

// DeployControlPlane creates the `ControlPlane` extension resource in the shoot namespace in the seed
// cluster. Gardener waits until an external controller did reconcile the cluster successfully.
func (b *Botanist) DeployControlPlane(ctx context.Context) error {
	var (
		cp = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      b.Shoot.Info.Name,
				Namespace: b.Shoot.SeedNamespace,
			},
		}
	)

	// In the future the providerConfig will be blindly copied from the core.gardener.cloud/v1alpha1.Shoot
	// resource. However, until we have completely moved to this resource, we have to compute the needed
	// configuration ourselves from garden.sapcloud.io/v1beta1.Shoot.
	providerConfig, err := migration.ShootToControlPlaneConfig(b.Shoot.Info)
	if err != nil {
		return err
	}

	return kutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), cp, func() error {
		metav1.SetMetaDataAnnotation(&cp.ObjectMeta, gardencorev1alpha1.GardenerOperation, gardencorev1alpha1.GardenerOperationReconcile)
		cp.Spec = extensionsv1alpha1.ControlPlaneSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: string(b.Shoot.CloudProvider),
			},
			Region: b.Shoot.Info.Spec.Cloud.Region,
			SecretRef: corev1.SecretReference{
				Name:      gardencorev1alpha1.SecretNameCloudProvider,
				Namespace: cp.Namespace,
			},
			ProviderConfig: &runtime.RawExtension{
				Object: providerConfig,
			},
			InfrastructureProviderStatus: &runtime.RawExtension{
				Raw: b.Shoot.InfrastructureStatus,
			},
		}
		return nil
	})
}

const controlPlaneExposureSuffix = "-exposure"

// DeployControlPlaneExposure creates the `ControlPlane` extension resource with purpose `exposure` in the shoot
// namespace in the seed cluster. Gardener waits until an external controller did reconcile the cluster successfully.
func (b *Botanist) DeployControlPlaneExposure(ctx context.Context) error {
	var (
		cp = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      b.Shoot.Info.Name + controlPlaneExposureSuffix,
				Namespace: b.Shoot.SeedNamespace,
			},
		}
	)

	purpose := new(extensionsv1alpha1.Purpose)
	*purpose = extensionsv1alpha1.Exposure

	return kutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), cp, func() error {
		metav1.SetMetaDataAnnotation(&cp.ObjectMeta, gardencorev1alpha1.GardenerOperation, gardencorev1alpha1.GardenerOperationReconcile)
		cp.Spec = extensionsv1alpha1.ControlPlaneSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: string(b.Seed.CloudProvider),
			},
			Purpose: purpose,
			Region:  b.Seed.Info.Spec.Cloud.Region,
			SecretRef: corev1.SecretReference{
				Name:      gardencorev1alpha1.SecretNameCloudProvider,
				Namespace: cp.Namespace,
			},
		}
		return nil
	})
}

// DestroyControlPlane deletes the `ControlPlane` extension resource in the shoot namespace in the seed cluster,
// and it waits for a maximum of 10m until it is deleted.
func (b *Botanist) DestroyControlPlane(ctx context.Context) error {
	return b.destroyControlPlane(ctx, b.Shoot.Info.Name)
}

// DestroyControlPlaneExposure deletes the `ControlPlane` extension resource with purpose `exposure`
// in the shoot namespace in the seed cluster, and it waits for a maximum of 10m until it is deleted.
func (b *Botanist) DestroyControlPlaneExposure(ctx context.Context) error {
	return b.destroyControlPlane(ctx, b.Shoot.Info.Name+controlPlaneExposureSuffix)
}

// destroyControlPlane deletes the `ControlPlane` extension resource with the following name in the shoot namespace
// in the seed cluster, and it waits for a maximum of 10m until it is deleted.
func (b *Botanist) destroyControlPlane(ctx context.Context, name string) error {
	return client.IgnoreNotFound(b.K8sSeedClient.Client().Delete(ctx, &extensionsv1alpha1.ControlPlane{ObjectMeta: metav1.ObjectMeta{Namespace: b.Shoot.SeedNamespace, Name: name}}))
}

// WaitUntilControlPlaneExposureReady waits until the control plane resource with purpose `exposure` has been reconciled successfully.
func (b *Botanist) WaitUntilControlPlaneExposureReady(ctx context.Context) error {
	return b.waitUntilControlPlaneReady(ctx, b.Shoot.Info.Name+controlPlaneExposureSuffix)
}

// WaitUntilControlPlaneReady waits until the control plane resource has been reconciled successfully.
func (b *Botanist) WaitUntilControlPlaneReady(ctx context.Context) error {
	return b.waitUntilControlPlaneReady(ctx, b.Shoot.Info.Name)
}

// waitUntilControlPlaneReady waits until the control plane resource has been reconciled successfully.
func (b *Botanist) waitUntilControlPlaneReady(ctx context.Context, name string) error {
	if err := retry.UntilTimeout(ctx, DefaultInterval, ControlPlaneDefaultTimeout, func(ctx context.Context) (bool, error) {
		cp := &extensionsv1alpha1.ControlPlane{}
		if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKey{Name: name, Namespace: b.Shoot.SeedNamespace}, cp); err != nil {
			return retry.SevereError(err)
		}

		if err := health.CheckExtensionObject(cp); err != nil {
			b.Logger.WithError(err).Error("Control plane did not get ready yet")
			return retry.MinorError(err)
		}

		if cp.Status.ProviderStatus != nil {
			b.Shoot.ControlPlaneStatus = cp.Status.ProviderStatus.Raw
		}
		return retry.Ok()
	}); err != nil {
		return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("failed to create control plane: %v", err))
	}
	return nil
}

// WaitUntilControlPlaneExposureDeleted waits until the control plane resource with pusrpose `exposure` has been deleted.
func (b *Botanist) WaitUntilControlPlaneExposureDeleted(ctx context.Context) error {
	return b.waitUntilControlPlaneDeleted(ctx, b.Shoot.Info.Name+controlPlaneExposureSuffix)
}

// WaitUntilControlPlaneDeleted waits until the control plane resource has been deleted.
func (b *Botanist) WaitUntilControlPlaneDeleted(ctx context.Context) error {
	return b.waitUntilControlPlaneDeleted(ctx, b.Shoot.Info.Name)
}

// waitUntilControlPlaneDeleted waits until the control plane resource with the following name has been deleted.
func (b *Botanist) waitUntilControlPlaneDeleted(ctx context.Context, name string) error {
	var lastError *gardencorev1alpha1.LastError

	if err := retry.UntilTimeout(ctx, DefaultInterval, ControlPlaneDefaultTimeout, func(ctx context.Context) (bool, error) {
		cp := &extensionsv1alpha1.ControlPlane{}
		if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKey{Name: name, Namespace: b.Shoot.SeedNamespace}, cp); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.SevereError(err)
		}

		if lastErr := cp.Status.LastError; lastErr != nil {
			b.Logger.Errorf("Control plane did not get deleted yet, lastError is: %s", lastErr.Description)
			lastError = lastErr
		}

		b.Logger.Infof("Waiting for control plane to be deleted...")
		return retry.MinorError(common.WrapWithLastError(fmt.Errorf("control plane is not yet deleted"), lastError))
	}); err != nil {
		message := fmt.Sprintf("Failed to delete control plane")
		if lastError != nil {
			return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, lastError.Description))
		}
		return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, err.Error()))
	}
	return nil
}

// DeployGardenerResourceManager deploys the gardener-resource-manager which will use CRD resources in order
// to ensure that they exist in a cluster/reconcile them in case somebody changed something.
func (b *Botanist) DeployGardenerResourceManager(ctx context.Context) error {
	var name = "gardener-resource-manager"

	defaultValues := map[string]interface{}{
		"podAnnotations": map[string]interface{}{
			"checksum/secret-" + name: b.CheckSums[name],
		},
		"replicas": b.Shoot.GetReplicas(1),
	}

	values, err := b.InjectSeedShootImages(defaultValues, common.GardenerResourceManagerImageName)
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(common.ChartPath, "seed-controlplane", "charts", name), b.Shoot.SeedNamespace, name, values, nil)
}

// DeployBackupEntryInGarden deploys the BackupEntry resource in garden.
func (b *Botanist) DeployBackupEntryInGarden(ctx context.Context) error {
	var (
		name        = common.GenerateBackupEntryName(b.Shoot.Info.Status.TechnicalID, b.Shoot.Info.Status.UID)
		backupEntry = &gardencorev1alpha1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: b.Shoot.Info.Namespace,
			},
		}
		bucketName string
		seedName   *string
	)

	if err := b.K8sGardenClient.Client().Get(ctx, kutil.Key(b.Shoot.Info.Namespace, name), backupEntry); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		// If backupEntry doesn't already exists, we have to assign backupBucket to backupEntry.
		bucketName = string(b.Seed.Info.UID)
		seedName = &b.Seed.Info.Name
	} else {
		bucketName = backupEntry.Spec.BucketName
		seedName = backupEntry.Spec.Seed
	}
	ownerRef := metav1.NewControllerRef(b.Shoot.Info, gardenv1beta1.SchemeGroupVersion.WithKind("Shoot"))
	blockOwnerDeletion := false
	ownerRef.BlockOwnerDeletion = &blockOwnerDeletion

	return kutil.CreateOrUpdate(ctx, b.K8sGardenClient.Client(), backupEntry, func() error {
		finalizers := sets.NewString(backupEntry.GetFinalizers()...)
		finalizers.Insert(gardenv1beta1.GardenerName)
		backupEntry.SetFinalizers(finalizers.UnsortedList())

		backupEntry.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerRef}
		backupEntry.Spec.BucketName = bucketName
		backupEntry.Spec.Seed = seedName
		return nil
	})
}
