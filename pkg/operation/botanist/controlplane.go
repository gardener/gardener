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
	"errors"
	"fmt"
	"hash/crc32"
	"path/filepath"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/utils/version"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	audit_internal "k8s.io/apiserver/pkg/apis/audit"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	auditv1alpha1 "k8s.io/apiserver/pkg/apis/audit/v1alpha1"
	auditv1beta1 "k8s.io/apiserver/pkg/apis/audit/v1beta1"
	auditvalidation "k8s.io/apiserver/pkg/apis/audit/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

	if _, err := controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), namespace, func() error {
		namespace.Annotations = map[string]string{
			v1beta1constants.DeprecatedShootUID: string(b.Shoot.Info.Status.UID),
		}
		namespace.Labels = map[string]string{
			v1beta1constants.DeprecatedGardenRole:    v1beta1constants.GardenRoleShoot,
			v1beta1constants.GardenRole:              v1beta1constants.GardenRoleShoot,
			v1beta1constants.LabelSeedProvider:       string(b.Seed.Info.Spec.Provider.Type),
			v1beta1constants.LabelShootProvider:      string(b.Shoot.Info.Spec.Provider.Type),
			v1beta1constants.LabelNetworkingProvider: string(b.Shoot.Info.Spec.Networking.Type),
			v1beta1constants.LabelBackupProvider:     string(b.Seed.Info.Spec.Provider.Type),
		}

		if b.Seed.Info.Spec.Backup != nil {
			namespace.Labels[v1beta1constants.LabelBackupProvider] = string(b.Seed.Info.Spec.Backup.Provider)
		}

		return nil
	}); err != nil {
		return err
	}

	b.SeedNamespaceObject = namespace
	return nil
}

// DeleteNamespace deletes the namespace in the Seed cluster which holds the control plane components. The built-in
// garbage collection in Kubernetes will automatically delete all resources which belong to this namespace. This
// comprises volumes and load balancers as well.
func (b *Botanist) DeleteNamespace(ctx context.Context) error {
	return b.deleteNamespace(ctx, b.Shoot.SeedNamespace)
}

func (b *Botanist) deleteNamespace(ctx context.Context, name string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	err := b.K8sSeedClient.Client().Delete(ctx, namespace, kubernetes.DefaultDeleteOptions...)
	if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
		return nil
	}
	return err
}

// DeleteKubeAPIServer deletes the kube-apiserver deployment in the Seed cluster which holds the Shoot's control plane.
func (b *Botanist) DeleteKubeAPIServer(ctx context.Context) error {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.DeploymentNameKubeAPIServer,
			Namespace: b.Shoot.SeedNamespace,
		},
	}
	return client.IgnoreNotFound(b.K8sSeedClient.Client().Delete(ctx, deploy, kubernetes.DefaultDeleteOptions...))
}

// DeployClusterAutoscaler deploys the cluster-autoscaler into the Shoot namespace in the Seed cluster. It is responsible
// for automatically scaling the worker pools of the Shoot.
func (b *Botanist) DeployClusterAutoscaler(ctx context.Context) error {
	if !b.Shoot.WantsClusterAutoscaler {
		return b.DeleteClusterAutoscaler(ctx)
	}

	var workerPools []map[string]interface{}
	for _, worker := range b.Shoot.MachineDeployments {

		workerPools = append(workerPools, map[string]interface{}{
			"name": worker.Name,
			"min":  worker.Minimum,
			"max":  worker.Maximum,
		})
	}

	defaultValues := map[string]interface{}{
		"podAnnotations": map[string]interface{}{
			"checksum/secret-cluster-autoscaler": b.CheckSums[v1beta1constants.DeploymentNameClusterAutoscaler],
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

	return b.ChartApplierSeed.Apply(ctx, filepath.Join(chartPathControlPlane, v1beta1constants.DeploymentNameClusterAutoscaler), b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameClusterAutoscaler, kubernetes.Values(values))
}

// DeleteClusterAutoscaler deletes the cluster-autoscaler deployment in the Seed cluster which holds the Shoot's control plane.
func (b *Botanist) DeleteClusterAutoscaler(ctx context.Context) error {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.DeploymentNameClusterAutoscaler,
			Namespace: b.Shoot.SeedNamespace,
		},
	}
	return client.IgnoreNotFound(b.K8sSeedClient.Client().Delete(ctx, deploy, kubernetes.DefaultDeleteOptions...))
}

