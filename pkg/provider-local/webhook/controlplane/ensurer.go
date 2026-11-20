// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/coreos/go-systemd/v22/unit"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/client-go/rest"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/extensions/pkg/webhook"
	extensionscontextwebhook "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	"github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/provider-local/imagevector"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

// NewEnsurer creates a new controlplane ensurer.
func NewEnsurer(client client.Client, restConfig *rest.Config, logger logr.Logger) genericmutator.Ensurer {
	return &ensurer{
		client:     client,
		restConfig: restConfig,
		logger:     logger.WithName("local-controlplane-ensurer"),
	}
}

type ensurer struct {
	genericmutator.NoopEnsurer

	client     client.Client
	restConfig *rest.Config
	logger     logr.Logger
}

// EnsureMachineControllerManagerDeployment ensures that the machine-controller-manager deployment conforms to the provider requirements.
func (e *ensurer) EnsureMachineControllerManagerDeployment(ctx context.Context, gctx extensionscontextwebhook.GardenContext, newObj, _ *appsv1.Deployment) error {
	cloudProviderSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.SecretNameCloudProvider,
			Namespace: newObj.Namespace,
		},
	}
	if err := e.client.Get(ctx, client.ObjectKeyFromObject(cloudProviderSecret), cloudProviderSecret); err != nil {
		return fmt.Errorf("failed getting cloudprovider secret: %w", err)
	}

	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameMachineControllerManagerProviderLocal)
	if err != nil {
		return err
	}

	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return fmt.Errorf("failed reading Cluster: %w", err)
	}

	sidecarContainer := machinecontrollermanager.ProviderSidecarContainer(cluster.Shoot, newObj.GetNamespace(), local.Name, image.String())

	if cloudProviderSecret.Labels[securityv1alpha1constants.LabelPurpose] == securityv1alpha1constants.LabelPurposeWorkloadIdentityTokenRequestor {
		const volumeName = "workload-identity"
		sidecarContainer.VolumeMounts = webhook.EnsureVolumeMountWithName(sidecarContainer.VolumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: "/var/run/secrets/gardener.cloud/workload-identity",
		})
		newObj.Spec.Template.Spec.Volumes = webhook.EnsureVolumeWithName(newObj.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: v1beta1constants.SecretNameCloudProvider,
								},
								Items: []corev1.KeyToPath{
									{
										Key:  securityv1alpha1constants.DataKeyToken,
										Path: "token",
									},
								},
							},
						},
					},
				},
			},
		})
	}

	newObj.Spec.Template.Spec.Containers = webhook.EnsureContainerWithName(
		newObj.Spec.Template.Spec.Containers,
		sidecarContainer,
	)

	// The provider sidecar talks to the infrastructure cluster hosting the machine pods, which is not necessarily the
	// runtime cluster of the seed (e.g., for the second kind cluster).
	metav1.SetMetaDataLabel(&newObj.Spec.Template.ObjectMeta, local.LabelNetworkPolicyToKindNetwork, v1beta1constants.LabelNetworkPolicyAllowed)
	return nil
}

// EnsureMachineControllerManagerVPA ensures that the machine-controller-manager VPA conforms to the provider requirements.
func (e *ensurer) EnsureMachineControllerManagerVPA(_ context.Context, _ extensionscontextwebhook.GardenContext, newObj, _ *vpaautoscalingv1.VerticalPodAutoscaler) error {
	newObj.Spec.ResourcePolicy.ContainerPolicies = webhook.EnsureVPAContainerResourcePolicyWithName(
		newObj.Spec.ResourcePolicy.ContainerPolicies,
		machinecontrollermanager.ProviderSidecarVPAContainerPolicy(local.Name),
	)
	return nil
}

// EnsureKubeletServiceUnitOptions ensures that the kubelet.service unit options conform to the provider requirements.
func (e *ensurer) EnsureKubeletServiceUnitOptions(ctx context.Context, gctx extensionscontextwebhook.GardenContext, _ *semver.Version, newObj, _ []*unit.UnitOption) ([]*unit.UnitOption, error) {
	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed reading Cluster: %w", err)
	}

	// Shoot nodes are only initialized by the node controller of cloud-controller-manager-local if the shoot has managed
	// infrastructure (i.e., nodes are machine pods in the runtime cluster). Only in this case, kubelet must be configured
	// with the external cloud provider so that nodes are registered with the "uninitialized" taint, which is removed by
	// the cloud-controller-manager once it has initialized the node (provider ID, topology labels, addresses).
	if !v1beta1helper.HasManagedInfrastructure(cluster.Shoot) {
		return newObj, nil
	}

	if opt := webhook.UnitOptionWithSectionAndName(newObj, "Service", "ExecStart"); opt != nil {
		command := webhook.DeserializeCommandLine(opt.Value)
		command = webhook.EnsureStringWithPrefix(command, "--cloud-provider=", "external")
		opt.Value = webhook.SerializeCommandLine(command, 0, " \\\n    ")
	}

	return newObj, nil
}

func (e *ensurer) EnsureKubeletConfiguration(_ context.Context, _ extensionscontextwebhook.GardenContext, _ *semver.Version, newObj, _ *kubeletconfigv1beta1.KubeletConfiguration) error {
	newObj.FailSwapOn = new(false)
	return nil
}

// EnsureKubeSchedulerDeployment ensures that the kube-scheduler deployment conforms to the provider requirements.
func (e *ensurer) EnsureKubeSchedulerDeployment(_ context.Context, _ extensionscontextwebhook.GardenContext, newObj, _ *appsv1.Deployment) error {
	newObj.Spec.Template.Labels["injected-by"] = "provider-local"
	return nil
}
