// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package common

import (
	"context"
	"fmt"
	"time"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// TODO(rickardsjp, istvanballok): remove this package in release v1.77

// LokiPvcExists checks if the loki-loki-0 PVC exists in the given namespace.
func LokiPvcExists(ctx context.Context, k8sClient client.Client, namespace string, log logr.Logger) (bool, error) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "loki-loki-0",
			Namespace: namespace,
		},
	}
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(pvc), pvc); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Loki2vali: Loki PVC not found", "lokiNamespace", namespace)
			return false, nil
		} else {
			return false, err
		}
	}
	log.Info("Loki2vali: Loki PVC found", "lokiNamespace", namespace)
	return true, nil
}

// DeleteLokiRetainPvc deletes all Loki resources in a given namespace.
func DeleteLokiRetainPvc(ctx context.Context, k8sClient client.Client, namespace string, log logr.Logger) error {
	resources := []client.Object{
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-loki", Namespace: namespace}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-loki", Namespace: namespace}},
		&hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: namespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: namespace}},
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: namespace}},
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: namespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "shoot-access-promtail", Namespace: namespace}},
		// We retain the PVC and reuse it with Vali.
		// &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "loki-loki-0", Namespace: namespace}},
	}

	// Loki currently needs 30s to terminate because after 1s of graceful shutdown preparation it waits for 30s until it is eventually
	// forcefully killed by the kubelet. We reduce the graceful termination timeout from 30s to 5s here so that the migration from loki to vali
	// can succeed in the 30s deadline of the shoot reconciliation.
	log.Info("Loki2vali: Deleting the pod loki-0 with a grace period of 5 seconds", "lokiNamespace", namespace)
	if err := k8sClient.Delete(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "loki-0", Namespace: namespace}}, client.GracePeriodSeconds(5)); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	log.Info("Loki2vali: Deleting the other artifacts like the loki Statefulset", "lokiNamespace", namespace)
	if err := kubernetesutils.DeleteObjects(ctx, k8sClient, resources...); err != nil {
		return err
	}

	deleteOptions := []client.DeleteAllOfOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			v1beta1constants.GardenRole: "logging",
			v1beta1constants.LabelApp:   "loki",
		},
	}

	return k8sClient.DeleteAllOf(ctx, &corev1.ConfigMap{}, deleteOptions...)
}

// getLokiPvc returns the PVC used by the loki-0 pod: "loki-loki-0".
func getLokiPvc(ctx context.Context, k8sClient client.Client, namespace string, log logr.Logger) (lokiPvc *corev1.PersistentVolumeClaim, shouldContinue bool, err error) {
	log.Info("Loki2vali: Get Loki PVC", "lokiNamespace", namespace)
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "loki-loki-0",
			Namespace: namespace,
		},
	}
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(pvc), pvc); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		} else {
			log.Info("Loki2vali: Error retrieving Loki PVC, aborting", "lokiNamespace", namespace, "lokiError", err)
			return nil, false, err
		}
	}
	log.Info("Loki2vali: Loki PVC found, attempting to rename it: loki-loki-0 --> vali-vali-0, so that it can be reused by the vali-0 pod", "lokiNamespace", namespace)
	return pvc, true, nil
}

// waitForLokiPodTermination waits for the loki-0 pod to terminate.
func waitForLokiPodTermination(ctx context.Context, k8sClient client.Client, namespace string, log logr.Logger) error {
	log.Info("Loki2vali: Verify that the pod loki-0 is not running, otherwise we cannot delete the PVC", "lokiNamespace", namespace)
	// ensure that the context has a deadline
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(30*time.Second))
	defer cancel()
	deadline, _ := ctx.Deadline()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "loki-0",
			Namespace: namespace,
		},
	}
	if err := wait.PollUntilWithContext(ctx, 1*time.Second, func(context.Context) (done bool, err error) {
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Loki2vali: pod loki-0 not found, continuing with the rename", "lokiNamespace", namespace)
				return true, nil
			} else {
				return true, fmt.Errorf("Loki2vali: %v: Error retrieving pod loki-0, aborting: %w", namespace, err)
			}
		}
		log.Info("Loki2vali: Waiting for pod loki-0 to terminate", "lokiNamespace", namespace, "timeLeft", time.Until(deadline))
		return false, nil
	}); err != nil && err == wait.ErrWaitTimeout {
		err := fmt.Errorf("Loki2vali: %v: Timeout while waiting for the loki-0 pod to terminate", namespace)
		log.Info("Loki2vali:", "lokiError", err)
		return err
	} else {
		return err
	}
}

