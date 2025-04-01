// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresource

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
)

const (
	descriptionAnnotation     = "resources.gardener.cloud/description"
	descriptionAnnotationText = `DO NOT EDIT - This resource is managed by gardener-resource-manager.
Any modifications are discarded and the resource is returned to the original state.`
)

// merge merges the values of the `desired` object into the `current` object while preserving `current`'s important
// metadata (like resourceVersion and finalizers), status and selected spec fields of the respective kind (e.g.
// .spec.selector of a Job).
func merge(origin string, desired, current *unstructured.Unstructured, forceOverwriteLabels bool, existingLabels map[string]string, forceOverwriteAnnotations bool, existingAnnotations map[string]string, preserveReplicas bool) error {
	// save copy of current object before merging
	oldObject := current.DeepCopy()

	// copy desired state into new object
	newObject := current
	desired.DeepCopyInto(newObject)

	// keep metadata information of old object to avoid unnecessary update calls
	if oldMetadataInterface, ok := oldObject.Object["metadata"]; ok {
		// cast to map to be able to check if metadata is empty
		if oldMetadataMap, ok := oldMetadataInterface.(map[string]any); ok {
			if len(oldMetadataMap) > 0 {
				newObject.Object["metadata"] = oldMetadataMap
			}
		}
	}

	if forceOverwriteLabels {
		newObject.SetLabels(desired.GetLabels())
	} else {
		newObject.SetLabels(mergeMapsBasedOnOldMap(desired.GetLabels(), oldObject.GetLabels(), existingLabels))
	}

	var ann map[string]string

	if forceOverwriteAnnotations {
		ann = desired.GetAnnotations()
	} else {
		// Here, we drop the 'reference' annotations from `oldObject` which are used by the garbage collector controller.
		// Typically, all annotations which were previously added to the desired state of a resource are preserved in the
		// `status` of the respective `ManagedResource`. This way, in subsequent reconciliations the controller can know
		// whether found annotations were earlier managed by us and have to be dropped or kept.
		// However, when an object has existing 'reference' annotations which were are not found in the `status` of the
		// `ManagedResource` (this can happen when resources are migrated from one `ManagedResource` to another) then
		// they would be kept.
		// Since the correct 'reference' annotations must be part of the `desired` object anyways, there is anyways no
		// point in potentially keeping old 'reference' annotations from `oldObject`, so we can always drop them here.
		ann = mergeMapsBasedOnOldMap(desired.GetAnnotations(), dropReferenceAnnotations(oldObject.GetAnnotations()), existingAnnotations)
	}

	if ann == nil {
		ann = map[string]string{}
	}

	ann[descriptionAnnotation] = descriptionAnnotationText
	ann[resourcesv1alpha1.OriginAnnotation] = origin
	newObject.SetAnnotations(ann)

	// keep status of old object if it is set and not empty
	var oldStatus map[string]any
	if oldStatusInterface, containsStatus := oldObject.Object["status"]; containsStatus {
		// cast to map to be able to check if status is empty
		if oldStatusMap, ok := oldStatusInterface.(map[string]any); ok {
			oldStatus = oldStatusMap
		}
	}

	if len(oldStatus) > 0 {
		newObject.Object["status"] = oldStatus
	} else {
		delete(newObject.Object, "status")
	}

	annotations := desired.GetAnnotations()
	if annotations[resourcesv1alpha1.PreserveReplicas] == "true" {
		preserveReplicas = true
	}
	preserveResources := annotations[resourcesv1alpha1.PreserveResources] == "true"

	switch newObject.GroupVersionKind().GroupKind() {
	case appsv1.SchemeGroupVersion.WithKind("Deployment").GroupKind(), extensionsv1beta1.SchemeGroupVersion.WithKind("Deployment").GroupKind():
		return mergeDeployment(scheme.Scheme, oldObject, newObject, preserveReplicas, preserveResources)
	case batchv1.SchemeGroupVersion.WithKind("Job").GroupKind():
		return mergeJob(scheme.Scheme, oldObject, newObject, preserveResources)
	case batchv1.SchemeGroupVersion.WithKind("CronJob").GroupKind():
		return mergeCronJob(scheme.Scheme, oldObject, newObject, preserveResources)
	case appsv1.SchemeGroupVersion.WithKind("StatefulSet").GroupKind(), extensionsv1beta1.SchemeGroupVersion.WithKind("StatefulSet").GroupKind():
		return mergeStatefulSet(scheme.Scheme, oldObject, newObject, preserveReplicas, preserveResources)
	case appsv1.SchemeGroupVersion.WithKind("DaemonSet").GroupKind():
		return mergeDaemonSet(scheme.Scheme, oldObject, newObject, preserveResources)
	case corev1.SchemeGroupVersion.WithKind("Service").GroupKind():
		return mergeService(scheme.Scheme, oldObject, newObject)
	case corev1.SchemeGroupVersion.WithKind("ServiceAccount").GroupKind():
		return mergeServiceAccount(scheme.Scheme, oldObject, newObject)
	}

	return nil
}

