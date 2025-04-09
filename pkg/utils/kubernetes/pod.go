// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// VisitPodSpec calls the given visitor for the PodSpec contained in the given object. The visitor may mutate the
// PodSpec.
func VisitPodSpec(obj runtime.Object, visit func(*corev1.PodSpec)) error {
	switch o := obj.(type) {
	case *corev1.Pod:
		visit(&o.Spec)

	case *appsv1.Deployment:
		visit(&o.Spec.Template.Spec)

	case *appsv1beta2.Deployment:
		visit(&o.Spec.Template.Spec)

	case *appsv1beta1.Deployment:
		visit(&o.Spec.Template.Spec)

	case *appsv1.StatefulSet:
		visit(&o.Spec.Template.Spec)

	case *appsv1beta2.StatefulSet:
		visit(&o.Spec.Template.Spec)

	case *appsv1beta1.StatefulSet:
		visit(&o.Spec.Template.Spec)

	case *appsv1.DaemonSet:
		visit(&o.Spec.Template.Spec)

	case *appsv1beta2.DaemonSet:
		visit(&o.Spec.Template.Spec)

	case *batchv1.Job:
		visit(&o.Spec.Template.Spec)

	case *batchv1.CronJob:
		visit(&o.Spec.JobTemplate.Spec.Template.Spec)

	case *batchv1beta1.CronJob:
		visit(&o.Spec.JobTemplate.Spec.Template.Spec)

	default:
		return fmt.Errorf("unhandled object type %T", obj)
	}

	return nil
}

// VisitContainers calls the given visitor for all (init) containers in the given PodSpec. If containerNames are given
// it only visits (init) containers with matching names. The visitor may mutate the Container.
func VisitContainers(podSpec *corev1.PodSpec, visit func(*corev1.Container), containerNames ...string) {
	for i, c := range podSpec.InitContainers {
		container := c
		if len(containerNames) == 0 || slices.Contains(containerNames, container.Name) {
			visit(&container)
			podSpec.InitContainers[i] = container
		}
	}

	for i, c := range podSpec.Containers {
		container := c
		if len(containerNames) == 0 || slices.Contains(containerNames, container.Name) {
			visit(&container)
			podSpec.Containers[i] = container
		}
	}
}

// AddVolume adds the given Volume to the given PodSpec if not present. If a Volume with the given name is already
// present it optionally overwrites the Volume according to the overwrite parameter.
func AddVolume(podSpec *corev1.PodSpec, volume corev1.Volume, overwrite bool) {
	for i, v := range podSpec.Volumes {
		if v.Name == volume.Name {
			// volume with given name is already present
			if overwrite {
				podSpec.Volumes[i] = volume
			}
			return
		}
	}

	// volume with given name is not present, add it
	podSpec.Volumes = append(podSpec.Volumes, volume)
}

// AddVolumeMount adds the given VolumeMount to the given Container if not present. If a VolumeMount with the given name
// is already present it optionally overwrites the VolumeMount according to the overwrite parameter.
func AddVolumeMount(container *corev1.Container, volumeMount corev1.VolumeMount, overwrite bool) {
	for i, v := range container.VolumeMounts {
		if v.Name == volumeMount.Name {
			// volumeMount with given name is already present
			if overwrite {
				container.VolumeMounts[i] = volumeMount
			}
			return
		}
	}

	// volumeMount with given name is not present, add it
	container.VolumeMounts = append(container.VolumeMounts, volumeMount)
}

// AddEnvVar adds the given EnvVar to the given Container if not present. If a EnvVar with the given name
// is already present it optionally overwrites the EnvVar according to the overwrite parameter.
func AddEnvVar(container *corev1.Container, envVar corev1.EnvVar, overwrite bool) {
	for i, e := range container.Env {
		if e.Name == envVar.Name {
			// envVar with given name is already present
			if overwrite {
				container.Env[i] = envVar
			}
			return
		}
	}

	// envVar with given name is not present, add it
	container.Env = append(container.Env, envVar)
}

// HasEnvVar checks if the given container has an EnvVar with the given name.
func HasEnvVar(container corev1.Container, name string) bool {
	envVars := sets.New[string]()

	for _, e := range container.Env {
		envVars.Insert(e.Name)
	}

	return envVars.Has(name)
}

