// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"path/filepath"

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
	mirrors := []RegistryMirror{
		{UpstreamHost: "localhost:5001", UpstreamServer: "http://localhost:5001", MirrorHost: "http://garden.local.gardener.cloud:5001"},
		{UpstreamHost: "gcr.io", UpstreamServer: "https://gcr.io", MirrorHost: "http://garden.local.gardener.cloud:5003"},
		{UpstreamHost: "eu.gcr.io", UpstreamServer: "https://eu.gcr.io", MirrorHost: "http://garden.local.gardener.cloud:5004"},
		{UpstreamHost: "ghcr.io", UpstreamServer: "https://ghcr.io", MirrorHost: "http://garden.local.gardener.cloud:5005"},
		{UpstreamHost: "registry.k8s.io", UpstreamServer: "https://registry.k8s.io", MirrorHost: "http://garden.local.gardener.cloud:5006"},
		{UpstreamHost: "quay.io", UpstreamServer: "https://quay.io", MirrorHost: "http://garden.local.gardener.cloud:5007"},
		{UpstreamHost: "europe-docker.pkg.dev", UpstreamServer: "https://europe-docker.pkg.dev", MirrorHost: "http://garden.local.gardener.cloud:5008"},
	}

	for _, mirror := range mirrors {
		// appendFileIfNotPresent in used instead of appendUniqueFile intentionally to allow enabling and testing the registry-cache extension in local setup.
		// A file appended by the registry-cache extension is always picked up because:
		// - if a file is already appended by the registry-cache extension, provider-local won't overwrite it (appendFileIfNotPresent)
		// - if a file is already appended by provider-local, the registry-cache extension will overwrite it (appendUniqueFile)
		appendFileIfNotPresent(new, extensionsv1alpha1.File{
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

func appendFileIfNotPresent(files *[]extensionsv1alpha1.File, file extensionsv1alpha1.File) {
	if !containsFilePath(files, file.Path) {
		*files = append(*files, file)
	}
}

func containsFilePath(files *[]extensionsv1alpha1.File, filePath string) bool {
	for _, f := range *files {
		if f.Path == filePath {
			return true
		}
	}

	return false
}
