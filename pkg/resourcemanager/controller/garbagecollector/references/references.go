// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package references

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

const (
	// LabelKeyGarbageCollectable is a constant for a label key on a Secret or ConfigMap resource which makes
	// the GRM's garbage collector controller considering it for potential deletion in case it is unused by any
	// workload.
	LabelKeyGarbageCollectable = "resources.gardener.cloud/garbage-collectable-reference"
	// LabelValueGarbageCollectable is a constant for a label value on a Secret or ConfigMap resource which
	// makes the GRM's garbage collector controller considering it for potential deletion in case it is unused by any
	// workload.
	LabelValueGarbageCollectable = "true"

	delimiter = "-"
	// AnnotationKeyPrefix is a constant for the prefix used in annotations keys to indicate references to
	// other resources.
	AnnotationKeyPrefix = "reference.resources.gardener.cloud/"
	// KindConfigMap is a constant for the 'configmap' kind used in reference annotations.
	KindConfigMap = "configmap"
	// KindSecret is a constant for the 'secret' kind used in reference annotations.
	KindSecret = "secret"
)

// AnnotationKey computes a reference annotation key based on the given object kind and object name.
func AnnotationKey(kind, name string) string {
	var (
		h         = sha256.Sum256([]byte(name))
		sha256hex = hex.EncodeToString(h[:])
	)

	return AnnotationKeyPrefix + kind + delimiter + sha256hex[:8]
}

// KindFromAnnotationKey computes the object kind and object name based on the given reference annotation key. If
// the key is not valid then both return values will be empty.
func KindFromAnnotationKey(key string) string {
	if !strings.HasPrefix(key, AnnotationKeyPrefix) {
		return ""
	}

	var (
		withoutPrefix = strings.TrimPrefix(key, AnnotationKeyPrefix)
		split         = strings.Split(withoutPrefix, delimiter)
	)

	if len(split) != 2 {
		return ""
	}

	return split[0]
}

// InjectAnnotations injects annotations into the annotation maps based
// on the referenced ConfigMaps/Secrets appearing in:
//   - pod template spec's `.volumes[]` or `.containers[].envFrom[]` or `.containers[].env[].valueFrom[]` or `.imagePullSecrets[]` lists
//   - managed resource spec's `.secretRefs`
//
// Additional reference annotations can be specified via the variadic parameter (expected format is that returned by
// `AnnotationKey`).
func InjectAnnotations(obj runtime.Object, additional ...string) error {
	switch o := obj.(type) {
	case *corev1.Pod:
		referenceAnnotations := computeAnnotations(o.Spec, additional...)
		o.Annotations = mergeAnnotations(o.Annotations, referenceAnnotations)

	case *appsv1.Deployment:
		referenceAnnotations := computeAnnotations(o.Spec.Template.Spec, additional...)
		o.Annotations = mergeAnnotations(o.Annotations, referenceAnnotations)
		o.Spec.Template.Annotations = mergeAnnotations(o.Spec.Template.Annotations, referenceAnnotations)

	case *appsv1beta2.Deployment:
		referenceAnnotations := computeAnnotations(o.Spec.Template.Spec, additional...)
		o.Annotations = mergeAnnotations(o.Annotations, referenceAnnotations)
		o.Spec.Template.Annotations = mergeAnnotations(o.Spec.Template.Annotations, referenceAnnotations)

	case *appsv1beta1.Deployment:
		referenceAnnotations := computeAnnotations(o.Spec.Template.Spec, additional...)
		o.Annotations = mergeAnnotations(o.Annotations, referenceAnnotations)
		o.Spec.Template.Annotations = mergeAnnotations(o.Spec.Template.Annotations, referenceAnnotations)

	case *appsv1.StatefulSet:
		referenceAnnotations := computeAnnotations(o.Spec.Template.Spec, additional...)
		o.Annotations = mergeAnnotations(o.Annotations, referenceAnnotations)
		o.Spec.Template.Annotations = mergeAnnotations(o.Spec.Template.Annotations, referenceAnnotations)

	case *appsv1beta2.StatefulSet:
		referenceAnnotations := computeAnnotations(o.Spec.Template.Spec, additional...)
		o.Annotations = mergeAnnotations(o.Annotations, referenceAnnotations)
		o.Spec.Template.Annotations = mergeAnnotations(o.Spec.Template.Annotations, referenceAnnotations)

	case *appsv1beta1.StatefulSet:
		referenceAnnotations := computeAnnotations(o.Spec.Template.Spec, additional...)
		o.Annotations = mergeAnnotations(o.Annotations, referenceAnnotations)
		o.Spec.Template.Annotations = mergeAnnotations(o.Spec.Template.Annotations, referenceAnnotations)

	case *appsv1.DaemonSet:
		referenceAnnotations := computeAnnotations(o.Spec.Template.Spec, additional...)
		o.Annotations = mergeAnnotations(o.Annotations, referenceAnnotations)
		o.Spec.Template.Annotations = mergeAnnotations(o.Spec.Template.Annotations, referenceAnnotations)

	case *appsv1beta2.DaemonSet:
		referenceAnnotations := computeAnnotations(o.Spec.Template.Spec, additional...)
		o.Annotations = mergeAnnotations(o.Annotations, referenceAnnotations)
		o.Spec.Template.Annotations = mergeAnnotations(o.Spec.Template.Annotations, referenceAnnotations)

	case *batchv1.Job:
		referenceAnnotations := computeAnnotations(o.Spec.Template.Spec, additional...)
		o.Annotations = mergeAnnotations(o.Annotations, referenceAnnotations)
		o.Spec.Template.Annotations = mergeAnnotations(o.Spec.Template.Annotations, referenceAnnotations)

	case *batchv1.CronJob:
		referenceAnnotations := computeAnnotations(o.Spec.JobTemplate.Spec.Template.Spec, additional...)
		o.Annotations = mergeAnnotations(o.Annotations, referenceAnnotations)
		o.Spec.JobTemplate.Annotations = mergeAnnotations(o.Spec.JobTemplate.Annotations, referenceAnnotations)
		o.Spec.JobTemplate.Spec.Template.Annotations = mergeAnnotations(o.Spec.JobTemplate.Spec.Template.Annotations, referenceAnnotations)

	case *batchv1beta1.CronJob:
		referenceAnnotations := computeAnnotations(o.Spec.JobTemplate.Spec.Template.Spec, additional...)
		o.Annotations = mergeAnnotations(o.Annotations, referenceAnnotations)
		o.Spec.JobTemplate.Annotations = mergeAnnotations(o.Spec.JobTemplate.Annotations, referenceAnnotations)
		o.Spec.JobTemplate.Spec.Template.Annotations = mergeAnnotations(o.Spec.JobTemplate.Spec.Template.Annotations, referenceAnnotations)

	case *resourcesv1alpha1.ManagedResource:
		referenceAnnotations := computeAnnotationsFromLocalObjRefs(o.Spec.SecretRefs, KindSecret, additional...)
		o.Annotations = mergeAnnotations(o.Annotations, referenceAnnotations)

	case *monitoringv1.Prometheus:
		referenceAnnotations := utils.MergeStringMaps(
			computeAnnotationsFromContainers(o.Spec.Containers),
			computeAnnotationsFromVolumes(o.Spec.Volumes),
			computeAnnotationsFromLocalObjRefs(localObjectReferencesFromRemoteWrites(o.Spec.RemoteWrite), KindSecret, additional...),
		)
		o.Annotations = mergeAnnotations(o.Annotations, referenceAnnotations)

	default:
		return fmt.Errorf("unhandled object type %T", obj)
	}

	return nil
}

