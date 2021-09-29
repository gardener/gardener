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
	"path/filepath"
	"time"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	extensionscontrolplane "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/controlplane"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeployVerticalPodAutoscaler deploys the VPA into the shoot namespace in the seed.
func (b *Botanist) DeployVerticalPodAutoscaler(ctx context.Context) error {
	if !b.Shoot.WantsVerticalPodAutoscaler {
		return common.DeleteVpa(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace, true)
	}

	var (
		podLabels = map[string]interface{}{
			v1beta1constants.LabelNetworkPolicyToDNS:            "allowed",
			v1beta1constants.LabelNetworkPolicyToShootAPIServer: "allowed",
		}
		admissionController = map[string]interface{}{
			"replicas": b.Shoot.GetReplicas(1),
			"podAnnotations": map[string]interface{}{
				"checksum/secret-vpa-tls-certs":            b.LoadCheckSum(common.VPASecretName),
				"checksum/secret-vpa-admission-controller": b.LoadCheckSum("vpa-admission-controller"),
			},
			"podLabels": utils.MergeMaps(podLabels, map[string]interface{}{
				v1beta1constants.LabelNetworkPolicyFromShootAPIServer: "allowed",
			}),
			"enableServiceAccount": false,
		}
		exporter = map[string]interface{}{
			"enabled":  false,
			"replicas": 0,
		}
		recommender = map[string]interface{}{
			"replicas": b.Shoot.GetReplicas(1),
			"podAnnotations": map[string]interface{}{
				"checksum/secret-vpa-recommender": b.LoadCheckSum("vpa-recommender"),
			},
			"podLabels":                    podLabels,
			"enableServiceAccount":         false,
			"recommendationMarginFraction": gardencorev1beta1.DefaultRecommendationMarginFraction,
			"interval":                     gardencorev1beta1.DefaultRecommenderInterval,
		}
		updater = map[string]interface{}{
			"replicas": b.Shoot.GetReplicas(1),
			"podAnnotations": map[string]interface{}{
				"checksum/secret-vpa-updater": b.LoadCheckSum("vpa-updater"),
			},
			"podLabels":              podLabels,
			"enableServiceAccount":   false,
			"evictAfterOOMThreshold": gardencorev1beta1.DefaultEvictAfterOOMThreshold,
			"evictionRateBurst":      gardencorev1beta1.DefaultEvictionRateBurst,
			"evictionRateLimit":      gardencorev1beta1.DefaultEvictionRateLimit,
			"evictionTolerance":      gardencorev1beta1.DefaultEvictionTolerance,
			"interval":               gardencorev1beta1.DefaultUpdaterInterval,
		}
		defaultValues = map[string]interface{}{
			"admissionController": admissionController,
			"exporter":            exporter,
			"recommender":         recommender,
			"updater":             updater,
			"deploymentLabels": map[string]interface{}{
				v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
			},
			"clusterType": "shoot",
		}
	)

	if verticalPodAutoscaler := b.Shoot.GetInfo().Spec.Kubernetes.VerticalPodAutoscaler; verticalPodAutoscaler != nil {
		if val := verticalPodAutoscaler.EvictAfterOOMThreshold; val != nil {
			updater["evictAfterOOMThreshold"] = *val
		}
		if val := verticalPodAutoscaler.EvictionRateBurst; val != nil {
			updater["evictionRateBurst"] = *val
		}
		if val := verticalPodAutoscaler.EvictionRateLimit; val != nil {
			updater["evictionRateLimit"] = *val
		}
		if val := verticalPodAutoscaler.EvictionTolerance; val != nil {
			updater["evictionTolerance"] = *val
		}
		if val := verticalPodAutoscaler.UpdaterInterval; val != nil {
			updater["interval"] = *val
		}
		if val := verticalPodAutoscaler.RecommendationMarginFraction; val != nil {
			recommender["recommendationMarginFraction"] = *val
		}
		if val := verticalPodAutoscaler.RecommenderInterval; val != nil {
			recommender["interval"] = *val
		}
	}

	values, err := b.InjectSeedShootImages(defaultValues, charts.ImageNameVpaAdmissionController, charts.ImageNameVpaExporter, charts.ImageNameVpaRecommender, charts.ImageNameVpaUpdater)
	if err != nil {
		return err
	}
	values["global"] = map[string]interface{}{"images": values["images"]}

	return b.K8sSeedClient.ChartApplier().Apply(ctx, filepath.Join(charts.Path, "seed-bootstrap", "charts", "vpa", "charts", "runtime"), b.Shoot.SeedNamespace, "vpa", kubernetes.Values(values))
}

