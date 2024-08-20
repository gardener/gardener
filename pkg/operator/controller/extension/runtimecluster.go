// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

func runtimeClusterAdmissionManagedResourceName(extension *operatorv1alpha1.Extension) string {
	return fmt.Sprintf("extension-admission-runtime-%s", extension.Name)
}

func (r *Reconciler) reconcileAdmissionRuntimeClusterResources(ctx context.Context, log logr.Logger, genericTokenKubeconfigSecretName string, extension *operatorv1alpha1.Extension) error {
	if extension.Spec.Deployment == nil ||
		extension.Spec.Deployment.AdmissionDeployment == nil ||
		extension.Spec.Deployment.AdmissionDeployment.RuntimeCluster == nil ||
		extension.Spec.Deployment.AdmissionDeployment.RuntimeCluster.Helm == nil {
		return r.deleteAdmissionRuntimeClusterResources(ctx, log, extension)
	}

	archive, err := r.HelmRegistry.Pull(ctx, extension.Spec.Deployment.AdmissionDeployment.RuntimeCluster.Helm.OCIRepository)
	if err != nil {
		return fmt.Errorf("failed pulling Helm chart from OCI repository: %w", err)
	}

	accessSecret := r.getVirtualClusterAccessSecret(extension)
	if err := accessSecret.Reconcile(ctx, r.RuntimeClientSet.Client()); err != nil {
		return fmt.Errorf("failed reconciling access secret: %w", err)
	}

	gardenerValues := map[string]any{
		"priorityClassName": v1beta1constants.PriorityClassNameGardenSystem400,
		"projectedKubeconfig": map[string]any{
			"baseMountPath":               gardenerutils.VolumeMountPathGenericKubeconfig,
			"genericKubeconfigSecretName": genericTokenKubeconfigSecretName,
			"tokenSecretName":             accessSecret.Secret.Name,
		},
	}

	var helmValues map[string]any
	if extension.Spec.Deployment.AdmissionDeployment.Values != nil {
		if err := json.Unmarshal(extension.Spec.Deployment.AdmissionDeployment.Values.Raw, &helmValues); err != nil {
			return err
		}
	}

	renderedChart, err := r.RuntimeClientSet.ChartRenderer().RenderArchive(archive, extension.Name, v1beta1constants.GardenNamespace, utils.MergeMaps(helmValues, gardenerValues))
	if err != nil {
		return fmt.Errorf("failed rendering Helm chart: %w", err)
	}

	if err := managedresources.CreateForSeedWithLabels(
		ctx,
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		runtimeClusterAdmissionManagedResourceName(extension),
		false,
		map[string]string{managedresources.LabelKeyOrigin: managedresources.LabelValueOperator},
		renderedChart.AsSecretData(),
	); err != nil {
		return fmt.Errorf("failed creating ManagedResource: %w", err)
	}

	if err := managedresources.WaitUntilHealthy(ctx, r.RuntimeClientSet.Client(), r.GardenNamespace, runtimeClusterAdmissionManagedResourceName(extension)); err != nil {
		return fmt.Errorf("failed waiting for ManagedResource to be healthy: %w", err)
	}

	r.Recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "Admission Helm chart applied successfully to runtime cluster")

	return nil
}

func (r *Reconciler) deleteAdmissionRuntimeClusterResources(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	managedResourceName := runtimeClusterAdmissionManagedResourceName(extension)

	log.Info("Deleting admission ManagedResource for runtime cluster if present", "managedResource", client.ObjectKey{Name: managedResourceName, Namespace: r.GardenNamespace})
	if err := managedresources.DeleteForSeed(ctx, r.RuntimeClientSet.Client(), r.GardenNamespace, managedResourceName); err != nil {
		return fmt.Errorf("failed deleting ManagedResource: %w", err)
	}

	if err := managedresources.WaitUntilDeleted(ctx, r.RuntimeClientSet.Client(), r.GardenNamespace, runtimeClusterAdmissionManagedResourceName(extension)); err != nil {
		return fmt.Errorf("failed waiting for ManagedResource to be deleted: %w", err)
	}

	accessSecret := r.getVirtualClusterAccessSecret(extension).Secret

	log.Info("Deleting admission access secret for virtual cluster", "secret", client.ObjectKeyFromObject(accessSecret))
	return kubernetesutils.DeleteObjects(ctx, r.RuntimeClientSet.Client(), accessSecret)
}