func mergeDeployment(scheme *runtime.Scheme, oldObj, newObj runtime.Object, preserveReplicas, preserveResources bool) error {
	oldDeployment := &appsv1.Deployment{}
	if err := scheme.Convert(oldObj, oldDeployment, nil); err != nil {
		return err
	}

	newDeployment := &appsv1.Deployment{}
	if err := scheme.Convert(newObj, newDeployment, nil); err != nil {
		return err
	}

	// Do not overwrite a Deployment's '.spec.replicas' if the new Deployment's '.spec.replicas'
	// field is unset or we are asked to preserve the replicas (e.g. the Deployment is scaled by HPA).
	if newDeployment.Spec.Replicas == nil || preserveReplicas {
		newDeployment.Spec.Replicas = oldDeployment.Spec.Replicas
	}

	mergePodTemplate(&oldDeployment.Spec.Template, &newDeployment.Spec.Template, preserveResources)

	return scheme.Convert(newDeployment, newObj, nil)
}

const restartedAtAnnotation = "kubectl.kubernetes.io/restartedAt"

func mergePodTemplate(oldPod, newPod *corev1.PodTemplateSpec, preserveResources bool) {
	// Do not overwrite the "kubectl.kubernetes.io/restartedAt" annotation as it is used by
	// the "kubectl rollout restart <RESOURCE>" command to trigger rollouts.
	// Otherwise, the resource-manager would revert this annotation set by the kubectl command and
	// this would revert the triggered rollout.
	// Ref https://kubernetes.io/docs/reference/labels-annotations-taints/#kubectl-k8s-io-restart-at
	if metav1.HasAnnotation(oldPod.ObjectMeta, restartedAtAnnotation) {
		metav1.SetMetaDataAnnotation(&newPod.ObjectMeta, restartedAtAnnotation, oldPod.Annotations[restartedAtAnnotation])
	}

	if preserveResources {
		// Do not overwrite a PodTemplate's resource requests / limits when we are asked to preserve the resources
		for i, newContainer := range newPod.Spec.Containers {
			for j, oldContainer := range oldPod.Spec.Containers {
				if newContainer.Name == oldContainer.Name {
					mergeContainer(&oldPod.Spec.Containers[j], &newPod.Spec.Containers[i], preserveResources)
					break
				}
			}
		}
	}
}

func mergeContainer(oldContainer, newContainer *corev1.Container, preserveResources bool) {
	if !preserveResources {
		return
	}

	for resourceName, oldRequests := range oldContainer.Resources.Requests {
		switch resourceName {
		case corev1.ResourceCPU, corev1.ResourceMemory:
			if newContainer.Resources.Requests == nil {
				newContainer.Resources.Requests = corev1.ResourceList{}
			}

			newContainer.Resources.Requests[resourceName] = oldRequests
		}
	}

	for resourceName, oldLimits := range oldContainer.Resources.Limits {
		switch resourceName {
		case corev1.ResourceCPU, corev1.ResourceMemory:
			if newContainer.Resources.Limits == nil {
				newContainer.Resources.Limits = corev1.ResourceList{}
			}

			newContainer.Resources.Limits[resourceName] = oldLimits
		}
	}
}

func mergeJob(scheme *runtime.Scheme, oldObj, newObj runtime.Object, preserveResources bool) error {
	oldJob := &batchv1.Job{}
	if err := scheme.Convert(oldObj, oldJob, nil); err != nil {
		return err
	}

	newJob := &batchv1.Job{}
	if err := scheme.Convert(newObj, newJob, nil); err != nil {
		return err
	}

	// Do not overwrite a Job's '.spec.selector' since it is immutable.
	newJob.Spec.Selector = oldJob.Spec.Selector

	// Do not overwrite Job managed labels as 'controller-uid' and 'job-name'. '.spec.template' is immutable.
	newJob.Spec.Template.Labels = labels.Merge(oldJob.Spec.Template.Labels, newJob.Spec.Template.Labels)

	mergePodTemplate(&oldJob.Spec.Template, &newJob.Spec.Template, preserveResources)

	return scheme.Convert(newJob, newObj, nil)
}

