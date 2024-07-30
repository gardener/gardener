// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func (r *Reconciler) reconcileVirtualClusterResources(ctx context.Context, log logr.Logger, virtualClusterClient client.Client, extension *operatorv1alpha1.Extension) error {
	// return early if we do not have to make a deployment
	if extension.Spec.Deployment == nil ||
		extension.Spec.Deployment.ExtensionDeployment == nil ||
		extension.Spec.Deployment.ExtensionDeployment.Helm == nil {
		return r.deleteVirtualClusterResources(ctx, log, virtualClusterClient, extension)
	}

	if err := r.reconcileControllerDeployment(ctx, virtualClusterClient, extension); err != nil {
		return fmt.Errorf("failed to reconcile ControllerDeployment: %w", err)
	}
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "ControllerDeployment applied successfully")

	if err := r.reconcileControllerRegistration(ctx, virtualClusterClient, extension); err != nil {
		return fmt.Errorf("failed to reconcile ControllerRegistration: %w", err)
	}
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "ControllerRegistration applied successfully")
	return nil
}

func (r *Reconciler) reconcileControllerDeployment(ctx context.Context, virtualClusterClient client.Client, extension *operatorv1alpha1.Extension) error {
	controllerDeployment := &gardencorev1.ControllerDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: extension.Name,
		},
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, virtualClusterClient, controllerDeployment,
		func() error {
			controllerDeployment.Helm = &gardencorev1.HelmControllerDeployment{
				Values:        extension.Spec.Deployment.ExtensionDeployment.Values,
				OCIRepository: extension.Spec.Deployment.ExtensionDeployment.Helm.OCIRepository,
			}
			return nil
		})
	return err
}

func (r *Reconciler) reconcileControllerRegistration(ctx context.Context, virtualClusterClient client.Client, extension *operatorv1alpha1.Extension) error {
	controllerRegistration := &gardencorev1beta1.ControllerRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: extension.Name,
		},
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, virtualClusterClient, controllerRegistration,
		func() error {
			// handle well known annotations
			if v, ok := extension.Annotations[v1beta1constants.AnnotationPodSecurityEnforce]; ok {
				metav1.SetMetaDataAnnotation(&controllerRegistration.ObjectMeta, v1beta1constants.AnnotationPodSecurityEnforce, v)
			} else {
				delete(controllerRegistration.Annotations, v1beta1constants.AnnotationPodSecurityEnforce)
			}

			controllerRegistration.Spec = gardencorev1beta1.ControllerRegistrationSpec{
				Resources: extension.Spec.Resources,
				Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
					Policy:       extension.Spec.Deployment.ExtensionDeployment.Policy,
					SeedSelector: extension.Spec.Deployment.ExtensionDeployment.SeedSelector,
					DeploymentRefs: []gardencorev1beta1.DeploymentRef{
						{
							Name: extension.Name,
						},
					},
				},
			}
			return nil
		})
	return err
}

func (r *Reconciler) deleteVirtualClusterResources(ctx context.Context, log logr.Logger, virtualClusterClient client.Client, extension *operatorv1alpha1.Extension) error {
	log.Info("Deleting extension virtual resources")
	var (
		controllerDeployment = &gardencorev1.ControllerDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: extension.Name,
			}}

		controllerRegistration = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: extension.Name,
			},
		}
	)

	log.Info("Deleting ControllerRegistration and ControllerDeployment")
	if err := kubernetesutils.DeleteObjects(ctx, virtualClusterClient, controllerDeployment, controllerRegistration); err != nil {
		return err
	}

	log.Info("Waiting until ControllerRegistration is gone")
	if err := kubernetesutils.WaitUntilResourceDeleted(ctx, virtualClusterClient, controllerRegistration, 5*time.Second); err != nil {
		return err
	}
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Deletion", "Successfully deleted ControllerRegistration")

	log.Info("Waiting until ControllerDeployment is gone")
	if err := kubernetesutils.WaitUntilResourceDeleted(ctx, virtualClusterClient, controllerDeployment, 5*time.Second); err != nil {
		return err
	}
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Deletion", "Successfully deleted ControllerDeployment")
	return nil
}