// WakeUpControlPlane scales the replicas to 1 for the following deployments which are needed in case of shoot deletion:
// * etcd-events
// * etcd-main
// * kube-apiserver
// * kube-controller-manager
func (b *Botanist) WakeUpControlPlane(ctx context.Context) error {
	client := b.K8sSeedClient.Client()

	for _, etcd := range []string{v1beta1constants.ETCDEvents, v1beta1constants.ETCDMain} {
		if err := kubernetes.ScaleEtcd(ctx, client, kutil.Key(b.Shoot.SeedNamespace, etcd), 1); err != nil {
			return err
		}
	}
	if err := b.WaitUntilEtcdReady(ctx); err != nil {
		return err
	}

	if err := b.DeployKubeAPIServerService(ctx); err != nil {
		return err
	}

	if err := b.WaitUntilKubeAPIServerServiceIsReady(ctx); err != nil {
		return err
	}

	if err := kubernetes.ScaleDeployment(ctx, client, kutil.Key(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer), 1); err != nil {
		return err
	}
	if err := b.WaitUntilKubeAPIServerReady(ctx); err != nil {
		return err
	}

	for _, deployment := range []string{
		v1beta1constants.DeploymentNameKubeControllerManager,
		v1beta1constants.DeploymentNameGardenerResourceManager,
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

	if b.K8sShootClient != nil {
		ctxWithTimeOut, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		// If a shoot is hibernated we only want to scale down the entire control plane if no nodes exist anymore. The node-lifecycle-controller
		// inside KCM is responsible for deleting Node objects of terminated/non-existing VMs, so let's wait for that before scaling down.
		if err := b.WaitUntilNodesDeleted(ctxWithTimeOut); err != nil {
			return err
		}

		// Also wait for all Pods to reflect the correct state before scaling down the control plane.
		// KCM should remove all Pods in the cluster that are bound to Nodes that no longer exist and
		// therefore there should be no Pods with state `Running` anymore.
		if err := b.WaitUntilNoPodRunning(ctxWithTimeOut); err != nil {
			return err
		}

		// Also wait for all Endpoints to not contain any IPs from the Shoot's PodCIDR.
		// This is to make sure that the Endpoints objects also reflect the correct state of the hibernated cluster.
		// Otherwise this could cause timeouts in user-defined webhooks for CREATE Pods or Nodes on wakeup.
		if err := b.WaitUntilEndpointsDoNotContainPodIPs(ctxWithTimeOut); err != nil {
			return err
		}
	}

	deployments := []string{
		v1beta1constants.DeploymentNameGardenerResourceManager,
		v1beta1constants.DeploymentNameKubeControllerManager,
		v1beta1constants.DeploymentNameKubeAPIServer,
	}
	for _, deployment := range deployments {
		if err := kubernetes.ScaleDeployment(ctx, c, kutil.Key(b.Shoot.SeedNamespace, deployment), 0); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	if err := c.Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: b.Shoot.SeedNamespace}}, kubernetes.DefaultDeleteOptions...); err != nil {
		if !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			return err
		}
	}

	if !b.Shoot.DisableDNS {
		if err := c.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: b.Shoot.SeedNamespace}}, kubernetes.DefaultDeleteOptions...); client.IgnoreNotFound(err) != nil {
			return err
		}

		if err := flow.Parallel(
			func(ctx context.Context) error { return b.DestroyInternalDomainDNSRecord(ctx) },
			func(ctx context.Context) error { return b.DestroyExternalDomainDNSRecord(ctx) },
			func(ctx context.Context) error { return b.DestroyIngressDNSRecord(ctx) },
		)(ctx); err != nil {
			return err
		}
	}

	for _, etcd := range []string{v1beta1constants.ETCDEvents, v1beta1constants.ETCDMain} {
		if err := kubernetes.ScaleEtcd(ctx, c, kutil.Key(b.Shoot.SeedNamespace, etcd), 0); client.IgnoreNotFound(err) != nil {
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
		providerConfig *runtime.RawExtension
	)

	if cfg := b.Shoot.Info.Spec.Provider.ControlPlaneConfig; cfg != nil {
		providerConfig = &runtime.RawExtension{
			Raw: cfg.Raw,
		}
	}

	_, err := controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), cp, func() error {
		metav1.SetMetaDataAnnotation(&cp.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		cp.Spec = extensionsv1alpha1.ControlPlaneSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           string(b.Shoot.Info.Spec.Provider.Type),
				ProviderConfig: providerConfig,
			},
			Region: b.Shoot.Info.Spec.Region,
			SecretRef: corev1.SecretReference{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: cp.Namespace,
			},
			InfrastructureProviderStatus: &runtime.RawExtension{
				Raw: b.Shoot.InfrastructureStatus,
			},
		}
		return nil
	})
	return err
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

	_, err := controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), cp, func() error {
		metav1.SetMetaDataAnnotation(&cp.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		cp.Spec = extensionsv1alpha1.ControlPlaneSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: b.Seed.Info.Spec.Provider.Type,
			},
			Region:  b.Seed.Info.Spec.Provider.Region,
			Purpose: purpose,
			SecretRef: corev1.SecretReference{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: cp.Namespace,
			},
		}
		return nil
	})
	return err
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
	obj := &extensionsv1alpha1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: b.Shoot.SeedNamespace,
			Name:      name},
	}

	if err := common.ConfirmDeletion(ctx, b.K8sSeedClient.Client(), obj); err != nil {
		return err
	}

	return client.IgnoreNotFound(b.K8sSeedClient.Client().Delete(ctx, obj))
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
		return gardencorev1beta1helper.DetermineError(err, fmt.Sprintf("failed to create control plane: %v", err))
	}
	return nil
}

// WaitUntilControlPlaneExposureDeleted waits until the control plane resource with purpose `exposure` has been deleted.
func (b *Botanist) WaitUntilControlPlaneExposureDeleted(ctx context.Context) error {
	return b.waitUntilControlPlaneDeleted(ctx, b.Shoot.Info.Name+controlPlaneExposureSuffix)
}

// WaitUntilControlPlaneDeleted waits until the control plane resource has been deleted.
func (b *Botanist) WaitUntilControlPlaneDeleted(ctx context.Context) error {
	return b.waitUntilControlPlaneDeleted(ctx, b.Shoot.Info.Name)
}

// waitUntilControlPlaneDeleted waits until the control plane resource with the following name has been deleted.
func (b *Botanist) waitUntilControlPlaneDeleted(ctx context.Context, name string) error {
	var lastError *gardencorev1beta1.LastError

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
		return retry.MinorError(gardencorev1beta1helper.WrapWithLastError(fmt.Errorf("control plane is not yet deleted"), lastError))
	}); err != nil {
		message := "Failed to delete control plane"
		if lastError != nil {
			return gardencorev1beta1helper.DetermineError(errors.New(lastError.Description), fmt.Sprintf("%s: %s", message, lastError.Description))
		}
		return gardencorev1beta1helper.DetermineError(err, fmt.Sprintf("%s: %s", message, err.Error()))
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

	return b.ChartApplierSeed.Apply(ctx, filepath.Join(common.ChartPath, "seed-controlplane", "charts", name), b.Shoot.SeedNamespace, name, kubernetes.Values(values))
}