func mergeCronJob(scheme *runtime.Scheme, oldObj, newObj runtime.Object, preserveResources bool) error {
	switch newObj.GetObjectKind().GroupVersionKind().Version {
	case batchv1beta1.SchemeGroupVersion.Version:
		return mergeV1beta1CronJob(scheme, oldObj, newObj, preserveResources)
	case batchv1.SchemeGroupVersion.Version:
		return mergeV1CronJob(scheme, oldObj, newObj, preserveResources)
	default:
		return fmt.Errorf("cannot merge objects with gvk: %s", newObj.GetObjectKind().GroupVersionKind().String())
	}
}

func mergeV1beta1CronJob(scheme *runtime.Scheme, oldObj, newObj runtime.Object, preserveResources bool) error {
	oldCronJob := &batchv1beta1.CronJob{}
	if err := scheme.Convert(oldObj, oldCronJob, nil); err != nil {
		return err
	}

	newCronJob := &batchv1beta1.CronJob{}
	if err := scheme.Convert(newObj, newCronJob, nil); err != nil {
		return err
	}

	mergePodTemplate(&oldCronJob.Spec.JobTemplate.Spec.Template, &newCronJob.Spec.JobTemplate.Spec.Template, preserveResources)

	return scheme.Convert(newCronJob, newObj, nil)
}

func mergeV1CronJob(scheme *runtime.Scheme, oldObj, newObj runtime.Object, preserveResources bool) error {
	oldCronJob := &batchv1.CronJob{}
	if err := scheme.Convert(oldObj, oldCronJob, nil); err != nil {
		return err
	}

	newCronJob := &batchv1.CronJob{}
	if err := scheme.Convert(newObj, newCronJob, nil); err != nil {
		return err
	}

	mergePodTemplate(&oldCronJob.Spec.JobTemplate.Spec.Template, &newCronJob.Spec.JobTemplate.Spec.Template, preserveResources)

	return scheme.Convert(newCronJob, newObj, nil)
}

func mergeStatefulSet(scheme *runtime.Scheme, oldObj, newObj runtime.Object, preserveReplicas, preserveResources bool) error {
	oldStatefulSet := &appsv1.StatefulSet{}
	if err := scheme.Convert(oldObj, oldStatefulSet, nil); err != nil {
		return err
	}

	newStatefulSet := &appsv1.StatefulSet{}
	if err := scheme.Convert(newObj, newStatefulSet, nil); err != nil {
		return err
	}

	// Do not overwrite a StatefulSet's '.spec.replicas' if the new StatefulSet's `.spec.replicas'
	// field is unset or we are asked to preserve the replicas (e.g. the StatefulSet is scaled by HPA).
	if newStatefulSet.Spec.Replicas == nil || preserveReplicas {
		newStatefulSet.Spec.Replicas = oldStatefulSet.Spec.Replicas
	}

	// Do not overwrite a StatefulSet's '.spec.volumeClaimTemplates' field once the StatefulSet has been created as it is immutable
	if !oldStatefulSet.CreationTimestamp.IsZero() {
		newStatefulSet.Spec.VolumeClaimTemplates = oldStatefulSet.Spec.VolumeClaimTemplates
	}

	mergePodTemplate(&oldStatefulSet.Spec.Template, &newStatefulSet.Spec.Template, preserveResources)

	return scheme.Convert(newStatefulSet, newObj, nil)
}

func mergeDaemonSet(scheme *runtime.Scheme, oldObj, newObj runtime.Object, preserveResources bool) error {
	oldDaemonSet := &appsv1.DaemonSet{}
	if err := scheme.Convert(oldObj, oldDaemonSet, nil); err != nil {
		return err
	}

	newDaemonSet := &appsv1.DaemonSet{}
	if err := scheme.Convert(newObj, newDaemonSet, nil); err != nil {
		return err
	}

	mergePodTemplate(&oldDaemonSet.Spec.Template, &newDaemonSet.Spec.Template, preserveResources)

	return scheme.Convert(newDaemonSet, newObj, nil)
}

