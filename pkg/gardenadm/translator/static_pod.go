// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Translate translates the given object into a list of files containing static pod manifests as well as ConfigMaps and
// Secrets that can be injected into an OperatingSystemConfig.
func Translate(ctx context.Context, c client.Client, o client.Object) ([]extensionsv1alpha1.File, error) {
	switch obj := o.(type) {
	case *appsv1.Deployment:
		return translatePodTemplate(ctx, c, obj.ObjectMeta, obj.Spec.Template)
	// TODO(rfranzke): Consider adding support for StatefulSet in the future.
	default:
		return nil, fmt.Errorf("unsupported object type %T", o)
	}
}

func translatePodTemplate(ctx context.Context, c client.Client, objectMeta metav1.ObjectMeta, podTemplate corev1.PodTemplateSpec) ([]extensionsv1alpha1.File, error) {
	pod := &corev1.Pod{ObjectMeta: podTemplate.ObjectMeta, Spec: podTemplate.Spec}
	pod.Name = objectMeta.Name
	pod.Namespace = objectMeta.Namespace

	translateConfig(&pod.Spec)
	translateContainers(pod.Spec.Containers)

	filesFromVolumes, err := translateVolumes(ctx, c, pod)
	if err != nil {
		return nil, fmt.Errorf("failed translating volumes for static pod %s: %w", client.ObjectKeyFromObject(pod), err)
	}

	staticPodYAML, err := kubernetesutils.Serialize(pod, c.Scheme())
	if err != nil {
		return nil, fmt.Errorf("failed serializing static pod manifest for %s to YAML: %w", client.ObjectKeyFromObject(pod), err)
	}

	return append([]extensionsv1alpha1.File{{
		Path:        filepath.Join("/", "etc", "kubernetes", "manifests", pod.Name+".yaml"),
		Permissions: ptr.To[uint32](0644),
		Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Data: staticPodYAML}},
	}}, filesFromVolumes...), nil
}

func translateConfig(spec *corev1.PodSpec) {
	spec.HostNetwork = true
	spec.PriorityClassName = "system-node-critical"
}

func translateContainers(containers []corev1.Container) {
	for i, container := range containers {
		if container.LivenessProbe != nil && container.LivenessProbe.HTTPGet != nil {
			containers[i].LivenessProbe.HTTPGet.Host = "127.0.0.1"
		}
		if container.ReadinessProbe != nil && container.ReadinessProbe.HTTPGet != nil {
			containers[i].ReadinessProbe.HTTPGet.Host = "127.0.0.1"
		}
		if container.StartupProbe != nil && container.StartupProbe.HTTPGet != nil {
			containers[i].StartupProbe.HTTPGet.Host = "127.0.0.1"
		}
	}
}

func translateVolumes(ctx context.Context, c client.Client, pod *corev1.Pod) ([]extensionsv1alpha1.File, error) {
	var (
		files               []extensionsv1alpha1.File
		addFileWithHostPath = func(hostPath, fileName, content string, permissions *int32, desiredItems []corev1.KeyToPath) {
			path := filepath.Join(hostPath, fileName)
			if len(desiredItems) == 0 || slices.ContainsFunc(desiredItems, func(item corev1.KeyToPath) bool {
				path = filepath.Join(hostPath, item.Path)
				return fileName == item.Key
			}) {
				files = append(files, extensionsv1alpha1.File{
					Path:        path,
					Permissions: ptr.To[uint32](uint32(ptr.Deref(permissions, 0644))), // #nosec G115 -- permissions cannot be negative
					Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{
						// replace 'server' in generic token kubeconfig to point to 'localhost' for static pods
						Data: regexp.MustCompile(`(?m)^(\s*)server: .*$`).ReplaceAllString(content, "${1}server: https://127.0.0.1"),
					}},
				})
			}
		}
	)

	for i, volume := range pod.Spec.Volumes {
		hostPath := filepath.Join("/", "etc", "kubernetes", pod.Name, volume.Name)

		switch {
		case volume.ConfigMap != nil:
			configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: volume.ConfigMap.Name, Namespace: pod.Namespace}}
			if err := c.Get(ctx, client.ObjectKeyFromObject(configMap), configMap); err != nil {
				return nil, fmt.Errorf("failed reading ConfigMap %s of volume %s for static pod %s: %w", client.ObjectKeyFromObject(configMap), volume.Name, client.ObjectKeyFromObject(pod), err)
			}
			for k, v := range configMap.Data {
				addFileWithHostPath(hostPath, k, v, nil, nil)
			}
			pod.Spec.Volumes[i].VolumeSource = corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: hostPath}}

		case volume.Secret != nil:
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: volume.Secret.SecretName, Namespace: pod.Namespace}}
			if err := c.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
				return nil, fmt.Errorf("failed reading Secret %s of volume %s for static pod %s: %w", client.ObjectKeyFromObject(secret), volume.Name, client.ObjectKeyFromObject(pod), err)
			}
			for fileName, content := range secret.Data {
				addFileWithHostPath(hostPath, fileName, string(content), nil, nil)
			}
			pod.Spec.Volumes[i].VolumeSource = corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: hostPath}}

		case volume.Projected != nil:
			for _, source := range volume.Projected.Sources {
				switch {
				case source.ConfigMap != nil:
					configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: source.ConfigMap.Name, Namespace: pod.Namespace}}
					if err := c.Get(ctx, client.ObjectKeyFromObject(configMap), configMap); err != nil {
						return nil, fmt.Errorf("failed reading ConfigMap %s of volume %s for static pod %s: %w", client.ObjectKeyFromObject(configMap), volume.Name, client.ObjectKeyFromObject(pod), err)
					}
					for fileName, content := range configMap.Data {
						addFileWithHostPath(hostPath, fileName, content, volume.Projected.DefaultMode, source.ConfigMap.Items)
					}

				case source.Secret != nil:
					secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: source.Secret.Name, Namespace: pod.Namespace}}
					if err := c.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
						return nil, fmt.Errorf("failed reading Secret %s of volume %s for static pod %s: %w", client.ObjectKeyFromObject(secret), volume.Name, client.ObjectKeyFromObject(pod), err)
					}
					for fileName, content := range secret.Data {
						addFileWithHostPath(hostPath, fileName, string(content), volume.Projected.DefaultMode, source.Secret.Items)
					}
				}
			}

			pod.Spec.Volumes[i].VolumeSource = corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: hostPath}}
		}
	}

	return files, nil
}