// DeployBackupEntryInGarden deploys the BackupEntry resource in garden.
func (b *Botanist) DeployBackupEntryInGarden(ctx context.Context) error {
	var (
		name        = common.GenerateBackupEntryName(b.Shoot.Info.Status.TechnicalID, b.Shoot.Info.Status.UID)
		backupEntry = &gardencorev1beta1.BackupEntry{
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
		seedName = backupEntry.Spec.SeedName
	}
	ownerRef := metav1.NewControllerRef(b.Shoot.Info, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot"))
	blockOwnerDeletion := false
	ownerRef.BlockOwnerDeletion = &blockOwnerDeletion

	_, err := controllerutil.CreateOrUpdate(ctx, b.K8sGardenClient.Client(), backupEntry, func() error {
		metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		finalizers := sets.NewString(backupEntry.GetFinalizers()...)
		finalizers.Insert(gardencorev1beta1.GardenerName)
		backupEntry.SetFinalizers(finalizers.UnsortedList())

		backupEntry.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerRef}
		backupEntry.Spec.BucketName = bucketName
		backupEntry.Spec.SeedName = seedName
		return nil
	})
	return err
}

const (
	auditPolicyConfigMapDataKey = "policy"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	decoder       = codecs.UniversalDecoder()
)

func init() {
	_ = auditv1alpha1.AddToScheme(runtimeScheme)
	_ = auditv1beta1.AddToScheme(runtimeScheme)
	_ = auditv1.AddToScheme(runtimeScheme)
	_ = audit_internal.AddToScheme(runtimeScheme)
}

// getResourcesForAPIServer returns the cpu and memory requirements for API server based on nodeCount
func getResourcesForAPIServer(nodeCount int32, scalingClass string) (string, string, string, string) {
	var (
		validScalingClasses = sets.NewString("small", "medium", "large", "xlarge", "2xlarge")
		cpuRequest          string
		memoryRequest       string
		cpuLimit            string
		memoryLimit         string
	)

	if !validScalingClasses.Has(scalingClass) {
		switch {
		case nodeCount <= 2:
			scalingClass = "small"
		case nodeCount <= 10:
			scalingClass = "medium"
		case nodeCount <= 50:
			scalingClass = "large"
		case nodeCount <= 100:
			scalingClass = "xlarge"
		default:
			scalingClass = "2xlarge"
		}
	}

	switch {
	case scalingClass == "small":
		cpuRequest = "800m"
		memoryRequest = "800Mi"

		cpuLimit = "1000m"
		memoryLimit = "1200Mi"
	case scalingClass == "medium":
		cpuRequest = "1000m"
		memoryRequest = "1100Mi"

		cpuLimit = "1200m"
		memoryLimit = "1900Mi"
	case scalingClass == "large":
		cpuRequest = "1200m"
		memoryRequest = "1600Mi"

		cpuLimit = "1500m"
		memoryLimit = "3900Mi"
	case scalingClass == "xlarge":
		cpuRequest = "2500m"
		memoryRequest = "5200Mi"

		cpuLimit = "3000m"
		memoryLimit = "5900Mi"
	case scalingClass == "2xlarge":
		cpuRequest = "3000m"
		memoryRequest = "5200Mi"

		cpuLimit = "4000m"
		memoryLimit = "7800Mi"
	}

	return cpuRequest, memoryRequest, cpuLimit, memoryLimit
}

func (b *Botanist) deployNetworkPolicies(ctx context.Context, denyAll bool) error {
	var (
		globalNetworkPoliciesValues = map[string]interface{}{
			"blockedAddresses": b.Seed.Info.Spec.Networks.BlockCIDRs,
			"denyAll":          denyAll,
		}
		excludeNets = []string{}

		values = map[string]interface{}{}
	)

	excludeNets = append(excludeNets, b.Seed.Info.Spec.Networks.BlockCIDRs...)

	var shootCIDRNetworks []string
	if v := b.Shoot.GetNodeNetwork(); v != nil {
		shootCIDRNetworks = append(shootCIDRNetworks, *v)
	}
	if v := b.Shoot.Info.Spec.Networking.Pods; v != nil {
		shootCIDRNetworks = append(shootCIDRNetworks, *v)
	}
	if v := b.Shoot.Info.Spec.Networking.Services; v != nil {
		shootCIDRNetworks = append(shootCIDRNetworks, *v)
	}
	shootNetworkValues, err := common.ExceptNetworks(shootCIDRNetworks, excludeNets...)
	if err != nil {
		return err
	}
	values["clusterNetworks"] = shootNetworkValues

	allCIDRNetworks := []string{b.Seed.Info.Spec.Networks.Pods, b.Seed.Info.Spec.Networks.Services}
	if v := b.Seed.Info.Spec.Networks.Nodes; v != nil {
		allCIDRNetworks = append(allCIDRNetworks, *v)
	}
	allCIDRNetworks = append(allCIDRNetworks, shootCIDRNetworks...)
	allCIDRNetworks = append(allCIDRNetworks, excludeNets...)

	privateNetworks, err := common.ToExceptNetworks(common.AllPrivateNetworkBlocks(), allCIDRNetworks...)
	if err != nil {
		return err
	}

	globalNetworkPoliciesValues["privateNetworks"] = privateNetworks
	values["global-network-policies"] = globalNetworkPoliciesValues

	return b.ChartApplierSeed.Apply(ctx, filepath.Join(chartPathControlPlane, "network-policies"), b.Shoot.SeedNamespace, "network-policies", kubernetes.Values(values))
}

// DeployNetworkPolicies creates a network policies in a Shoot cluster's namespace that
// deny all traffic and allow certain components to use annotations to declare their desire
// to transmit/receive traffic to/from other Pods/IP addresses.
func (b *Botanist) DeployNetworkPolicies(ctx context.Context) error {
	return b.deployNetworkPolicies(ctx, true)
}

// DeployKubeAPIServerService deploys kube-apiserver service.
func (b *Botanist) DeployKubeAPIServerService(ctx context.Context) error {
	const name = "kube-apiserver-service"
	return b.ChartApplierSeed.Apply(ctx, filepath.Join(chartPathControlPlane, name), b.Shoot.SeedNamespace, name)
}

// DeployKubeAPIServer deploys kube-apiserver deployment.
func (b *Botanist) DeployKubeAPIServer(ctx context.Context) error {
	hvpaEnabled := gardenletfeatures.FeatureGate.Enabled(features.HVPA)
	memoryMetricForHpaEnabled := false

	if b.ShootedSeed != nil {
		// Override for shooted seeds
		hvpaEnabled = gardenletfeatures.FeatureGate.Enabled(features.HVPAForShootedSeed)
		memoryMetricForHpaEnabled = true
	}

	var (
		minReplicas int32 = 1
		maxReplicas int32 = 4

		shootNetworks = map[string]interface{}{
			"services": b.Shoot.Networks.Services.String(),
			"pods":     b.Shoot.Networks.Pods.String(),
		}

		defaultValues = map[string]interface{}{
			"etcdServicePort":           2379,
			"kubernetesVersion":         b.Shoot.Info.Spec.Kubernetes.Version,
			"enableBasicAuthentication": gardencorev1beta1helper.ShootWantsBasicAuthentication(b.Shoot.Info),
			"probeCredentials":          b.APIServerHealthCheckToken,
			"securePort":                443,
			"podAnnotations": map[string]interface{}{
				"checksum/secret-ca":                     b.CheckSums[v1beta1constants.SecretNameCACluster],
				"checksum/secret-ca-front-proxy":         b.CheckSums[v1beta1constants.SecretNameCAFrontProxy],
				"checksum/secret-kube-apiserver":         b.CheckSums[v1beta1constants.DeploymentNameKubeAPIServer],
				"checksum/secret-kube-aggregator":        b.CheckSums["kube-aggregator"],
				"checksum/secret-kube-apiserver-kubelet": b.CheckSums["kube-apiserver-kubelet"],
				"checksum/secret-static-token":           b.CheckSums[common.StaticTokenSecretName],
				"checksum/secret-vpn-seed":               b.CheckSums["vpn-seed"],
				"checksum/secret-vpn-seed-tlsauth":       b.CheckSums["vpn-seed-tlsauth"],
				"checksum/secret-service-account-key":    b.CheckSums["service-account-key"],
				"checksum/secret-etcd-ca":                b.CheckSums[v1beta1constants.SecretNameCAETCD],
				"checksum/secret-etcd-client-tls":        b.CheckSums["etcd-client-tls"],
			},
			"hvpa": map[string]interface{}{
				"enabled": hvpaEnabled,
			},
			"hpa": map[string]interface{}{
				"memoryMetricForHpaEnabled": memoryMetricForHpaEnabled,
			},
		}
	)

	if v := b.Shoot.GetNodeNetwork(); v != nil {
		shootNetworks["nodes"] = *v
	}
	defaultValues["shootNetworks"] = shootNetworks

	enableEtcdEncryption, err := version.CheckVersionMeetsConstraint(b.Shoot.Info.Spec.Kubernetes.Version, ">= 1.13")
	if err != nil {
		return err
	}
	if enableEtcdEncryption {
		defaultValues["enableEtcdEncryption"] = true
		defaultValues["podAnnotations"].(map[string]interface{})["checksum/secret-etcd-encryption"] = b.CheckSums[common.EtcdEncryptionSecretName]
	}

	if gardencorev1beta1helper.ShootWantsBasicAuthentication(b.Shoot.Info) {
		defaultValues["podAnnotations"].(map[string]interface{})["checksum/secret-"+common.BasicAuthSecretName] = b.CheckSums[common.BasicAuthSecretName]
	}

	foundDeployment := true
	deployment := &appsv1.Deployment{}
	if err := b.K8sSeedClient.Client().Get(context.TODO(), kutil.Key(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer), deployment); err != nil && !apierrors.IsNotFound(err) {
		return err
	} else if apierrors.IsNotFound(err) {
		foundDeployment = false
	}

	if b.ShootedSeed != nil && !hvpaEnabled {
		var (
			apiServer  = b.ShootedSeed.APIServer
			autoscaler = apiServer.Autoscaler
		)
		defaultValues["replicas"] = *apiServer.Replicas
		minReplicas = *autoscaler.MinReplicas
		maxReplicas = autoscaler.MaxReplicas

		defaultValues["apiServerResources"] = map[string]interface{}{
			"requests": map[string]interface{}{
				"cpu":    "1750m",
				"memory": "2Gi",
			},
			"limits": map[string]interface{}{
				"cpu":    "4000m",
				"memory": "8Gi",
			},
		}
	} else {
		replicas := deployment.Spec.Replicas

		// As kube-apiserver HPA manages the number of replicas, we have to maintain current number of replicas
		// otherwise keep the value to default
		if replicas != nil && *replicas > 0 {
			defaultValues["replicas"] = *replicas
		}
		// If the shoot is hibernated then we want to keep the number of replicas (scale down happens later).
		if b.Shoot.HibernationEnabled && (replicas == nil || *replicas == 0) {
			defaultValues["replicas"] = 0
		}

		var cpuRequest, memoryRequest, cpuLimit, memoryLimit string
		if hvpaEnabled {
			cpuRequest, memoryRequest, cpuLimit, memoryLimit = getResourcesForAPIServer(b.Shoot.GetMinNodeCount(), b.Shoot.Info.Annotations[common.ShootAlphaScalingAPIServerClass])
		} else {
			cpuRequest, memoryRequest, cpuLimit, memoryLimit = getResourcesForAPIServer(b.Shoot.GetMaxNodeCount(), b.Shoot.Info.Annotations[common.ShootAlphaScalingAPIServerClass])
		}
		defaultValues["apiServerResources"] = map[string]interface{}{
			"limits": map[string]interface{}{
				"cpu":    cpuLimit,
				"memory": memoryLimit,
			},
			"requests": map[string]interface{}{
				"cpu":    cpuRequest,
				"memory": memoryRequest,
			},
		}
	}

	if foundDeployment && hvpaEnabled {
		// Deployment is already created AND is controlled by HVPA
		// Keep the "resources" as it is.
		for k := range deployment.Spec.Template.Spec.Containers {
			v := &deployment.Spec.Template.Spec.Containers[k]
			if v.Name == "kube-apiserver" {
				defaultValues["apiServerResources"] = v.Resources.DeepCopy()
				break
			}
		}
	}

	defaultValues["minReplicas"] = minReplicas
	defaultValues["maxReplicas"] = maxReplicas

	var (
		apiServerConfig              = b.Shoot.Info.Spec.Kubernetes.KubeAPIServer
		admissionPlugins             = kubernetes.GetAdmissionPluginsForVersion(b.Shoot.Info.Spec.Kubernetes.Version)
		serviceAccountTokenIssuerURL = fmt.Sprintf("https://%s", b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, true))
		serviceAccountConfigVals     = map[string]interface{}{}
	)

	if apiServerConfig != nil {
		defaultValues["featureGates"] = apiServerConfig.FeatureGates
		defaultValues["runtimeConfig"] = apiServerConfig.RuntimeConfig

		if apiServerConfig.OIDCConfig != nil {
			defaultValues["oidcConfig"] = apiServerConfig.OIDCConfig
		}

		if serviceAccountConfig := apiServerConfig.ServiceAccountConfig; serviceAccountConfig != nil {
			if issuer := serviceAccountConfig.Issuer; issuer != nil {
				serviceAccountTokenIssuerURL = *issuer
			}

			if signingKeySecret := serviceAccountConfig.SigningKeySecret; signingKeySecret != nil {
				signingKey, err := common.GetServiceAccountSigningKeySecret(context.TODO(), b.K8sGardenClient.Client(), b.Shoot.Info.Namespace, signingKeySecret.Name)
				if err != nil {
					return err
				}

				serviceAccountConfigVals["signingKey"] = signingKey
			}
		}

		if apiServerConfig.APIAudiences != nil {
			defaultValues["apiAudiences"] = apiServerConfig.APIAudiences
		}

		for _, plugin := range apiServerConfig.AdmissionPlugins {
			pluginOverwritesDefault := false

			for i, defaultPlugin := range admissionPlugins {
				if defaultPlugin.Name == plugin.Name {
					pluginOverwritesDefault = true
					admissionPlugins[i] = plugin
					break
				}
			}

			if !pluginOverwritesDefault {
				admissionPlugins = append(admissionPlugins, plugin)
			}
		}

		if apiServerConfig.AuditConfig != nil &&
			apiServerConfig.AuditConfig.AuditPolicy != nil &&
			apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef != nil {
			auditPolicy, err := b.getAuditPolicy(apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef.Name, b.Shoot.Info.Namespace)
			if err != nil {
				return fmt.Errorf("retrieving audit policy from the ConfigMap '%v' failed with reason '%v'", apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef.Name, err)
			}
			defaultValues["auditConfig"] = map[string]interface{}{
				"auditPolicy": auditPolicy,
			}
		}
	}

	serviceAccountConfigVals["issuer"] = serviceAccountTokenIssuerURL
	defaultValues["serviceAccountConfig"] = serviceAccountConfigVals

	defaultValues["admissionPlugins"] = admissionPlugins

	values, err := b.InjectSeedShootImages(defaultValues,
		common.KubeAPIServerImageName,
		common.VPNSeedImageName,
		common.BlackboxExporterImageName,
		common.AlpineIptablesImageName,
	)
	if err != nil {
		return err
	}

	// If HVPA feature gate is enabled then we should delete the old HPA and VPA resources as
	// the HVPA controller will create its own for the kube-apiserver deployment.
	if hvpaEnabled {
		for _, obj := range []struct {
			apiGroup string
			version  string
			kind     string
			name     string
		}{
			{"autoscaling", "v2beta1", "HorizontalPodAutoscaler", v1beta1constants.DeploymentNameKubeAPIServer},
			{"autoscaling.k8s.io", "v1beta2", "VerticalPodAutoscaler", v1beta1constants.DeploymentNameKubeAPIServer + "-vpa"},
		} {
			u := &unstructured.Unstructured{}
			u.SetName(obj.name)
			u.SetNamespace(b.Shoot.SeedNamespace)
			u.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   obj.apiGroup,
				Version: obj.version,
				Kind:    obj.kind,
			})
			if err := b.K8sSeedClient.Client().Delete(context.TODO(), u); client.IgnoreNotFound(err) != nil {
				return err
			}
		}
	} else {
		// If HVPA is disabled, delete any HVPA that was already deployed
		u := &unstructured.Unstructured{}
		u.SetName(v1beta1constants.DeploymentNameKubeAPIServer)
		u.SetNamespace(b.Shoot.SeedNamespace)
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "autoscaling.k8s.io",
			Version: "v1alpha1",
			Kind:    "Hvpa",
		})
		if err := b.K8sSeedClient.Client().Delete(context.TODO(), u); err != nil {
			if !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
				return err
			}
		}
	}

	return b.ChartApplierSeed.Apply(ctx, filepath.Join(chartPathControlPlane, v1beta1constants.DeploymentNameKubeAPIServer), b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer, kubernetes.Values(values))
}

