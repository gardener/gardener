// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerregistration

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// Interface contains functions to handle the registration of extensions for shoot clusters.
type Interface interface {
	// Reconcile creates or updates the ControllerRegistration and ControllerDeployment for the given extension.
	Reconcile(context.Context, logr.Logger, *operatorv1alpha1.Extension) error
	// Delete deletes the ControllerRegistration and ControllerDeployment for the given extension.
	Delete(context.Context, logr.Logger, *operatorv1alpha1.Extension) error
}

type registration struct {
	runtimeClient client.Client
	recorder      record.EventRecorder

	gardenNamespace string
}

// Reconcile creates or updates the ControllerRegistration and ControllerDeployment for the given extension.
// If the extension doesn't define an extension deployment, the registration is deleted.
func (r *registration) Reconcile(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	if extension.Spec.Deployment == nil ||
		extension.Spec.Deployment.ExtensionDeployment == nil ||
		extension.Spec.Deployment.ExtensionDeployment.Helm == nil {
		if err := r.Delete(ctx, log, extension); err != nil {
			return err
		}
		r.recorder.Event(extension, corev1.EventTypeNormal, "Deletion", "ControllerRegistration and ControllerDeployment deleted successfully")

		return nil
	}

	log.Info("Deploying ControllerRegistration and ControllerDeployment")
	if err := r.createOrUpdateControllerRegistration(ctx, extension); err != nil {
		return fmt.Errorf("failed to reconcile ControllerRegistration: %w", err)
	}
	r.recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "ControllerRegistration and ControllerDeployment applied successfully")

	return nil
}

func (r *registration) createOrUpdateControllerRegistration(ctx context.Context, extension *operatorv1alpha1.Extension) error {
	var (
		registry = managedresources.NewRegistry(kubernetes.GardenScheme, kubernetes.GardenCodec, kubernetes.GardenSerializer)

		controllerDeployment = &gardencorev1.ControllerDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: extension.Name,
			},
			Helm: &gardencorev1.HelmControllerDeployment{
				Values:        extension.Spec.Deployment.ExtensionDeployment.Values,
				OCIRepository: extension.Spec.Deployment.ExtensionDeployment.Helm.OCIRepository,
			},
		}

		controllerRegistration = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: extension.Name,
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
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
			},
		}
	)

	if v, ok := extension.Annotations[v1beta1constants.AnnotationPodSecurityEnforce]; ok {
		metav1.SetMetaDataAnnotation(&controllerRegistration.ObjectMeta, v1beta1constants.AnnotationPodSecurityEnforce, v)
	} else {
		delete(controllerRegistration.Annotations, v1beta1constants.AnnotationPodSecurityEnforce)
	}

	data, err := registry.AddAllAndSerialize(controllerDeployment, controllerRegistration)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, r.runtimeClient, r.gardenNamespace, managedResourceName(extension), managedresources.LabelValueOperator, false, data)
}

// Delete deletes the ControllerRegistration and ControllerDeployment for the given extension.
func (r *registration) Delete(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	mrName := managedResourceName(extension)

	log.Info("Deleting extension registration ManagedResource", "managedResource", client.ObjectKey{Name: mrName, Namespace: r.gardenNamespace})
	if err := managedresources.DeleteForShoot(ctx, r.runtimeClient, r.gardenNamespace, mrName); err != nil {
		return fmt.Errorf("failed deleting ManagedResource: %w", err)
	}

	return managedresources.WaitUntilDeleted(ctx, r.runtimeClient, r.gardenNamespace, mrName)
}

func managedResourceName(extension *operatorv1alpha1.Extension) string {
	return fmt.Sprintf("extension-registration-%s", extension.Name)
}

// New creates a new handler for ControllerRegistrations.
func New(runtimeClient client.Client, recorder record.EventRecorder, gardenNamespace string) Interface {
	return &registration{
		runtimeClient: runtimeClient,
		recorder:      recorder,

		gardenNamespace: gardenNamespace,
	}
}
