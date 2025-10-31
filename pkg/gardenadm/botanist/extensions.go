// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenadm"
	"github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/controllerinstallation"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/oci"
)

// ComputeExtensions takes a list of ControllerRegistrations and ControllerDeployments and computes a corresponding list
// of Extensions.
func ComputeExtensions(resources gardenadm.Resources, runsControlPlane, managedInfrastructure bool) ([]Extension, error) {
	var extensions []Extension

	wantedControllerRegistrationNames, err := computeWantedControllerRegistrationNames(
		resources,
		wantedExtensionKinds(runsControlPlane, managedInfrastructure),
	)
	if err != nil {
		return nil, fmt.Errorf("failed computing the names of the wanted ControllerRegistrations: %w", err)
	}

	for _, controllerRegistration := range resources.ControllerRegistrations {
		if !wantedControllerRegistrationNames.Has(controllerRegistration.Name) {
			continue
		}

		if controllerRegistration.Spec.Deployment == nil || len(controllerRegistration.Spec.Deployment.DeploymentRefs) != 1 {
			return nil, fmt.Errorf("ControllerRegistration %s has invalid deployment refs in its spec (must reference exactly one ControllerDeployment)", controllerRegistration.Name)
		}

		idx := slices.IndexFunc(resources.ControllerDeployments, func(controllerDeployment *gardencorev1.ControllerDeployment) bool {
			return controllerDeployment.Name == controllerRegistration.Spec.Deployment.DeploymentRefs[0].Name
		})
		if idx == -1 {
			return nil, fmt.Errorf("ControllerDeployment %s referenced in ControllerRegistration %s was not found", controllerRegistration.Spec.Deployment.DeploymentRefs[0].Name, controllerRegistration.Name)
		}

		var (
			controllerDeployment   = resources.ControllerDeployments[idx].DeepCopy()
			controllerInstallation = &gardencorev1beta1.ControllerInstallation{
				ObjectMeta: metav1.ObjectMeta{Name: controllerRegistration.Name},
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					RegistrationRef: corev1.ObjectReference{Name: controllerRegistration.Name},
					DeploymentRef:   &corev1.ObjectReference{Name: controllerDeployment.Name},
					SeedRef:         corev1.ObjectReference{Name: resources.Shoot.Name},
				},
			}
		)

		// Remove the InjectGardenKubeconfig field from the ControllerDeployment because we don't have any information
		// about a potentially existing garden cluster.
		controllerDeployment.InjectGardenKubeconfig = nil

		extensions = append(extensions, Extension{
			ControllerRegistration: controllerRegistration,
			ControllerDeployment:   controllerDeployment,
			ControllerInstallation: controllerInstallation,
		})
	}

	return extensions, nil
}

// wantedExtensionKinds returns the set of extension kinds that are needed and supported for self-hosted shoot clusters.
// runsControlPlane indicates whether we are bootstrapping the control plane of the cluster (i.e., when executing
// `gardenadm init`).
func wantedExtensionKinds(runsControlPlane, managedInfrastructure bool) sets.Set[string] {
	if !runsControlPlane {
		// When running `gardenadm bootstrap` against the bootstrap cluster, we create Infrastructure, OSC, Worker, and
		// DNSRecord for the control plane of the self-hosted shoot cluster, so we only need to deploy a subset of the
		// extensions required for the shoot.
		return sets.New[string](extensionsv1alpha1.InfrastructureResource, extensionsv1alpha1.OperatingSystemConfigResource, extensionsv1alpha1.WorkerResource, extensionsv1alpha1.DNSRecordResource)
	}

	// In the "unmanaged infrastructure" scenario, we don't deploy Infrastructure, Worker, and DNSRecord extensions
	// because they are managed outside of Gardener.
	if !managedInfrastructure {
		return extensionsv1alpha1.AllExtensionKinds.Clone().Delete(extensionsv1alpha1.InfrastructureResource, extensionsv1alpha1.WorkerResource, extensionsv1alpha1.DNSRecordResource)
	}

	// In `gardenadm init`, we deploy all extensions referenced by the shoot in the "managed infrastructure" scenario.
	return extensionsv1alpha1.AllExtensionKinds.Clone()
}