func (b *Botanist) getAuditPolicy(name, namespace string) (string, error) {
	auditPolicyCm := &corev1.ConfigMap{}
	if err := b.K8sGardenClient.Client().Get(context.TODO(), kutil.Key(namespace, name), auditPolicyCm); err != nil {
		return "", err
	}
	auditPolicy, ok := auditPolicyCm.Data[auditPolicyConfigMapDataKey]
	if !ok {
		return "", fmt.Errorf("missing '.data.policy' in audit policy configmap %v/%v", namespace, name)
	}
	if len(auditPolicy) == 0 {
		return "", fmt.Errorf("empty audit policy. Provide non-empty audit policy")
	}
	auditPolicyObj, schemaVersion, err := decoder.Decode([]byte(auditPolicy), nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decode the provided audit policy err=%v", err)
	}

	if isValidVersion, err := IsValidAuditPolicyVersion(b.ShootVersion(), schemaVersion); err != nil {
		return "", err
	} else if !isValidVersion {
		return "", fmt.Errorf("your shoot cluster version %q is not compatible with audit policy version %q", b.ShootVersion(), schemaVersion.GroupVersion().String())
	}

	auditPolicyInternal, ok := auditPolicyObj.(*audit_internal.Policy)
	if !ok {
		return "", fmt.Errorf("failure to cast to audit Policy type: %v", schemaVersion)
	}
	errList := auditvalidation.ValidatePolicy(auditPolicyInternal)
	if len(errList) != 0 {
		return "", fmt.Errorf("provided invalid audit policy err=%v", errList)
	}
	return auditPolicy, nil
}