// HibernateControlPlane hibernates the entire control plane if the shoot shall be hibernated.
func (b *Botanist) HibernateControlPlane(ctx context.Context) error {
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

		// TODO: check if we can remove this mitigation once there is a garbage collection for VolumeAttachments (ref https://github.com/kubernetes/kubernetes/issues/77324)
		// Currently on hibernation Machines are forecefully deleted and machine-controller-manager does not wait volumes to be detached.
		// In this case kube-controller-manager cannot delete the corresponding VolumeAttachment objects and they are orphaned.
		// Such orphaned VolumeAttachments then prevent/block PV deletion. For more details see https://github.com/gardener/gardener-extension-provider-gcp/issues/172.
		// As the Nodes are already deleted, we can delete all VolumeAttachments.
		// Note: if custom csi-drivers are installed in the cluster (controllers running on the shoot itself), the VolumeAttachments will
		// probably not be finalized, because the controller pods are drained like all the other pods, so we still need to cleanup
		// VolumeAttachments of those csi-drivers.
		if err := CleanVolumeAttachments(ctxWithTimeOut, b.K8sShootClient.Client()); err != nil {
			return err
		}
	}

	// invalidate shoot client here before scaling down API server
	if err := b.ClientMap.InvalidateClient(keys.ForShoot(b.Shoot.GetInfo())); err != nil {
		return err
	}
	b.K8sShootClient = nil

	deployments := []string{
		v1beta1constants.DeploymentNameGardenerResourceManager,
		v1beta1constants.DeploymentNameKubeControllerManager,
		v1beta1constants.DeploymentNameKubeAPIServer,
	}
	for _, deployment := range deployments {
		if err := kubernetes.ScaleDeployment(ctx, b.K8sSeedClient.Client(), kutil.Key(b.Shoot.SeedNamespace, deployment), 0); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	if err := b.K8sSeedClient.Client().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: b.Shoot.SeedNamespace}}, kubernetes.DefaultDeleteOptions...); err != nil {
		if !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			return err
		}
	}

	if !b.Shoot.DisableDNS && !b.APIServerSNIEnabled() {
		if err := b.Shoot.Components.ControlPlane.KubeAPIServerService.Destroy(ctx); err != nil {
			return err
		}

		if err := b.Shoot.Components.ControlPlane.KubeAPIServerService.WaitCleanup(ctx); err != nil {
			return err
		}
	}

	if err := b.Shoot.Components.ControlPlane.KubeAPIServerSNI.Destroy(ctx); err != nil {
		return err
	}

	return client.IgnoreNotFound(b.ScaleETCDToZero(ctx))
}

// DefaultControlPlane creates the default deployer for the ControlPlane custom resource with the given purpose.
func (b *Botanist) DefaultControlPlane(purpose extensionsv1alpha1.Purpose) extensionscontrolplane.Interface {
	values := &extensionscontrolplane.Values{
		Name:      b.Shoot.GetInfo().Name,
		Namespace: b.Shoot.SeedNamespace,
		Purpose:   purpose,
	}

	switch purpose {
	case extensionsv1alpha1.Normal:
		values.Type = b.Shoot.GetInfo().Spec.Provider.Type
		values.ProviderConfig = b.Shoot.GetInfo().Spec.Provider.ControlPlaneConfig
		values.Region = b.Shoot.GetInfo().Spec.Region

	case extensionsv1alpha1.Exposure:
		values.Type = b.Seed.GetInfo().Spec.Provider.Type
		values.Region = b.Seed.GetInfo().Spec.Provider.Region
	}

	return extensionscontrolplane.New(
		b.Logger,
		b.K8sSeedClient.Client(),
		values,
		extensionscontrolplane.DefaultInterval,
		extensionscontrolplane.DefaultSevereThreshold,
		extensionscontrolplane.DefaultTimeout,
	)
}

// DeployControlPlane deploys or restores the ControlPlane custom resource (purpose normal).
func (b *Botanist) DeployControlPlane(ctx context.Context) error {
	b.Shoot.Components.Extensions.ControlPlane.SetInfrastructureProviderStatus(b.Shoot.Components.Extensions.Infrastructure.ProviderStatus())
	return b.deployOrRestoreControlPlane(ctx, b.Shoot.Components.Extensions.ControlPlane)
}

// DeployControlPlaneExposure deploys or restores the ControlPlane custom resource (purpose exposure).
func (b *Botanist) DeployControlPlaneExposure(ctx context.Context) error {
	return b.deployOrRestoreControlPlane(ctx, b.Shoot.Components.Extensions.ControlPlaneExposure)
}

func (b *Botanist) deployOrRestoreControlPlane(ctx context.Context, controlPlane extensionscontrolplane.Interface) error {
	if b.isRestorePhase() {
		return controlPlane.Restore(ctx, b.GetShootState())
	}
	return controlPlane.Deploy(ctx)
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
