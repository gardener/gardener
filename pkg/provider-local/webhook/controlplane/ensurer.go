// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"path/filepath"
	"slices"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/extensions/pkg/webhook"
	extensionscontextwebhook "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/provider-local/imagevector"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

// NewEnsurer creates a new controlplane ensurer.
func NewEnsurer(logger logr.Logger) genericmutator.Ensurer {
	return &ensurer{
		logger: logger.WithName("local-controlplane-ensurer"),
	}
}

type ensurer struct {
	genericmutator.NoopEnsurer
	logger logr.Logger
}

// EnsureMachineControllerManagerDeployment ensures that the machine-controller-manager deployment conforms to the provider requirements.
func (e *ensurer) EnsureMachineControllerManagerDeployment(_ context.Context, _ extensionscontextwebhook.GardenContext, newObj, _ *appsv1.Deployment) error {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameMachineControllerManagerProviderLocal)
	if err != nil {
		return err
	}

	newObj.Spec.Template.Spec.Containers = webhook.EnsureContainerWithName(
		newObj.Spec.Template.Spec.Containers,
		machinecontrollermanager.ProviderSidecarContainer(newObj.Namespace, local.Name, image.String()),
	)
	return nil
}

// EnsureMachineControllerManagerVPA ensures that the machine-controller-manager VPA conforms to the provider requirements.
func (e *ensurer) EnsureMachineControllerManagerVPA(_ context.Context, _ extensionscontextwebhook.GardenContext, newObj, _ *vpaautoscalingv1.VerticalPodAutoscaler) error {
	var (
		minAllowed = corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		}
		maxAllowed = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("5G"),
		}
	)

	newObj.Spec.ResourcePolicy.ContainerPolicies = webhook.EnsureVPAContainerResourcePolicyWithName(
		newObj.Spec.ResourcePolicy.ContainerPolicies,
		machinecontrollermanager.ProviderSidecarVPAContainerPolicy(local.Name, minAllowed, maxAllowed),
	)
	return nil
}

func (e *ensurer) EnsureKubeletConfiguration(_ context.Context, _ extensionscontextwebhook.GardenContext, _ *semver.Version, newObj, _ *kubeletconfigv1beta1.KubeletConfiguration) error {
	newObj.FailSwapOn = ptr.To(false)
	newObj.CgroupDriver = "systemd"
	return nil
}

func (e *ensurer) EnsureAdditionalProvisionFiles(_ context.Context, _ extensionscontextwebhook.GardenContext, new, _ *[]extensionsv1alpha1.File) error {
	// The localhost upstream must be added via the provisioning OSC to pull the GNA dev image.
	// Further registries are supposed to be added via the `EnsureCRIConfig` function (reconcile OSC).
	localhostMirror := RegistryMirror{
		UpstreamHost:   "localhost:5001",
		UpstreamServer: "http://localhost:5001",
		MirrorHost:     "http://garden.local.gardener.cloud:5001",
	}

	*new = webhook.EnsureFileWithPath(*new, extensionsv1alpha1.File{
		Path:        filepath.Join("/etc/containerd/certs.d", localhostMirror.UpstreamHost, "hosts.toml"),
		Permissions: ptr.To[int32](0644),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Data: localhostMirror.HostsTOML(),
			},
		},
	})

	return nil
}

func (e *ensurer) EnsureCRIConfig(_ context.Context, _ extensionscontextwebhook.GardenContext, new, _ *extensionsv1alpha1.CRIConfig) error {
	if !extensionsv1alpha1helper.HasContainerdConfiguration(new) {
		return nil
	}

	for _, registry := range []extensionsv1alpha1.RegistryConfig{
		{
			Upstream: "gcr.io",
			Server:   ptr.To("https://gcr.io"),
			Hosts:    []extensionsv1alpha1.RegistryHost{{URL: "http://garden.local.gardener.cloud:5003"}},
		},
		{
			Upstream: "eu.gcr.io",
			Server:   ptr.To("https://eu.gcr.io"),
			Hosts:    []extensionsv1alpha1.RegistryHost{{URL: "http://garden.local.gardener.cloud:5004"}},
		},
		{
			Upstream: "ghcr.io",
			Server:   ptr.To("https://ghcr.io"),
			Hosts:    []extensionsv1alpha1.RegistryHost{{URL: "http://garden.local.gardener.cloud:5005"}},
		},
		{
			Upstream: "registry.k8s.io",
			Server:   ptr.To("https://registry.k8s.io"),
			Hosts:    []extensionsv1alpha1.RegistryHost{{URL: "http://garden.local.gardener.cloud:5006"}},
		},
		{
			Upstream: "quay.io",
			Server:   ptr.To("https://quay.io"),
			Hosts:    []extensionsv1alpha1.RegistryHost{{URL: "http://garden.local.gardener.cloud:5007"}},
		},
		{
			Upstream: "europe-docker.pkg.dev",
			Server:   ptr.To("https://europe-docker.pkg.dev"),
			Hosts:    []extensionsv1alpha1.RegistryHost{{URL: "http://garden.local.gardener.cloud:5008"}},
		},
	} {
		// Only add registry when it is not already set in the OSC.
		// This way, it is not added repeatably and extensions (e.g. registry cache) in the local setup may decide
		// to configure the same upstreams differently. The configuration of s	uch an extension should have precedence.
		addRegistryIfNotAvailable(registry, new.Containerd)
	}

	return nil
}

func addRegistryIfNotAvailable(registry extensionsv1alpha1.RegistryConfig, config *extensionsv1alpha1.ContainerdConfig) {
	if !slices.ContainsFunc(config.Registries, func(r extensionsv1alpha1.RegistryConfig) bool { return r.Upstream == registry.Upstream }) {
		config.Registries = append(config.Registries, registry)
	}
}
