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
	newObj.Spec.ResourcePolicy.ContainerPolicies = webhook.EnsureVPAContainerResourcePolicyWithName(
		newObj.Spec.ResourcePolicy.ContainerPolicies,
		machinecontrollermanager.ProviderSidecarVPAContainerPolicy(local.Name),
	)
	return nil
}

func (e *ensurer) EnsureKubeletConfiguration(_ context.Context, _ extensionscontextwebhook.GardenContext, _ *semver.Version, newObj, _ *kubeletconfigv1beta1.KubeletConfiguration) error {
	newObj.FailSwapOn = ptr.To(false)
	newObj.CgroupDriver = "systemd"
	return nil
}

func (e *ensurer) EnsureAdditionalProvisionFiles(_ context.Context, _ extensionscontextwebhook.GardenContext, new, _ *[]extensionsv1alpha1.File) error {
	for _, mirror := range []RegistryMirror{
		// localhost upstream is required for loading the GNA image.
		{
			UpstreamHost: "localhost:5001",
			MirrorHost:   "http://garden.local.gardener.cloud:5001",
		},
		// europe-docker.pkg.dev upstream is required for loading the Hyperkube image.
		// Further registries are supposed to be added via the `EnsureCRIConfig` function (reconcile OSC).
		{
			UpstreamHost: "europe-docker.pkg.dev",
			MirrorHost:   "http://garden.local.gardener.cloud:5008",
		},
	} {
		*new = webhook.EnsureFileWithPath(*new, extensionsv1alpha1.File{
			Path:        filepath.Join("/etc/containerd/certs.d", mirror.UpstreamHost, "hosts.toml"),
			Permissions: ptr.To[int32](0644),
			Content: extensionsv1alpha1.FileContent{
				Inline: &extensionsv1alpha1.FileContentInline{
					Data: mirror.HostsTOML(),
				},
			},
		})
	}

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
			Upstream: "registry.k8s.io",
			Server:   ptr.To("https://registry.k8s.io"),
			Hosts:    []extensionsv1alpha1.RegistryHost{{URL: "http://garden.local.gardener.cloud:5006"}},
		},
		{
			Upstream: "quay.io",
			Server:   ptr.To("https://quay.io"),
			Hosts:    []extensionsv1alpha1.RegistryHost{{URL: "http://garden.local.gardener.cloud:5007"}},
		},
		// Enable containerd to reach registry at garden.local.gardener.cloud:5001 via HTTP.
		{
			Upstream: "garden.local.gardener.cloud:5001",
			Server:   ptr.To("http://garden.local.gardener.cloud:5001"),
			Hosts:    []extensionsv1alpha1.RegistryHost{{URL: "http://garden.local.gardener.cloud:5001"}},
		},
	} {
		// Only add registry when it is not already set in the OSC.
		// This way, it is not added repeatably and extensions (e.g. registry cache) in the local setup may decide
		// to configure the same upstreams differently. The configuration of such an extension should have precedence.
		addRegistryIfNotAvailable(registry, new.Containerd)
	}

	return nil
}

func addRegistryIfNotAvailable(registry extensionsv1alpha1.RegistryConfig, config *extensionsv1alpha1.ContainerdConfig) {
	if !slices.ContainsFunc(config.Registries, func(r extensionsv1alpha1.RegistryConfig) bool { return r.Upstream == registry.Upstream }) {
		config.Registries = append(config.Registries, registry)
	}
}