// getLokiPv returns the PV used by the "loki-loki-0" PVC.
func getLokiPv(ctx context.Context, k8sClient client.Client, namespace string, pvc *corev1.PersistentVolumeClaim, log logr.Logger) (*corev1.PersistentVolume, error) {
	log.Info("Loki2vali: Get Loki PV", "lokiNamespace", namespace)
	pvId := pvc.Spec.VolumeName
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvId,
		},
	}
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(pv), pv); err != nil {
		return nil, err
	}
	return pv, nil
}

// patchLokiPvReclaimPolicy changes the ReclaimPolicy of the PV used by the "loki-loki-0" PVC to "Retain".
func patchLokiPvReclaimPolicy(ctx context.Context, k8sClient client.Client, namespace string, pv *corev1.PersistentVolume, log logr.Logger) error {
	log.Info("Loki2vali: Change the Loki PV's PersistentVolumeReclaimPolicy to Retain temporarily. This way the PV is not deleted when the PVC is deleted during the migration of Loki to Vali", "lokiNamespace", namespace)
	patch := client.MergeFrom(pv.DeepCopy())
	pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
	if err := k8sClient.Patch(ctx, pv, patch); err != nil {
		return err
	}
	log.Info("Loki2vali: Successfully changed the Loki PV's PersistentVolumeReclaimPolicy to Retain", "lokiNamespace", namespace)
	return nil
}

// revertLokiPvReclaimPolicy changes the ReclaimPolicy of the PV used by the "loki-loki-0" PVC to "Delete".
func revertLokiPvReclaimPolicy(ctx context.Context, k8sClient client.Client, namespace string, pv *corev1.PersistentVolume, log logr.Logger) error {
	log.Info("Loki2vali: Change the Vali PV's PersistentVolumeReclaimPolicy back to be Delete. The reclaim policy should be Delete so that the PV is deleted when the Logging stack (including the PVC) is deleted", "lokiNamespace", namespace)
	patch := client.MergeFrom(pv.DeepCopy())
	pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimDelete

	if err := k8sClient.Patch(ctx, pv, patch); err != nil {
		return err
	}
	log.Info("Loki2vali: Successfully changed the Vali PV's PersistentVolumeReclaimPolicy back to be Delete", "lokiNamespace", namespace)
	return nil
}

// deleteLokiPvc deletes the "loki-loki-0" PVC.
func deleteLokiPvc(ctx context.Context, k8sClient client.Client, namespace string, pvc *corev1.PersistentVolumeClaim, log logr.Logger) error {
	log.Info("Loki2vali: Delete Loki PVC", "lokiNamespace", namespace)
	if err := kubernetesutils.DeleteObject(ctx, k8sClient, pvc); err != nil {
		return err
	}
	// ensure that the context has a deadline
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(30*time.Second))
	defer cancel()
	deadline, _ := ctx.Deadline()

	if err := wait.PollUntilWithContext(ctx, 1*time.Second, func(context.Context) (done bool, err error) {
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(pvc), pvc); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Loki2vali: Successfully deleted the Loki PVC", "lokiNamespace", namespace)
				return true, nil
			} else {
				return true, err
			}
		}
		log.Info("Loki2vali: Wait for Loki PVC deletion to complete", "lokiNamespace", namespace, "timeLeft", time.Until(deadline))
		return false, nil
	}); err != nil && err == wait.ErrWaitTimeout {
		err := fmt.Errorf("Loki2vali: %v: Timeout while waiting for the loki-loki-0 PVC to terminate", namespace)
		log.Info("Loki2vali:", "lokiError", err)
		return err
	} else {
		return err
	}
}