// IsValidAuditPolicyVersion checks whether the api server support the provided audit policy apiVersion
func IsValidAuditPolicyVersion(shootVersion string, schemaVersion *schema.GroupVersionKind) (bool, error) {
	auditGroupVersion := schemaVersion.GroupVersion().String()

	if auditGroupVersion == "audit.k8s.io/v1" {
		return version.CheckVersionMeetsConstraint(shootVersion, ">= v1.12")
	}
	return true, nil
}

// DeployKubeControllerManager deploys kube-controller-manager deployment.
func (b *Botanist) DeployKubeControllerManager(ctx context.Context) error {
	defaultValues := map[string]interface{}{
		"clusterName":       b.Shoot.SeedNamespace,
		"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
		"podNetwork":        b.Shoot.Networks.Pods.String(),
		"serviceNetwork":    b.Shoot.Networks.Services.String(),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-ca":                             b.CheckSums[v1beta1constants.SecretNameCACluster],
			"checksum/secret-kube-controller-manager":        b.CheckSums[v1beta1constants.DeploymentNameKubeControllerManager],
			"checksum/secret-kube-controller-manager-server": b.CheckSums[common.KubeControllerManagerServerName],
			"checksum/secret-service-account-key":            b.CheckSums["service-account-key"],
		},
		"podLabels": map[string]interface{}{
			v1beta1constants.LabelPodMaintenanceRestart: "true",
		},
	}

	if b.Shoot.HibernationEnabled == b.Shoot.Info.Status.IsHibernated {
		// Keep the replica count same if the shoot is not transitioning from/to hibernation state
		// otherwise this may interfere with dependency-watchdog
		replicaCount, err := common.CurrentReplicaCount(b.K8sSeedClient.Client(), b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeControllerManager)
		if err != nil {
			return err
		}
		defaultValues["replicas"] = replicaCount
	}

	controllerManagerConfig := b.Shoot.Info.Spec.Kubernetes.KubeControllerManager
	if controllerManagerConfig != nil {
		defaultValues["featureGates"] = controllerManagerConfig.FeatureGates

		if controllerManagerConfig.HorizontalPodAutoscalerConfig != nil {
			defaultValues["horizontalPodAutoscaler"] = controllerManagerConfig.HorizontalPodAutoscalerConfig
		}

		if controllerManagerConfig.NodeCIDRMaskSize != nil {
			defaultValues["nodeCIDRMaskSize"] = *controllerManagerConfig.NodeCIDRMaskSize
		}
	}

	values, err := b.InjectSeedShootImages(defaultValues, common.KubeControllerManagerImageName)
	if err != nil {
		return err
	}

	return b.ChartApplierSeed.Apply(ctx, filepath.Join(chartPathControlPlane, v1beta1constants.DeploymentNameKubeControllerManager), b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeControllerManager, kubernetes.Values(values))
}

