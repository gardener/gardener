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

func (r *Reconciler) reconcileControllerRegistration(ctx context.Context, log logr.Logger, virtualClusterClient client.Client, extension *operatorv1alpha1.Extension) error {
	if extension.Spec.Deployment == nil ||
		extension.Spec.Deployment.ExtensionDeployment == nil ||
		extension.Spec.Deployment.ExtensionDeployment.Helm == nil {
		if err := r.deleteControllerRegistration(ctx, log, virtualClusterClient, extension); err != nil {
			return err
		}
		r.Recorder.Event(extension, corev1.EventTypeNormal, "Deletion", "ControllerRegistration and ControllerDeployment deleted successfully")

		return nil
	}

	if err := r.createOrUpdateControllerRegistration(ctx, virtualClusterClient, extension); err != nil {
		return fmt.Errorf("failed to reconcile ControllerRegistration: %w", err)
	}
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "ControllerRegistration and ControllerDeployment applied successfully")

	return nil
}

func (r *Reconciler) createOrUpdateControllerRegistration(ctx context.Context, virtualClusterClient client.Client, extension *operatorv1alpha1.Extension) error {
	var (
		controllerDeployment   = emptyControllerDeployment(extension)
		controllerRegistration = emptyControllerRegistration(extension)
	)

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, virtualClusterClient, controllerDeployment,
		func() error {
			controllerDeployment.Helm = &gardencorev1.HelmControllerDeployment{
				Values:        extension.Spec.Deployment.ExtensionDeployment.Values,
				OCIRepository: extension.Spec.Deployment.ExtensionDeployment.Helm.OCIRepository,
			}
			return nil
		})
	if err != nil {
		return err
	}

	_, err = controllerutils.GetAndCreateOrMergePatch(ctx, virtualClusterClient, controllerRegistration,
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

func (r *Reconciler) deleteControllerRegistration(ctx context.Context, log logr.Logger, virtualClusterClient client.Client, extension *operatorv1alpha1.Extension) error {
	var (
		controllerDeployment   = emptyControllerDeployment(extension)
		controllerRegistration = emptyControllerRegistration(extension)
	)

	log.Info("Deleting ControllerRegistration and ControllerDeployment")
	if err := kubernetesutils.DeleteObjects(ctx, virtualClusterClient, controllerDeployment, controllerRegistration); err != nil {
		return err
	}

	log.Info("Waiting until ControllerRegistration is gone")
	return kubernetesutils.WaitUntilResourceDeleted(ctx, virtualClusterClient, controllerRegistration, 5*time.Second)
}

func emptyControllerRegistration(extension *operatorv1alpha1.Extension) *gardencorev1beta1.ControllerRegistration {
	return &gardencorev1beta1.ControllerRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: extension.Name,
		},
	}
}

func emptyControllerDeployment(extension *operatorv1alpha1.Extension) *gardencorev1.ControllerDeployment {
	return &gardencorev1.ControllerDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: extension.Name,
		},
	}
}