// deleteLokiPv deletes the PV used by loki.
func deleteLokiPv(ctx context.Context, k8sClient client.Client, namespace string, pv *corev1.PersistentVolume, log logr.Logger) error {
	log.Info("Loki2vali: Delete Loki PV", "lokiNamespace", namespace)
	if err := kubernetesutils.DeleteObject(ctx, k8sClient, pv); err != nil {
		return err
	}
	log.Info("Loki2vali: Successfully deleted the Loki PV", "lokiNamespace", namespace)
	return nil
}

// deleteValiPvc deletes the "vali-vali-0" PVC.
func deleteValiPvc(ctx context.Context, k8sClient client.Client, namespace string, pvc *corev1.PersistentVolumeClaim, log logr.Logger) error {
	log.Info("Loki2vali: Delete Vali PVC", "lokiNamespace", namespace)
	if err := kubernetesutils.DeleteObject(ctx, k8sClient, pvc); err != nil {
		return err
	}
	log.Info("Loki2vali: Successfully deleted the Vali PVC", "lokiNamespace", namespace)
	return nil
}

// assertPvStillExists asserts that the PV formerly used by the "loki-loki-0" PVC still exists.
func assertPvStillExists(ctx context.Context, k8sClient client.Client, namespace string, pv *corev1.PersistentVolume) (isDeleted bool, err error) {
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(pv), pv); err != nil {
		if apierrors.IsNotFound(err) {
			return true, fmt.Errorf("Loki2vali: %v: Loki PV is deleted, %w", namespace, err)
		}
		return false, err
	}
	if pv.ObjectMeta.DeletionTimestamp != nil {
		return true, fmt.Errorf("Loki2vali: %v: Loki PV is deleted", namespace)
	}
	return false, nil
}

// removeLokiPvClaimRef removes the claimRef from the PV of the "loki-loki-0" PVC.
func removeLokiPvClaimRef(ctx context.Context, k8sClient client.Client, namespace string, pv *corev1.PersistentVolume, log logr.Logger) error {
	log.Info("Loki2vali: Remove Loki PV's ClaimRef. This is needed so that it can be bound to the soon to be created Vali PVC", "lokiNamespace", namespace)
	patch := client.MergeFrom(pv.DeepCopy())
	pv.Spec.ClaimRef = nil
	if err := k8sClient.Patch(ctx, pv, patch); err != nil {
		log.Info("Loki2vali: removeLokiPvClaimRef failed", "lokiNamespace", namespace, "lokiError", err)
		return err
	}
	log.Info("Loki2vali: Successfully removed the claim reference from the PV", "lokiNamespace", namespace)
	return nil
}

// createValiPvc creates a new PVC "vali-vali-0" that uses the PV formerly used by the "loki-loki-0" PVC.
func createValiPvc(ctx context.Context, k8sClient client.Client, namespace string, pvc *corev1.PersistentVolumeClaim, log logr.Logger) (*corev1.PersistentVolumeClaim, error) {
	log.Info("Loki2vali: Create Vali PVC. It is basically a copy of the Loki PVC, with the name changed from loki-loki-0 to vali-vali-0", "lokiNamespace", namespace)
	labels := pvc.DeepCopy().Labels
	for k, v := range labels {
		if v == "loki" {
			labels[k] = "vali"
		}
	}
	valiPvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   pvc.Namespace,
			Name:        "vali-vali-0",
			Annotations: pvc.DeepCopy().Annotations,
			Labels:      labels,
		},
		Spec: *pvc.Spec.DeepCopy(),
	}
	if err := k8sClient.Create(ctx, valiPvc); err != nil {
		err2 := fmt.Errorf("Loki2vali: %v: Create Vali PVC failed. %w", namespace, err)
		log.Info("Loki2vali:", "lokiError", err2)
		return nil, err2
	}
	return valiPvc, nil
}