// DeployKubeScheduler deploys kube-scheduler deployment.
func (b *Botanist) DeployKubeScheduler(ctx context.Context) error {
	defaultValues := map[string]interface{}{
		"replicas":          b.Shoot.GetReplicas(1),
		"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
		"podAnnotations": map[string]interface{}{
			"checksum/secret-kube-scheduler":        b.CheckSums[v1beta1constants.DeploymentNameKubeScheduler],
			"checksum/secret-kube-scheduler-server": b.CheckSums[common.KubeSchedulerServerName],
		},
	}

	if b.ShootedSeed != nil {
		defaultValues["resources"] = map[string]interface{}{
			"limits": map[string]interface{}{
				"cpu":    "300m",
				"memory": "512Mi",
			},
		}
	}

	schedulerConfig := b.Shoot.Info.Spec.Kubernetes.KubeScheduler
	if schedulerConfig != nil {
		defaultValues["featureGates"] = schedulerConfig.FeatureGates
	}

	values, err := b.InjectSeedShootImages(defaultValues, common.KubeSchedulerImageName)
	if err != nil {
		return err
	}

	return b.ChartApplierSeed.Apply(ctx, filepath.Join(chartPathControlPlane, v1beta1constants.DeploymentNameKubeScheduler), b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeScheduler, kubernetes.Values(values))
}

