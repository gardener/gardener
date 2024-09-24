// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/oci"
)

// Interface contains functions for the extension deployer in the garden runtime cluster.
type Interface interface {
	// Reconcile creates or updates the extension deployment in the garden runtime cluster.
	Reconcile(context.Context, logr.Logger, *operatorv1alpha1.Extension) error
	// Delete deletes the extension deployment in the garden runtime cluster.
	Delete(context.Context, logr.Logger, *operatorv1alpha1.Extension) error
}

type deployer struct {
	runtimeClientSet kubernetes.Interface
	recorder         record.EventRecorder

	gardenNamespace string
	helmRegistry    oci.Interface
}

// Reconcile creates or updates the extension deployment in the garden runtime cluster.
// If the extension doesn't define an extension deployment for the runtime cluster, the deployment is deleted.
func (d *deployer) Reconcile(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	if !extensionDeploymentSpecified(extension) {
		return d.Delete(ctx, log, extension)
	}

	if err := d.createOrUpdateResources(ctx, extension); err != nil {
		return err
	}
	d.recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "Extension applied successfully in runtime cluster")
	return nil
}

// Delete deletes the extension deployment in the garden runtime cluster.
func (d *deployer) Delete(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	log.Info("Deleting extension resources in garden runtime cluster")
	if err := d.deleteResources(ctx, log, extension); err != nil {
		return err
	}

	d.recorder.Event(extension, corev1.EventTypeNormal, "Deletion", "Extension deployment deleted successfully in runtime cluster")
	return nil
}

func (d *deployer) createOrUpdateResources(ctx context.Context, extension *operatorv1alpha1.Extension) error {
	archive, err := d.helmRegistry.Pull(ctx, extension.Spec.Deployment.ExtensionDeployment.Helm.OCIRepository)
	if err != nil {
		return fmt.Errorf("failed pulling Helm chart from OCI repository %q: %w", extension.Spec.Deployment.ExtensionDeployment.Helm.OCIRepository.GetURL(), err)
	}

	gardenerValues := map[string]any{
		"gardener": map[string]any{
			"runtimeCluster": map[string]any{
				"enabled":           "true",
				"priorityClassName": v1beta1constants.PriorityClassNameGardenSystem200,
			},
		},
	}

	var helmValues map[string]any
	if extension.Spec.Deployment.ExtensionDeployment.RuntimeClusterValues != nil {
		if err := json.Unmarshal(extension.Spec.Deployment.ExtensionDeployment.RuntimeClusterValues.Raw, &helmValues); err != nil {
			return err
		}
	}

	renderedChart, err := d.runtimeClientSet.ChartRenderer().RenderArchive(archive, extension.Name, d.gardenNamespace, utils.MergeMaps(helmValues, gardenerValues))
	if err != nil {
		return fmt.Errorf("failed rendering Helm chart %q: %w", extension.Spec.Deployment.ExtensionDeployment.Helm.OCIRepository.GetURL(), err)
	}

	mrName := managedResourceName(extension)
	if err := managedresources.CreateForSeed(ctx, d.runtimeClientSet.Client(), d.gardenNamespace, mrName, false, renderedChart.AsSecretData()); err != nil {
		return fmt.Errorf("failed creating ManagedResource: %w", err)
	}

	if err := managedresources.WaitUntilHealthyAndNotProgressing(ctx, d.runtimeClientSet.Client(), d.gardenNamespace, mrName); err != nil {
		return fmt.Errorf("failed waiting for ManagedResource to be healthy: %w", err)
	}
	return nil
}

func (d *deployer) deleteResources(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	mrName := managedResourceName(extension)

	log.Info("Deleting extension ManagedResource for runtime cluster if present", "managedResource", client.ObjectKey{Name: mrName, Namespace: d.gardenNamespace})
	if err := managedresources.DeleteForSeed(ctx, d.runtimeClientSet.Client(), d.gardenNamespace, mrName); err != nil {
		return fmt.Errorf("failed deleting ManagedResource: %w", err)
	}

	if err := managedresources.WaitUntilDeleted(ctx, d.runtimeClientSet.Client(), d.gardenNamespace, mrName); err != nil {
		return fmt.Errorf("failed waiting for ManagedResource to be deleted: %w", err)
	}
	return nil
}

func managedResourceName(extension *operatorv1alpha1.Extension) string {
	return fmt.Sprintf("extension-%s-garden", extension.Name)
}

func extensionDeploymentSpecified(extension *operatorv1alpha1.Extension) bool {
	return extension.Spec.Deployment != nil &&
		extension.Spec.Deployment.ExtensionDeployment != nil &&
		extension.Spec.Deployment.ExtensionDeployment.Helm != nil &&
		extension.Spec.Deployment.ExtensionDeployment.RuntimeClusterValues != nil
}

// New creates a new runtime deployer.
func New(runtimeClientSet kubernetes.Interface, recorder record.EventRecorder, gardenNamespace string, registry oci.Interface) Interface {
	return &deployer{
		runtimeClientSet: runtimeClientSet,
		recorder:         recorder,
		gardenNamespace:  gardenNamespace,
		helmRegistry:     registry,
	}
}