// waitForValiPvcToBeBound waits until the "vali-vali-0" PVC is bound to the PV formerly used by the "loki-loki-0" PVC.
func waitForValiPvcToBeBound(ctx context.Context, k8sClient client.Client, namespace string, valiPvc *corev1.PersistentVolumeClaim, log logr.Logger) error {
	// ensure that the context has a deadline
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(1*time.Minute))
	defer cancel()
	deadline, _ := ctx.Deadline()
	if err := wait.PollUntilWithContext(ctx, 1*time.Second, func(context.Context) (done bool, err error) {
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(valiPvc), valiPvc); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Loki2vali: Vali PVC is not yet created", "lokiNamespace", namespace, "lokiError", err)
				return false, nil
			}
			log.Info("Loki2vali: waitForValiPvcToBeBound failed", "lokiNamespace", namespace, "lokiError", err)
			return true, err
		}
		if valiPvc.Status.Phase == corev1.ClaimBound {
			log.Info("Loki2vali: PVC is bound", "lokiNamespace", namespace)
			return true, nil
		} else if valiPvc.Status.Phase == corev1.ClaimLost {
			err := fmt.Errorf("Loki2vali: %v: Vali PVC is in Lost state, triggering recovery", namespace)
			log.Info("Loki2vali:", "lokiError", err)
			return true, err
		}
		log.Info("Loki2vali: Wait for the Vali PVC to be bound", "lokiNamespace", namespace, "timeLeft", time.Until(deadline))
		return false, nil
	}); err != nil && err == wait.ErrWaitTimeout {
		err := fmt.Errorf("Loki2vali: %v: Timeout while waiting for the vali PVC to be bound", namespace)
		log.Info("Loki2vali:", "lokiError", err)
		return err
	} else {
		return err
	}
}

