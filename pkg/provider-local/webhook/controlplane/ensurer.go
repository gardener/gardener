// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controlplane

import (
	"context"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/extensions/pkg/webhook"
	extensionscontextwebhook "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/provider-local/imagevector"
	"github.com/gardener/gardener/pkg/provider-local/local"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// NewEnsurer creates a new controlplane ensurer.
func NewEnsurer(logger logr.Logger, gardenletManagesMCM bool) genericmutator.Ensurer {
	return &ensurer{
		logger:              logger.WithName("local-controlplane-ensurer"),
		gardenletManagesMCM: gardenletManagesMCM,
	}
}

type ensurer struct {
	genericmutator.NoopEnsurer
	logger              logr.Logger
	gardenletManagesMCM bool
}

// EnsureMachineControllerManagerDeployment ensures that the machine-controller-manager deployment conforms to the provider requirements.
func (e *ensurer) EnsureMachineControllerManagerDeployment(ctx context.Context, gctx extensionscontextwebhook.GardenContext, newObj, _ *appsv1.Deployment) error {
	if !e.gardenletManagesMCM {
		return nil
	}

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
	if !e.gardenletManagesMCM {
		return nil
	}

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

func (e *ensurer) EnsureKubeAPIServerDeployment(_ context.Context, _ extensionscontextwebhook.GardenContext, new, _ *appsv1.Deployment) error {
	metav1.SetMetaDataLabel(&new.Spec.Template.ObjectMeta, gardenerutils.NetworkPolicyLabel("machines", 10250), v1beta1constants.LabelNetworkPolicyAllowed)
	return nil
}

func (e *ensurer) EnsureKubeletConfiguration(_ context.Context, _ extensionscontextwebhook.GardenContext, _ *semver.Version, newObj, _ *kubeletconfigv1beta1.KubeletConfiguration) error {
	newObj.FailSwapOn = pointer.Bool(false)
	newObj.CgroupDriver = "systemd"
	return nil
}

func (e *ensurer) EnsureAdditionalFiles(ctx context.Context, gc extensionscontextwebhook.GardenContext, new, _ *[]extensionsv1alpha1.File) error {
	mirrors := []RegistryMirror{
		{UpstreamHost: "localhost:5001", UpstreamServer: "http://localhost:5001", MirrorHost: "http://garden.local.gardener.cloud:5001"},
		{UpstreamHost: "gcr.io", UpstreamServer: "https://gcr.io", MirrorHost: "http://garden.local.gardener.cloud:5003"},
		{UpstreamHost: "eu.gcr.io", UpstreamServer: "https://eu.gcr.io", MirrorHost: "http://garden.local.gardener.cloud:5004"},
		{UpstreamHost: "ghcr.io", UpstreamServer: "https://ghcr.io", MirrorHost: "http://garden.local.gardener.cloud:5005"},
		{UpstreamHost: "registry.k8s.io", UpstreamServer: "https://registry.k8s.io", MirrorHost: "http://garden.local.gardener.cloud:5006"},
		{UpstreamHost: "quay.io", UpstreamServer: "https://quay.io", MirrorHost: "http://garden.local.gardener.cloud:5007"},
	}

	for _, mirror := range mirrors {
		// appendFileIfNotPresent in used instead of appendUniqueFile intentionally to allow enabling and testing the registry-cache extension in local setup.
		// A file appended by the registry-cache extension is always picked up because:
		// - if a file is already appended by the registry-cache extension, provider-local won't overwrite it (appendFileIfNotPresent)
		// - if a file is already appended by provider-local, the registry-cache extension will overwrite it (appendUniqueFile)
		appendFileIfNotPresent(new, extensionsv1alpha1.File{
			Path:        filepath.Join("/etc/containerd/certs.d", mirror.UpstreamHost, "hosts.toml"),
			Permissions: pointer.Int32(0644),
			Content: extensionsv1alpha1.FileContent{
				Inline: &extensionsv1alpha1.FileContentInline{
					Data: mirror.HostsTOML(),
				},
			},
		})
	}

	return nil
}

// TODO(ialidzhikov): Drop the containerd-configuration-local-setup.service unit in 1.81.
// It is preserved only for graceful migration purposes. Currently it is a no-op unit that only sleeps 1s.

const unitNameInitializer = "containerd-configuration-local-setup.service"

func (e *ensurer) EnsureAdditionalUnits(_ context.Context, _ extensionscontextwebhook.GardenContext, new, _ *[]extensionsv1alpha1.Unit) error {
	unit := extensionsv1alpha1.Unit{
		Name:    unitNameInitializer,
		Command: pointer.String("start"),
		Enable:  pointer.Bool(true),
		Content: pointer.String(`[Unit]
Description=Containerd config configuration for local-setup

[Install]
WantedBy=multi-user.target

[Unit]
After=containerd-initializer.service
Requires=containerd-initializer.service

[Service]
Type=oneshot
RemainAfterExit=no
ExecStart=sleep 1s`)}

	appendUniqueUnit(new, unit)

	return nil
}

func appendFileIfNotPresent(files *[]extensionsv1alpha1.File, file extensionsv1alpha1.File) {
	if !containsFilePath(files, file.Path) {
		*files = append(*files, file)
	}
}

// appendUniqueUnit appends a unit only if it does not exist, otherwise overwrite content of previous unit
func appendUniqueUnit(units *[]extensionsv1alpha1.Unit, unit extensionsv1alpha1.Unit) {
	resFiles := make([]extensionsv1alpha1.Unit, 0, len(*units))

	for _, f := range *units {
		if f.Name != unit.Name {
			resFiles = append(resFiles, f)
		}
	}

	*units = append(resFiles, unit)
}

func containsFilePath(files *[]extensionsv1alpha1.File, filePath string) bool {
	for _, f := range *files {
		if f.Path == filePath {
			return true
		}
	}

	return false
}