// DeployETCD deploys two etcd clusters via StatefulSets. The first etcd cluster (called 'main') is used for all the
// data the Shoot Kubernetes cluster needs to store, whereas the second etcd luster (called 'events') is only used to
// store the events data. The objectstore is also set up to store the backups.
func (b *Botanist) DeployETCD(ctx context.Context) error {
	hvpaEnabled := gardenletfeatures.FeatureGate.Enabled(features.HVPA)
	if b.ShootedSeed != nil {
		// Override for shooted seeds
		hvpaEnabled = gardenletfeatures.FeatureGate.Enabled(features.HVPAForShootedSeed)
	}

	defaultValues := map[string]interface{}{
		"annotations": map[string]string{
			v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
		},
		"podAnnotations": map[string]interface{}{
			"checksum/secret-etcd-ca":          b.CheckSums[v1beta1constants.SecretNameCAETCD],
			"checksum/secret-etcd-server-cert": b.CheckSums[common.EtcdServerTLS],
			"checksum/secret-etcd-client-tls":  b.CheckSums[common.EtcdClientTLS],
		},
		"storageCapacity": b.Seed.GetValidVolumeSize("10Gi"),
	}

	values, err := b.InjectSeedShootImages(
		defaultValues,
		common.ETCDImageName,
		common.ETCDBackupRestoreImageName,
	)
	if err != nil {
		return err
	}

	for _, role := range []string{common.EtcdRoleMain, common.EtcdRoleEvents} {
		var (
			etcdValues    = make(map[string]interface{})
			sidecarValues = make(map[string]interface{})
			hvpaValues    = make(map[string]interface{})

			name = fmt.Sprintf("etcd-%s", role)
		)

		foundEtcd := true
		etcd := &druidv1alpha1.Etcd{}
		if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, name), etcd); client.IgnoreNotFound(err) != nil {
			return err
		} else if apierrors.IsNotFound(err) {
			foundEtcd = false
		}

		statefulSetName := name
		if foundEtcd && etcd.Status.Etcd.Name != "" {
			statefulSetName = etcd.Status.Etcd.Name
		}

		foundStatefulset := true
		statefulSet := &appsv1.StatefulSet{}
		if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, statefulSetName), statefulSet); client.IgnoreNotFound(err) != nil {
			return err
		} else if apierrors.IsNotFound(err) {
			foundStatefulset = false
		}

		defragmentSchedule, err := DetermineDefragmentSchedule(b.Shoot.Info, etcd)
		if err != nil {
			return err
		}
		etcdValues["defragmentSchedule"] = defragmentSchedule

		hvpaValues["enabled"] = hvpaEnabled
		hvpaValues["maintenanceWindow"] = b.Shoot.Info.Spec.Maintenance.TimeWindow

		switch role {
		case common.EtcdRoleMain:
			etcdValues["metrics"] = "extensive" // etcd-main emits extensive (histogram) metrics

			hvpaValues["minAllowed"] = map[string]interface{}{
				"cpu":    "200m",
				"memory": "700M",
			}

			if b.Seed.Info.Spec.Backup != nil {
				secret := &corev1.Secret{}
				if err := b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, common.BackupSecretName), secret); err != nil {
					return err
				}

				snapshotSchedule, err := DetermineBackupSchedule(b.Shoot.Info, etcd)
				if err != nil {
					return err
				}

				sidecarValues["backup"] = map[string]interface{}{
					"provider":                 b.Seed.Info.Spec.Backup.Provider,
					"secretRefName":            common.BackupSecretName,
					"prefix":                   common.GenerateBackupEntryName(b.Shoot.Info.Status.TechnicalID, b.Shoot.Info.Status.UID),
					"container":                string(secret.Data[common.BackupBucketName]),
					"fullSnapshotSchedule":     snapshotSchedule,
					"deltaSnapshotMemoryLimit": "100Mi",
					"deltaSnapshotPeriod":      "5m",
				}
			}

		case common.EtcdRoleEvents:
			hvpaValues["minAllowed"] = map[string]interface{}{
				"cpu":    "50m",
				"memory": "200M",
			}
		}

		// TODO(georgekuruvillak): Remove this, once HVPA support updating resources in CRD spec
		if foundStatefulset && hvpaEnabled {
			// etcd is already created AND is controlled by HVPA
			// Keep the "resources" as it is.
			for k := range statefulSet.Spec.Template.Spec.Containers {
				v := &statefulSet.Spec.Template.Spec.Containers[k]
				if v.Name == "etcd" {
					etcdValues["resources"] = v.Resources.DeepCopy()
					break
				} else if v.Name == "backup-restore" {
					sidecarValues["resources"] = v.Resources.DeepCopy()
					break
				}
			}
		}

		if b.Shoot.HibernationEnabled {
			// Restore the replica count from capture statefulSet state.
			values["replicas"] = 0
			if foundEtcd {
				values["replicas"] = etcd.Spec.Replicas
			} else if foundStatefulset && statefulSet.Spec.Replicas != nil {
				values["replicas"] = *statefulSet.Spec.Replicas
			}
		}

		if !hvpaEnabled {
			// If HVPA is disabled, delete any HVPA that was already deployed
			u := &unstructured.Unstructured{}
			u.SetName(name)
			u.SetNamespace(b.Shoot.SeedNamespace)
			u.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "autoscaling.k8s.io",
				Version: "v1alpha1",
				Kind:    "Hvpa",
			})
			if err := b.K8sSeedClient.Client().Delete(ctx, u); err != nil {
				if !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
					return err
				}
			}
		}

		values["role"] = role
		values["etcd"] = etcdValues
		values["sidecar"] = sidecarValues
		values["hvpa"] = hvpaValues

		if err := b.ChartApplierSeed.Apply(ctx, filepath.Join(chartPathControlPlane, "etcd"), b.Shoot.SeedNamespace, name, kubernetes.Values(values)); err != nil {
			return err
		}
	}

	return nil
}