func computeAnnotationsFromLocalObjRefs(refs []corev1.LocalObjectReference, kind string, additional ...string) map[string]string {
	out := make(map[string]string, len(refs)+len(additional))
	for _, ref := range refs {
		out[AnnotationKey(kind, ref.Name)] = ref.Name
	}

	for _, v := range additional {
		out[v] = ""
	}

	return out
}

func computeAnnotations(spec corev1.PodSpec, additional ...string) map[string]string {
	out := make(map[string]string)

	out = utils.MergeStringMaps(out, computeAnnotationsFromContainers(spec.Containers))
	out = utils.MergeStringMaps(out, computeAnnotationsFromVolumes(spec.Volumes))
	out = utils.MergeStringMaps(out, computeAnnotationsFromLocalObjRefs(spec.ImagePullSecrets, KindSecret))

	for _, v := range additional {
		out[v] = ""
	}

	return out
}

func computeAnnotationsFromContainers(containers []corev1.Container) map[string]string {
	out := make(map[string]string)

	for _, container := range containers {
		for _, env := range container.EnvFrom {
			if env.SecretRef != nil {
				out[AnnotationKey(KindSecret, env.SecretRef.Name)] = env.SecretRef.Name
			}

			if env.ConfigMapRef != nil {
				out[AnnotationKey(KindConfigMap, env.ConfigMapRef.Name)] = env.ConfigMapRef.Name
			}
		}

		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
				out[AnnotationKey(KindSecret, env.ValueFrom.SecretKeyRef.Name)] = env.ValueFrom.SecretKeyRef.Name
			}

			if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil {
				out[AnnotationKey(KindConfigMap, env.ValueFrom.ConfigMapKeyRef.Name)] = env.ValueFrom.ConfigMapKeyRef.Name
			}
		}
	}

	return out
}

func computeAnnotationsFromVolumes(volumes []corev1.Volume) map[string]string {
	out := make(map[string]string)

	for _, volume := range volumes {
		if volume.Secret != nil {
			out[AnnotationKey(KindSecret, volume.Secret.SecretName)] = volume.Secret.SecretName
		}

		if volume.ConfigMap != nil {
			out[AnnotationKey(KindConfigMap, volume.ConfigMap.Name)] = volume.ConfigMap.Name
		}

		if volume.Projected != nil {
			for _, source := range volume.Projected.Sources {
				if source.Secret != nil {
					out[AnnotationKey(KindSecret, source.Secret.Name)] = source.Secret.Name
				}

				if source.ConfigMap != nil {
					out[AnnotationKey(KindConfigMap, source.ConfigMap.Name)] = source.ConfigMap.Name
				}
			}
		}
	}

	return out
}

func mergeAnnotations(oldAnnotations, newAnnotations map[string]string) map[string]string {
	// Remove all existing annotations with the AnnotationKeyPrefix to make sure that no longer referenced resources
	// do not remain in the annotations.
	old := make(map[string]string, len(oldAnnotations))
	for k, v := range oldAnnotations {
		if !strings.HasPrefix(k, AnnotationKeyPrefix) {
			old[k] = v
		}
	}

	return utils.MergeStringMaps(old, newAnnotations)
}

func localObjectReferencesFromRemoteWrites(remoteWrites []monitoringv1.RemoteWriteSpec) []corev1.LocalObjectReference {
	var references []corev1.LocalObjectReference
	for _, remoteWrite := range remoteWrites {
		if basicAuth := remoteWrite.BasicAuth; basicAuth != nil {
			references = append(references,
				basicAuth.Username.LocalObjectReference,
				basicAuth.Password.LocalObjectReference,
			)
		}
	}
	return references
}