// GetDeploymentForPod returns the deployment the pod belongs to by traversing its metadata.
func GetDeploymentForPod(ctx context.Context, reader client.Reader, namespace string, podOwnerReferences []metav1.OwnerReference) (*appsv1.Deployment, error) {
	var replicaSetName string
	for _, ownerReference := range podOwnerReferences {
		if ownerReference.APIVersion == appsv1.SchemeGroupVersion.String() && ownerReference.Kind == "ReplicaSet" {
			replicaSetName = ownerReference.Name
		}
	}

	if replicaSetName == "" {
		return nil, nil
	}

	replicaSet := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: replicaSetName, Namespace: namespace}}
	replicaSet.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("ReplicaSet"))
	if err := reader.Get(ctx, client.ObjectKeyFromObject(replicaSet), replicaSet); err != nil {
		return nil, fmt.Errorf("failed reading ReplicaSet %s: %w", client.ObjectKeyFromObject(replicaSet), err)
	}

	var deploymentName string
	for _, ownerReference := range replicaSet.OwnerReferences {
		if ownerReference.APIVersion == appsv1.SchemeGroupVersion.String() && ownerReference.Kind == "Deployment" {
			deploymentName = ownerReference.Name
		}
	}

	if deploymentName == "" {
		return nil, nil
	}

	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: replicaSet.Namespace}}
	if err := reader.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil {
		return nil, fmt.Errorf("failed reading Deployment %s: %w", client.ObjectKeyFromObject(deployment), err)
	}

	return deployment, nil
}

// DeleteStalePods deletes stale pods.
func DeleteStalePods(ctx context.Context, log logr.Logger, c client.Client, pods []corev1.Pod) error {
	var result error

	for _, pod := range pods {
		logger := log.WithValues("pod", client.ObjectKeyFromObject(&pod))

		if health.IsPodStale(pod.Status.Reason) {
			logger.V(1).Info("Deleting stale pod", "reason", pod.Status.Reason)
			if err := c.Delete(ctx, &pod); client.IgnoreNotFound(err) != nil {
				result = multierror.Append(result, err)
			}

			continue
		}

		if shouldObjectBeRemoved(&pod) {
			logger.V(1).Info("Deleting stuck terminating pod")
			forceDeleteOptions := []client.DeleteOption{
				client.PropagationPolicy(metav1.DeletePropagationBackground),
				client.GracePeriodSeconds(0),
			}
			if err := c.Delete(ctx, &pod, forceDeleteOptions...); client.IgnoreNotFound(err) != nil {
				result = multierror.Append(result, err)
			}
		}
	}

	return result
}

// shouldObjectBeRemoved determines whether the given object should be gone now.
// This is calculated by first checking the deletion timestamp of an object: If the deletion timestamp
// is unset, the object should not be removed - i.e. this returns false.
// Otherwise, it is checked whether the deletionTimestamp is before the current time minus the
// grace period.
func shouldObjectBeRemoved(obj metav1.Object) bool {
	// gardenerDeletionGracePeriod is the default grace period for Gardener's force deletion methods.
	const gardenerDeletionGracePeriod = 5 * time.Minute

	deletionTimestamp := obj.GetDeletionTimestamp()
	if deletionTimestamp == nil {
		return false
	}

	return deletionTimestamp.Time.Before(time.Now().Add(-gardenerDeletionGracePeriod))
}

// GetPodLogs retrieves the pod logs of the pod of the given name with the given options.
func GetPodLogs(ctx context.Context, podInterface corev1client.PodInterface, name string, options *corev1.PodLogOptions) ([]byte, error) {
	request := podInterface.GetLogs(name, options)

	stream, err := request.Stream(ctx)
	if err != nil {
		return nil, err
	}

	defer func() { utilruntime.HandleError(stream.Close()) }()

	return io.ReadAll(stream)
}

// InjectKubernetesServiceHostEnv injects the KUBERNETES_SERVICE_HOST env var into all containers if not already
// defined.
func InjectKubernetesServiceHostEnv(containers []corev1.Container, host string) {
	for i, container := range containers {
		if slices.ContainsFunc(container.Env, func(envVar corev1.EnvVar) bool { return envVar.Name == "KUBERNETES_SERVICE_HOST" }) {
			continue
		}

		if container.Env == nil {
			container.Env = make([]corev1.EnvVar, 0, 1)
		}

		containers[i].Env = append(containers[i].Env, corev1.EnvVar{
			Name:      "KUBERNETES_SERVICE_HOST",
			Value:     host,
			ValueFrom: nil,
		})
	}
}
