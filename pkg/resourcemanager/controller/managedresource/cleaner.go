// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresource

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// cleanup tries to cleanup any resources created by the given object, that are left in the target cluster. It returns a
// bool indicating whether there are still some deletions pending and an error if any occurred.
func cleanup(ctx context.Context, c client.Client, scheme *runtime.Scheme, obj *unstructured.Unstructured, deletePVCs bool) error {
	switch obj.GroupVersionKind().GroupKind() {
	case appsv1.SchemeGroupVersion.WithKind("StatefulSet").GroupKind(), extensionsv1beta1.SchemeGroupVersion.WithKind("StatefulSet").GroupKind():
		return cleanupStatefulSet(ctx, c, scheme, obj, deletePVCs)
	}

	return nil
}

// cleanupStatefulSet tries to delete all PVCs created by this StatefulSet if the ManagedResource is configured accordingly.
func cleanupStatefulSet(ctx context.Context, c client.Client, scheme *runtime.Scheme, obj runtime.Object, deletePVCs bool) error {
	if !deletePVCs {
		return nil
	}

	errMsg := "failed cleaning up PersistentVolumeClaims of StatefulSet"

	statefulSet := &appsv1.StatefulSet{}
	if err := scheme.Convert(obj, statefulSet, nil); err != nil {
		return fmt.Errorf("%s: could not convert object to StatefulSet: %v", errMsg, err)
	}

	if len(statefulSet.Spec.VolumeClaimTemplates) == 0 {
		return nil
	}

	// the StatefulSet controller computes the labels for the PVCs by combining the labels given in the volumeClaimTemplate and the StatefulSet's selector
	// with the selector labels taking precedence, so we can delete all PVCs by using the StatefulSet's selector.
	// (ref: https://github.com/kubernetes/kubernetes/blob/d3a0c149a36b912a5c3ab3cc63047b1cbc758720/pkg/controller/statefulset/stateful_set_utils.go#L146-L152)
	pvcLabels := statefulSet.Spec.Selector.MatchLabels

	// first check, if there are any PVCs left to delete for this StatefulSet
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := c.List(ctx, pvcList, client.InNamespace(statefulSet.Namespace), client.MatchingLabels(pvcLabels)); err != nil {
		return fmt.Errorf("%s: could not list PVCs of StatefulSet: %v", errMsg, err)
	}
	if len(pvcList.Items) == 0 {
		return nil
	}

	if err := c.DeleteAllOf(ctx, &corev1.PersistentVolumeClaim{}, client.InNamespace(statefulSet.Namespace), client.MatchingLabels(pvcLabels)); err != nil {
		return fmt.Errorf("%s: %v", errMsg, err)
	}

	return nil
}
