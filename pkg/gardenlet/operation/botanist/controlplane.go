// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	extensionscontrolplane "github.com/gardener/gardener/pkg/component/extensions/controlplane"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func (b *Botanist) determineControllerReplicas(ctx context.Context, deploymentName string, defaultReplicas int32, controlledByDependencyWatchdog bool) (int32, error) {
	isCreateOrRestoreOperation := b.Shoot.GetInfo().Status.LastOperation != nil &&
		(b.Shoot.GetInfo().Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeCreate ||
			b.Shoot.GetInfo().Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeRestore)

	if (isCreateOrRestoreOperation && b.Shoot.HibernationEnabled) ||
		(!isCreateOrRestoreOperation && b.Shoot.HibernationEnabled && b.Shoot.GetInfo().Status.IsHibernated) {
		// Shoot is being created or restored with .spec.hibernation.enabled=true or
		// Shoot is being reconciled with .spec.hibernation.enabled=.status.isHibernated=true,
		// so keep the replicas which are already available.
		return kubernetesutils.CurrentReplicaCountForDeployment(ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, deploymentName)
	}
	if controlledByDependencyWatchdog && !isCreateOrRestoreOperation && !b.Shoot.HibernationEnabled && !b.Shoot.GetInfo().Status.IsHibernated {
		// The replicas of the component are controlled by dependency-watchdog and
		// Shoot is being reconciled with .spec.hibernation.enabled=.status.isHibernated=false,
		// so keep the replicas which are already available.
		return kubernetesutils.CurrentReplicaCountForDeployment(ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, deploymentName)
	}

	// If kube-apiserver is set to 0 replicas then we also want to return 0 here
	// since the controller is most likely not able to run w/o communicating to the Apiserver.
	if ptr.Deref(b.Shoot.Components.ControlPlane.KubeAPIServer.GetAutoscalingReplicas(), 0) == 0 {
		return 0, nil
	}

	// Shoot is being reconciled with .spec.hibernation.enabled!=.status.isHibernated, so deploy the controller.
	// In case the shoot is being hibernated then it will be scaled down to zero later after all machines are gone.
	return defaultReplicas, nil
}

// HibernateControlPlane hibernates the entire control plane if the shoot shall be hibernated.
func (b *Botanist) HibernateControlPlane(ctx context.Context) error {
	if b.ShootClientSet != nil {
		ctxWithTimeOut, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		if !b.Shoot.IsWorkerless {
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
			// Currently on hibernation Machines are forcefully deleted and machine-controller-manager does not wait volumes to be detached.
			// In this case kube-controller-manager cannot delete the corresponding VolumeAttachment objects and they are orphaned.
			// Such orphaned VolumeAttachments then prevent/block PV deletion. For more details see https://github.com/gardener/gardener-extension-provider-gcp/issues/172.
			// As the Nodes are already deleted, we can delete all VolumeAttachments.
			// Note: if custom csi-drivers are installed in the cluster (controllers running on the shoot itself), the VolumeAttachments will
			// probably not be finalized, because the controller pods are drained like all the other pods, so we still need to cleanup
			// VolumeAttachments of those csi-drivers.
			if err := CleanVolumeAttachments(ctxWithTimeOut, b.ShootClientSet.Client()); err != nil {
				return err
			}
		}
	}

	// invalidate shoot client here before scaling down API server
	if err := b.ShootClientMap.InvalidateClient(keys.ForShoot(b.Shoot.GetInfo())); err != nil {
		return err
	}
	b.ShootClientSet = nil

	deployments := []string{
		v1beta1constants.DeploymentNameGardenerResourceManager,
		v1beta1constants.DeploymentNameKubeControllerManager,
		v1beta1constants.DeploymentNameKubeAPIServer,
	}
	for _, deployment := range deployments {
		if err := kubernetesutils.ScaleDeployment(ctx, b.SeedClientSet.Client(), client.ObjectKey{Namespace: b.Shoot.ControlPlaneNamespace, Name: deployment}, 0); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	if err := waitUntilNoPodsExistAnymore(ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, deployments); err != nil {
		return err
	}

	if !b.ShootUsesDNS() {
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

	if err := client.IgnoreNotFound(b.ScaleETCDToZero(ctx)); err != nil {
		return err
	}

	return b.WaitUntilEtcdsReady(ctx)
}

// DefaultControlPlane creates the default deployer for the ControlPlane custom resource with the given purpose.
func (b *Botanist) DefaultControlPlane(purpose extensionsv1alpha1.Purpose) extensionscontrolplane.Interface {
	values := &extensionscontrolplane.Values{
		Name:      b.Shoot.GetInfo().Name,
		Namespace: b.Shoot.ControlPlaneNamespace,
		Purpose:   purpose,
	}

	// TODO(theoddora): Remove this after v1.123.0 was released when the Purpose field (exposure/normal) is removed.
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
		b.SeedClientSet.Client(),
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

func (b *Botanist) deployOrRestoreControlPlane(ctx context.Context, controlPlane extensionscontrolplane.Interface) error {
	if b.IsRestorePhase() {
		return controlPlane.Restore(ctx, b.Shoot.GetShootState())
	}
	return controlPlane.Deploy(ctx)
}

// RestoreControlPlane restores the ControlPlane custom resource (purpose normal)
func (b *Botanist) RestoreControlPlane(ctx context.Context) error {
	b.Shoot.Components.Extensions.ControlPlane.SetInfrastructureProviderStatus(b.Shoot.Components.Extensions.Infrastructure.ProviderStatus())
	return b.Shoot.Components.Extensions.ControlPlane.Restore(ctx, b.Shoot.GetShootState())
}

// RestartControlPlanePods restarts (deletes) pods of the shoot control plane.
func (b *Botanist) RestartControlPlanePods(ctx context.Context) error {
	return b.SeedClientSet.Client().DeleteAllOf(
		ctx,
		&corev1.Pod{},
		client.InNamespace(b.Shoot.ControlPlaneNamespace),
		client.MatchingLabels{v1beta1constants.LabelPodMaintenanceRestart: "true"},
	)
}

func waitUntilNoPodsExistAnymore(ctx context.Context, c client.Client, namespace string, deployments []string) error {
	fns := make([]flow.TaskFn, 0, len(deployments))
	for _, deploymentName := range deployments {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: namespace,
			},
		}
		fns = append(fns, func(ctx context.Context) error {
			if err := c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil {
				return err
			}

			podList := &metav1.PartialObjectMetadataList{}
			podList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("PodList"))
			timeoutContext, cancel := context.WithTimeout(ctx, time.Minute*5)
			defer cancel()
			return kubernetesutils.WaitUntilResourcesDeleted(timeoutContext, c, podList, time.Second*5, client.InNamespace(namespace), client.MatchingLabels(deployment.Spec.Selector.MatchLabels), client.Limit(1))
		})
	}
	return flow.Parallel(fns...)(ctx)
}