// RenameLokiPvcToValiPvc "renames" the PVC used by the loki-0 pod: "loki-loki-0" to "vali-vali-0". It can then be used by the vali-0 pod of the vali StatefulSet.
// It is not possible in kubernetes to rename a PVC, so we have to create a new PVC after deleting the old one.
// The PV of the PVC is retained and reused by the new PVC. To achieve this, the PV's ReclaimPolicy is temporarily set to "Retain" before deleting the PVC.
// The PV's ReclaimPolicy is reset at the end back to "Delete" so that the PV is deleted when the PVC is deleted for another reason.
func RenameLokiPvcToValiPvc(ctx context.Context, k8sClient client.Client, namespace string, log logr.Logger) error {
	log.Info("Loki2vali: Entering RenameLokiPvcToValiPvc", "lokiNamespace", namespace)

	pvc, shouldContinue, err := getLokiPvc(ctx, k8sClient, namespace, log)
	if err != nil {
		log.Info("Loki2vali: Error happened in getLokiPvc, we'll retry in the next reconciliation loop", "lokiNamespace", namespace, "lokiError", err)
		return err
	} else if !shouldContinue {
		log.Info("Loki2vali: Loki PVC not found, nothing to do", "lokiNamespace", namespace)
		return nil // ok, nothing to do
	}

	if err := waitForLokiPodTermination(ctx, k8sClient, namespace, log); err != nil {
		log.Info("Loki2vali: Error happened in waitForLokiPodTermination, we'll retry in the next reconciliation loop", "lokiNamespace", namespace, "lokiError", err)
		return err
	}

	pv, err := getLokiPv(ctx, k8sClient, namespace, pvc, log)
	if err != nil {
		log.Info("Loki2vali: Error happened in getLokiPv, we'll retry in the next reconciliation loop", "lokiNamespace", namespace, "lokiError", err)
		return err
	}

	deadline, hasDeadline := ctx.Deadline()
	if hasDeadline && time.Until(deadline) < 15*time.Second {
		err := fmt.Errorf("Loki2vali: %v: Bailing out to avoid hitting context deadline in this reconciliation loop, time remaining is %v", namespace, time.Until(deadline))
		log.Info("Loki2vali:", "lokiError", err)
		return err
	} else if hasDeadline {
		log.Info("Loki2vali: Context deadline is in the future, continuing with the rename", "lokiNamespace", namespace, "timeLeft", time.Until(deadline))
	} else {
		log.Info("Loki2vali: Context has no deadline, continuing with the rename", "lokiNamespace", namespace)
	}

	// Change the PV's reclaim policy to retain. We need to cover all the error cases to either change it back or clean up.
	if err := patchLokiPvReclaimPolicy(ctx, k8sClient, namespace, pv, log); err != nil {
		recoveryContext, cancel := context.WithDeadline(context.TODO(), time.Now().Add(1*time.Minute))
		defer cancel()
		err2 := revertLokiPvReclaimPolicy(recoveryContext, k8sClient, namespace, pv, log)
		errs := fmt.Errorf("Loki2vali: %v: Error %w happened in patchLokiPvReclaimPolicy, cleanup attempt: revertLokiPvReclaimPolicy (err: %w)", namespace, err, err2)
		log.Info("Loki2vali:", "lokiError", errs)
		return errs
	}

	// Wait a bit so that the two changes (PV's reclaim policy and PVC's deletion) can be processed in this order by the kube-controller-manager.
	time.Sleep(1 * time.Second)

	if err := deleteLokiPvc(ctx, k8sClient, namespace, pvc, log); err != nil {
		recoveryContext, cancel := context.WithDeadline(context.TODO(), time.Now().Add(1*time.Minute))
		defer cancel()
		err2 := deleteLokiPvc(recoveryContext, k8sClient, namespace, pvc, log)
		err3 := deleteLokiPv(recoveryContext, k8sClient, namespace, pv, log)
		errs := fmt.Errorf("Loki2vali: %v: Error %w happened in deleteLokiPvc, cleanup attempt: deleteLokiPvc (err: %w), deleteLokiPv (err: %w)", namespace, err, err2, err3)
		log.Info("Loki2vali:", "lokiError", errs)
		return errs
	}

	isDeleted, err := assertPvStillExists(ctx, k8sClient, namespace, pv)
	if err != nil {
		recoveryContext, cancel := context.WithDeadline(context.TODO(), time.Now().Add(1*time.Minute))
		defer cancel()
		err2 := deleteLokiPv(recoveryContext, k8sClient, namespace, pv, log)
		errs := fmt.Errorf("Loki2vali: %v: Error %w happened in assertPvStillExists. Cleanup attempt: deleteLokiPv (err: %w)", namespace, err, err2)
		log.Info("Loki2vali:", "lokiError", errs)
		return errs
	} else if isDeleted {
		return fmt.Errorf("Loki2vali: %v: Loki PV is deleted. This means we lost the disk. The PVC and the PV is deleted, we can retry the next time and we'll get a new disk", namespace)
	}

	if err := removeLokiPvClaimRef(ctx, k8sClient, namespace, pv, log); err != nil {
		recoveryContext, cancel := context.WithDeadline(context.TODO(), time.Now().Add(1*time.Minute))
		defer cancel()
		err2 := deleteLokiPv(recoveryContext, k8sClient, namespace, pv, log)
		errs := fmt.Errorf("Loki2vali: %v: Error %w happened in removeLokiPvClaimRef. Cleanup attempt: deleteLokiPv (err: %w)", namespace, err, err2)
		log.Info("Loki2vali:", "lokiError", errs)
		return errs
	}

	pvc, err = createValiPvc(ctx, k8sClient, namespace, pvc, log)
	if err != nil {
		recoveryContext, cancel := context.WithDeadline(context.TODO(), time.Now().Add(1*time.Minute))
		defer cancel()
		err2 := deleteValiPvc(recoveryContext, k8sClient, namespace, pvc, log)
		err3 := deleteLokiPv(recoveryContext, k8sClient, namespace, pv, log)
		errs := fmt.Errorf("Loki2vali: %v: Error %w happened in createValiPvc. Cleanup attempt: deleteValiPvc (err: %w), deleteLokiPv (err: %w)", namespace, err, err2, err3)
		log.Info("Loki2vali:", "lokiError", errs)
		return errs
	}

	extendedContext, cancel := context.WithDeadline(context.TODO(), time.Now().Add(1*time.Minute))
	defer cancel()

	if err := waitForValiPvcToBeBound(extendedContext, k8sClient, namespace, pvc, log); err != nil {
		recoveryContext, cancel := context.WithDeadline(context.TODO(), time.Now().Add(1*time.Minute))
		defer cancel()
		err2 := deleteValiPvc(recoveryContext, k8sClient, namespace, pvc, log)
		err3 := deleteLokiPv(recoveryContext, k8sClient, namespace, pv, log)
		errs := fmt.Errorf("Loki2vali: %v: Error %w happened in waitForValiPvcToBeBound. Cleanup attempt: deleteValiPvc (err: %w), deleteLokiPv (err: %w)", namespace, err, err2, err3)
		log.Info("Loki2vali:", "lokiError", errs)
		return errs
	}

	extendedContext2, cancel := context.WithDeadline(context.TODO(), time.Now().Add(1*time.Minute))
	defer cancel()
	if err := revertLokiPvReclaimPolicy(extendedContext2, k8sClient, namespace, pv, log); err != nil {
		recoveryContext, cancel := context.WithDeadline(context.TODO(), time.Now().Add(1*time.Minute))
		defer cancel()
		err2 := deleteValiPvc(recoveryContext, k8sClient, namespace, pvc, log)
		err3 := deleteLokiPv(recoveryContext, k8sClient, namespace, pv, log)
		errs := fmt.Errorf("Loki2vali: %v: Error %w happened in revertLokiPvReclaimPolicy. Cleanup attempt: deleteValiPvc (err: %w), deleteLokiPv (err: %w)", namespace, err, err2, err3)
		log.Info("Loki2vali:", "lokiError", errs)
		return errs
	}

	log.Info("Loki2vali: Successfully finished RenameLokiPvcToValiPvc", "lokiNamespace", namespace)
	return nil
}

// DeleteGrafana deletes the Grafana resources that are no longer necessary due to the migration to Plutono.
func DeleteGrafana(ctx context.Context, k8sClient kubernetes.Interface, namespace string) error {
	if k8sClient == nil {
		return fmt.Errorf("require kubernetes client")
	}

	deleteOptions := []client.DeleteAllOfOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			"component": "grafana",
		},
	}

	if err := k8sClient.Client().DeleteAllOf(ctx, &appsv1.Deployment{}, append(deleteOptions, client.PropagationPolicy(metav1.DeletePropagationForeground))...); err != nil {
		return err
	}

	if err := k8sClient.Client().DeleteAllOf(ctx, &corev1.ConfigMap{}, deleteOptions...); err != nil {
		return err
	}

	if err := k8sClient.Client().DeleteAllOf(ctx, &networkingv1.Ingress{}, deleteOptions...); err != nil {
		return err
	}

	if err := k8sClient.Client().DeleteAllOf(ctx, &corev1.Secret{}, deleteOptions...); err != nil {
		return err
	}

	if err := k8sClient.Client().Delete(
		ctx,
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "grafana",
				Namespace: namespace,
			}},
	); client.IgnoreNotFound(err) != nil {
		return err
	}

	return nil
}
