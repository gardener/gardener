// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"encoding/json"
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
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

func virtualClusterAdmissionManagedResourceName(extension *operatorv1alpha1.Extension) string {
	return fmt.Sprintf("extension-admission-virtual-%s", extension.Name)
}

func (r *Reconciler) reconcileVirtualClusterResources(ctx context.Context, log logr.Logger, virtualClusterClient client.Client, extension *operatorv1alpha1.Extension) error {
	// return early if we do not have to make a deployment
	if extension.Spec.Deployment == nil ||
		extension.Spec.Deployment.ExtensionDeployment == nil ||
		extension.Spec.Deployment.ExtensionDeployment.Helm == nil {
		return r.deleteVirtualClusterDeploymentResources(ctx, log, virtualClusterClient, extension)
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

func (r *Reconciler) deleteVirtualClusterDeploymentResources(ctx context.Context, log logr.Logger, virtualClusterClient client.Client, extension *operatorv1alpha1.Extension) error {
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

func (r *Reconciler) reconcileAdmissionVirtualClusterResources(ctx context.Context, log logr.Logger, virtualClusterClientSet kubernetes.Interface, extension *operatorv1alpha1.Extension) error {
	// return early if we do not have to make a deployment
	if extension.Spec.Deployment == nil ||
		extension.Spec.Deployment.AdmissionDeployment == nil ||
		extension.Spec.Deployment.AdmissionDeployment.VirtualCluster == nil ||
		extension.Spec.Deployment.AdmissionDeployment.VirtualCluster.Helm == nil {
		return r.deleteAdmissionVirtualClusterResources(ctx, log, extension)
	}

	archive, err := r.HelmRegistry.Pull(ctx, extension.Spec.Deployment.AdmissionDeployment.VirtualCluster.Helm.OCIRepository)
	if err != nil {
		return fmt.Errorf("failed pulling Helm chart from OCI repository: %w", err)
	}

	gardenerValues := map[string]any{
		"global": map[string]any{
			"virtualGarden": map[string]any{
				"enabled": true,
				"user": map[string]any{
					"name": fmt.Sprintf("system:serviceaccount:kube-system:%s", r.getVirtualClusterAccessSecret(extension).ServiceAccountName),
				},
			},
		},
	}

	var helmValues map[string]any
	if extension.Spec.Deployment.AdmissionDeployment.Values != nil {
		if err := json.Unmarshal(extension.Spec.Deployment.AdmissionDeployment.Values.Raw, &helmValues); err != nil {
			return err
		}
	}

	renderedChart, err := virtualClusterClientSet.ChartRenderer().RenderArchive(archive, extension.Name, v1beta1constants.GardenNamespace, utils.MergeMaps(helmValues, gardenerValues))
	if err != nil {
		return fmt.Errorf("failed rendering Helm chart: %w", err)
	}

	if err := managedresources.CreateForShoot(ctx, r.RuntimeClientSet.Client(), r.GardenNamespace, virtualClusterAdmissionManagedResourceName(extension), managedresources.LabelValueGardener, false, renderedChart.AsSecretData()); err != nil {
		return fmt.Errorf("failed creating ManagedResource: %w", err)
	}

	if err := managedresources.WaitUntilHealthy(ctx, r.RuntimeClientSet.Client(), r.GardenNamespace, virtualClusterAdmissionManagedResourceName(extension)); err != nil {
		return fmt.Errorf("failed waiting for ManagedResource to be healthy: %w", err)
	}

	r.Recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "Admission Helm chart applied successfully to virtual cluster")

	return nil
}

func (r *Reconciler) deleteAdmissionVirtualClusterResources(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	log.Info("Deleting admission ManagedResource for virtual cluster")
	if err := managedresources.DeleteForShoot(ctx, r.RuntimeClientSet.Client(), r.GardenNamespace, virtualClusterAdmissionManagedResourceName(extension)); err != nil {
		return fmt.Errorf("failed deleting ManagedResource: %w", err)
	}

	return managedresources.WaitUntilDeleted(ctx, r.RuntimeClientSet.Client(), r.GardenNamespace, virtualClusterAdmissionManagedResourceName(extension))
}