// computeWantedControllerRegistrationNames returns the names of all ControllerRegistrations relevant for the
// gardenadm botanist based on the parsed manifests and the wanted extension kinds.
func computeWantedControllerRegistrationNames(resources gardenadm.Resources, wantedExtensionKinds sets.Set[string]) (sets.Set[string], error) {
	var (
		result                                   = sets.New[string]()
		requiredExtensionIDs                     = sets.New[string]()
		extensionIDToControllerRegistrationNames = make(map[string][]string)
	)

	// collect available extension IDs from ControllerRegistrations, and add always-deployed extensions to result
	for _, controllerRegistration := range resources.ControllerRegistrations {
		for _, resource := range controllerRegistration.Spec.Resources {
			id := gardenerutils.ExtensionsID(resource.Kind, resource.Type)
			extensionIDToControllerRegistrationNames[id] = append(extensionIDToControllerRegistrationNames[id], controllerRegistration.Name)
		}

		if controllerRegistration.Spec.Deployment != nil && ptr.Deref(controllerRegistration.Spec.Deployment.Policy, "") == gardencorev1beta1.ControllerDeploymentPolicyAlways {
			result.Insert(controllerRegistration.Name)
		}
	}

	// collect extension IDs required by the shoot
	for _, extensionID := range gardenerutils.ComputeRequiredExtensionsForShoot(resources.Shoot, nil, controllerRegistrationSliceToList(resources.ControllerRegistrations), nil, nil).UnsortedList() {
		extensionKind, _, err := gardenerutils.ExtensionKindAndTypeForID(extensionID)
		if err != nil {
			return nil, err
		}

		if wantedExtensionKinds.Has(extensionKind) {
			requiredExtensionIDs.Insert(extensionID)
		}
	}

	// map required extension IDs back to ControllerRegistration names
	for extensionID := range requiredExtensionIDs {
		names, ok := extensionIDToControllerRegistrationNames[extensionID]
		if !ok {
			return nil, fmt.Errorf("need to install an extension controller for %q but no appropriate ControllerRegistration found", extensionID)
		}
		result.Insert(names...)
	}

	return result, nil
}

func controllerRegistrationSliceToList(controllerRegistrations []*gardencorev1beta1.ControllerRegistration) *gardencorev1beta1.ControllerRegistrationList {
	list := &gardencorev1beta1.ControllerRegistrationList{}
	for _, controllerRegistration := range controllerRegistrations {
		if controllerRegistration != nil {
			list.Items = append(list.Items, *controllerRegistration)
		}
	}
	return list
}

// ReconcileExtensionControllerInstallations reconciles the ControllerInstallation resources necessary to deploy the
// extension controllers.
func (b *GardenadmBotanist) ReconcileExtensionControllerInstallations(ctx context.Context, bootstrapMode bool) error {
	reconciler := controllerinstallation.Reconciler{
		GardenClient:              b.GardenClient,
		SeedClientSet:             b.SeedClientSet,
		HelmRegistry:              oci.NewHelmRegistry(b.SeedClientSet.Client()),
		Clock:                     b.Clock,
		Identity:                  &b.Shoot.GetInfo().Status.Gardener,
		GardenNamespace:           b.Shoot.ControlPlaneNamespace,
		BootstrapControlPlaneNode: bootstrapMode,
	}

	for _, extension := range b.Extensions {
		b.Logger.Info("Reconciling ControllerInstallation using gardenlet's reconciliation logic", "controllerInstallationName", extension.ControllerInstallation.Name)

		reconcilerCtx := log.IntoContext(ctx, b.Logger.WithName("controllerinstallation-reconciler").WithValues("controllerInstallationName", extension.ControllerInstallation.Name))
		if _, err := reconciler.Reconcile(reconcilerCtx, reconcile.Request{NamespacedName: types.NamespacedName{Name: extension.ControllerInstallation.Name}}); err != nil {
			return fmt.Errorf("failed running ControllerInstallation controller for %q: %w", extension.ControllerInstallation.Name, err)
		}
	}

	return nil
}

// TimeoutManagedResourceHealthCheck is the timeout for the health check of the managed resources.
// Exposed for testing.
var TimeoutManagedResourceHealthCheck = 2 * time.Minute

// WaitUntilExtensionControllerInstallationsHealthy waits until all ControllerInstallation resources used for
// extension controller deployments are healthy.
func (b *GardenadmBotanist) WaitUntilExtensionControllerInstallationsHealthy(ctx context.Context) error {
	var taskFns []flow.TaskFn

	for _, extension := range b.Extensions {
		taskFns = append(taskFns, func(ctx context.Context) error {
			return managedresources.WaitUntilHealthyAndNotProgressing(ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, extension.ControllerInstallation.Name)
		})
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutManagedResourceHealthCheck)
	defer cancel()

	return flow.Parallel(taskFns...)(timeoutCtx)
}
