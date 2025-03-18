// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package staticpod

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Translate translates the given object into a list of files containing static pod manifests as well as ConfigMaps and
// Secrets that can be injected into an OperatingSystemConfig.
func Translate(ctx context.Context, c client.Client, o client.Object) ([]extensionsv1alpha1.File, error) {
	switch obj := o.(type) {
	case *appsv1.Deployment:
		return translatePodTemplate(ctx, c, obj.ObjectMeta, obj.Spec.Template)
	case *corev1.Pod:
		return translatePodTemplate(ctx, c, obj.ObjectMeta, corev1.PodTemplateSpec{ObjectMeta: obj.ObjectMeta, Spec: obj.Spec})
	// TODO(rfranzke): Consider adding support for StatefulSet in the future.
	default:
		return nil, fmt.Errorf("unsupported object type %T", o)
	}
}

func translatePodTemplate(ctx context.Context, c client.Client, objectMeta metav1.ObjectMeta, podTemplate corev1.PodTemplateSpec) ([]extensionsv1alpha1.File, error) {
	pod := &corev1.Pod{ObjectMeta: podTemplate.ObjectMeta, Spec: podTemplate.Spec}
	pod.Name = objectMeta.Name
	pod.Namespace = metav1.NamespaceSystem

	translateSpec(&pod.Spec)

	filesFromVolumes, err := translateVolumes(ctx, c, pod, objectMeta.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed translating volumes for static pod %s: %w", client.ObjectKeyFromObject(pod), err)
	}

	staticPodYAML, err := kubernetesutils.Serialize(pod, c.Scheme())
	if err != nil {
		return nil, fmt.Errorf("failed serializing static pod manifest for %s to YAML: %w", client.ObjectKeyFromObject(pod), err)
	}

	return append([]extensionsv1alpha1.File{{
		Path:        filepath.Join(kubelet.FilePathKubernetesManifests, pod.Name+".yaml"),
		Permissions: ptr.To[uint32](0600),
		Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(staticPodYAML))}},
	}}, filesFromVolumes...), nil
}

func translateSpec(spec *corev1.PodSpec) {
	spec.HostNetwork = true
	spec.PriorityClassName = "system-node-critical"
}

func translateVolumes(ctx context.Context, c client.Client, pod *corev1.Pod, sourceNamespace string) ([]extensionsv1alpha1.File, error) {
	var (
		files               []extensionsv1alpha1.File
		addFileWithHostPath = func(hostPath, fileName string, content []byte, desiredItems []corev1.KeyToPath) {
			file := extensionsv1alpha1.File{
				Permissions: ptr.To[uint32](0600),
				Content:     extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64(content)}},
			}

			if len(desiredItems) == 0 {
				file.Path = filepath.Join(hostPath, fileName)
				files = append(files, file)
			}

			if idx := slices.IndexFunc(desiredItems, func(item corev1.KeyToPath) bool {
				return fileName == item.Key
			}); idx != -1 {
				file.Path = filepath.Join(hostPath, desiredItems[idx].Path)
				files = append(files, file)
			}
		}
	)

	for i, volume := range pod.Spec.Volumes {
		hostPath := filepath.Join(kubelet.FilePathKubernetesManifests, pod.Name, volume.Name)

		switch {
		case volume.ConfigMap != nil:
			configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: volume.ConfigMap.Name, Namespace: sourceNamespace}}
			if err := c.Get(ctx, client.ObjectKeyFromObject(configMap), configMap); err != nil {
				return nil, fmt.Errorf("failed reading ConfigMap %s of volume %s for static pod %s: %w", client.ObjectKeyFromObject(configMap), volume.Name, client.ObjectKeyFromObject(pod), err)
			}
			for fileName, content := range configMap.Data {
				addFileWithHostPath(hostPath, fileName, []byte(content), volume.ConfigMap.Items)
			}
			pod.Spec.Volumes[i].VolumeSource = corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: hostPath}}

		case volume.Secret != nil:
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: volume.Secret.SecretName, Namespace: sourceNamespace}}
			if err := c.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
				return nil, fmt.Errorf("failed reading Secret %s of volume %s for static pod %s: %w", client.ObjectKeyFromObject(secret), volume.Name, client.ObjectKeyFromObject(pod), err)
			}
			for fileName, content := range secret.Data {
				addFileWithHostPath(hostPath, fileName, content, volume.Secret.Items)
			}
			pod.Spec.Volumes[i].VolumeSource = corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: hostPath}}

		case volume.Projected != nil:
			for _, source := range volume.Projected.Sources {
				switch {
				case source.ConfigMap != nil:
					configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: source.ConfigMap.Name, Namespace: sourceNamespace}}
					if err := c.Get(ctx, client.ObjectKeyFromObject(configMap), configMap); err != nil {
						return nil, fmt.Errorf("failed reading ConfigMap %s of volume %s for static pod %s: %w", client.ObjectKeyFromObject(configMap), volume.Name, client.ObjectKeyFromObject(pod), err)
					}
					for fileName, content := range configMap.Data {
						addFileWithHostPath(hostPath, fileName, []byte(content), source.ConfigMap.Items)
					}

				case source.Secret != nil:
					secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: source.Secret.Name, Namespace: sourceNamespace}}
					if err := c.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
						return nil, fmt.Errorf("failed reading Secret %s of volume %s for static pod %s: %w", client.ObjectKeyFromObject(secret), volume.Name, client.ObjectKeyFromObject(pod), err)
					}
					for fileName, content := range secret.Data {
						addFileWithHostPath(hostPath, fileName, content, source.Secret.Items)
					}
				}
			}

			pod.Spec.Volumes[i].VolumeSource = corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: hostPath}}
		}
	}

	return files, nil
}