// CheckVPNConnection checks whether the VPN connection between the control plane and the shoot networks
// is established.
func (b *Botanist) CheckVPNConnection(ctx context.Context, logger *logrus.Entry) (bool, error) {
	podList := &corev1.PodList{}
	if err := b.K8sShootClient.Client().List(ctx, podList, client.InNamespace(metav1.NamespaceSystem), client.MatchingLabels{"app": "vpn-shoot"}); err != nil {
		return retry.SevereError(err)
	}

	var vpnPod *corev1.Pod
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			vpnPod = &pod
			break
		}
	}

	if vpnPod == nil {
		logger.Info("Waiting until a running vpn-shoot pod exists in the Shoot cluster...")
		return retry.MinorError(fmt.Errorf("no running vpn-shoot pod found yet in the shoot cluster"))
	}

	if err := b.K8sShootClient.CheckForwardPodPort(vpnPod.Namespace, vpnPod.Name, 0, 22); err != nil {
		logger.Info("Waiting until the VPN connection has been established...")
		return retry.MinorError(fmt.Errorf("could not forward to vpn-shoot pod: %v", err))
	}

	logger.Info("VPN connection has been established.")
	return retry.Ok()
}

// DetermineBackupSchedule determines the backup schedule based on the shoot creation and maintenance time window.
func DetermineBackupSchedule(shoot *gardencorev1beta1.Shoot, etcd *druidv1alpha1.Etcd) (string, error) {
	if etcd.Spec.Backup.FullSnapshotSchedule != nil {
		return *etcd.Spec.Backup.FullSnapshotSchedule, nil
	}

	schedule := "%d %d * * *"

	return determineSchedule(shoot, schedule, func(maintenanceTimeWindow *utils.MaintenanceTimeWindow, shootUID types.UID) string {
		// Randomize the snapshot timing daily but within last hour.
		// The 15 minutes buffer is set to snapshot upload time before actual maintenance window start.
		snapshotWindowBegin := maintenanceTimeWindow.Begin().Add(-1, -15, 0)
		randomMinutes := int(crc32.ChecksumIEEE([]byte(shootUID)) % 60)
		snapshotTime := snapshotWindowBegin.Add(0, randomMinutes, 0)
		return fmt.Sprintf(schedule, snapshotTime.Minute(), snapshotTime.Hour())
	})
}

// DetermineDefragmentSchedule determines the defragment schedule based on the shoot creation and maintenance time window.
func DetermineDefragmentSchedule(shoot *gardencorev1beta1.Shoot, etcd *druidv1alpha1.Etcd) (string, error) {
	if etcd.Spec.Etcd.DefragmentationSchedule != nil {
		return *etcd.Spec.Etcd.DefragmentationSchedule, nil
	}

	schedule := "%d %d */3 * *"

	return determineSchedule(shoot, schedule, func(maintenanceTimeWindow *utils.MaintenanceTimeWindow, shootUID types.UID) string {
		// Randomize the defragment timing but within the maintainence window.
		maintainenceWindowBegin := maintenanceTimeWindow.Begin()
		windowInMinutes := uint32(maintenanceTimeWindow.Duration().Minutes())
		randomMinutes := int(crc32.ChecksumIEEE([]byte(shootUID)) % windowInMinutes)
		maintenanceTime := maintainenceWindowBegin.Add(0, randomMinutes, 0)
		return fmt.Sprintf(schedule, maintenanceTime.Minute(), maintenanceTime.Hour())
	})
}

func determineSchedule(shoot *gardencorev1beta1.Shoot, schedule string, f func(*utils.MaintenanceTimeWindow, types.UID) string) (string, error) {
	var (
		begin, end string
		shootUID   types.UID
	)

	if shoot.Spec.Maintenance != nil && shoot.Spec.Maintenance.TimeWindow != nil {
		begin = shoot.Spec.Maintenance.TimeWindow.Begin
		end = shoot.Spec.Maintenance.TimeWindow.End
		shootUID = shoot.Status.UID
	}

	if len(begin) != 0 && len(end) != 0 {
		maintenanceTimeWindow, err := utils.ParseMaintenanceTimeWindow(begin, end)
		if err != nil {
			return "", err
		}

		if !maintenanceTimeWindow.Equal(utils.AlwaysTimeWindow) {
			return f(maintenanceTimeWindow, shootUID), nil
		}
	}

	creationMinute := shoot.CreationTimestamp.Minute()
	creationHour := shoot.CreationTimestamp.Hour()
	return fmt.Sprintf(schedule, creationMinute, creationHour), nil
}

// RestartControlPlanePods restarts (deletes) pods of the shoot control plane.
func (b *Botanist) RestartControlPlanePods(ctx context.Context) error {
	return b.K8sSeedClient.Client().DeleteAllOf(
		ctx,
		&corev1.Pod{},
		client.InNamespace(b.Shoot.SeedNamespace),
		client.MatchingLabels{v1beta1constants.LabelPodMaintenanceRestart: "true"},
	)
}