// mergeService merges new service into old service
func mergeService(scheme *runtime.Scheme, oldObj, newObj runtime.Object) error {
	oldService := &corev1.Service{}
	if err := scheme.Convert(oldObj, oldService, nil); err != nil {
		return err
	}

	if oldService.Spec.Type == "" {
		oldService.Spec.Type = corev1.ServiceTypeClusterIP
	}

	newService := &corev1.Service{}
	if err := scheme.Convert(newObj, newService, nil); err != nil {
		return err
	}

	if newService.Spec.Type == "" {
		newService.Spec.Type = corev1.ServiceTypeClusterIP
	}

	if len(oldService.Annotations) > 0 {
		mergedAnnotations := map[string]string{}
		for annotation, value := range oldService.Annotations {
			for _, keepAnnotation := range keepServiceAnnotations() {
				if strings.HasPrefix(annotation, keepAnnotation) {
					mergedAnnotations[annotation] = value
				}
			}
		}

		if len(newService.Annotations) > 0 {
			for annotation, value := range newService.Annotations {
				mergedAnnotations[annotation] = value
			}
		}

		newService.Annotations = mergedAnnotations
	}

	switch newService.Spec.Type {
	case corev1.ServiceTypeLoadBalancer, corev1.ServiceTypeNodePort:
		// do not override ports
		var ports []corev1.ServicePort

		for _, np := range newService.Spec.Ports {
			p := np

			for _, op := range oldService.Spec.Ports {
				if (np.Port == op.Port || np.Name == op.Name) && np.NodePort == 0 {
					p.NodePort = op.NodePort
				}
			}

			ports = append(ports, p)
		}
		newService.Spec.Ports = ports

		// do not override loadbalancer IP
		if newService.Spec.LoadBalancerIP == "" && oldService.Spec.LoadBalancerIP != "" {
			newService.Spec.LoadBalancerIP = oldService.Spec.LoadBalancerIP
		}

	case corev1.ServiceTypeExternalName:
		// there is no ClusterIP in this case
		return scheme.Convert(newService, newObj, nil)
	}

	// ClusterIP is immutable unless we want to transform the service into headless
	// where ClusterIP = None or if the previous type of the service was ExternalName
	// and the user wants to explicitly set an ClusterIP.
	if newService.Spec.ClusterIP != corev1.ClusterIPNone &&
		oldService.Spec.Type != corev1.ServiceTypeExternalName {
		newService.Spec.ClusterIP = oldService.Spec.ClusterIP
	}

	if oldService.Spec.Type == corev1.ServiceTypeLoadBalancer &&
		newService.Spec.Type == corev1.ServiceTypeLoadBalancer &&
		newService.Spec.ExternalTrafficPolicy == corev1.ServiceExternalTrafficPolicyLocal &&
		oldService.Spec.ExternalTrafficPolicy == corev1.ServiceExternalTrafficPolicyLocal &&
		newService.Spec.HealthCheckNodePort == 0 {
		newService.Spec.HealthCheckNodePort = oldService.Spec.HealthCheckNodePort
	}

	return scheme.Convert(newService, newObj, nil)
}

func mergeServiceAccount(scheme *runtime.Scheme, oldObj, newObj runtime.Object) error {
	oldServiceAccount := &corev1.ServiceAccount{}
	if err := scheme.Convert(oldObj, oldServiceAccount, nil); err != nil {
		return err
	}

	newServiceAccount := &corev1.ServiceAccount{}
	if err := scheme.Convert(newObj, newServiceAccount, nil); err != nil {
		return err
	}

	// Do not overwrite a ServiceAccount's '.secrets[]' list or '.imagePullSecrets[]'.
	newServiceAccount.Secrets = oldServiceAccount.Secrets
	newServiceAccount.ImagePullSecrets = oldServiceAccount.ImagePullSecrets

	return scheme.Convert(newServiceAccount, newObj, nil)
}

// mergeMapsBasedOnOldMap merges the values of the desired map into the current map.
// It takes an optional map of old desired values and removes any keys/values from the resulting map
// that were once desired (part of `old`) but are not desired anymore.
func mergeMapsBasedOnOldMap(desired, current, old map[string]string) map[string]string {
	out := make(map[string]string, len(current))
	// use current as base
	for k, v := range current {
		out[k] = v
	}

	// overwrite desired values
	for k, v := range desired {
		out[k] = v
	}

	// check if we should remove values which were once desired but are not desired anymore
	for k, oldValue := range old {
		currentValue, isInCurrent := current[k]
		if !isInCurrent || currentValue != oldValue {
			// is not part of the current map anymore or has been changed by the enduser -> don't remove
			continue
		}

		if _, isDesired := desired[k]; !isDesired {
			delete(out, k)
		}
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

func dropReferenceAnnotations(annotations map[string]string) map[string]string {
	out := make(map[string]string, len(annotations))
	for k, v := range annotations {
		if !strings.HasPrefix(k, references.AnnotationKeyPrefix) {
			out[k] = v
		}
	}
	return out
}

func keepServiceAnnotations() []string {
	return []string{"loadbalancer.openstack.org"}
}
