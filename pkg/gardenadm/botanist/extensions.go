// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
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
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/controllerinstallation"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/oci"
)

// ComputeExtensions takes a list of ControllerRegistrations and ControllerDeployments and computes a corresponding list
// of Extensions.
func ComputeExtensions(
	shoot *gardencorev1beta1.Shoot,
	controllerRegistrations []*gardencorev1beta1.ControllerRegistration,
	controllerDeployments []*gardencorev1.ControllerDeployment,
) (
	[]Extension,
	error,
) {
	var extensions []Extension

	wantedControllerRegistrationNames, err := computeWantedControllerRegistrationNames(shoot, controllerRegistrations)
	if err != nil {
		return nil, fmt.Errorf("failed computing the names of the wanted ControllerRegistrations: %w", err)
	}

	for _, controllerRegistration := range controllerRegistrations {
		if !wantedControllerRegistrationNames.Has(controllerRegistration.Name) {
			continue
		}

		if controllerRegistration.Spec.Deployment == nil || len(controllerRegistration.Spec.Deployment.DeploymentRefs) != 1 {
			return nil, fmt.Errorf("ControllerRegistration %s has invalid deployment refs in its spec (must reference exactly one ControllerDeployment)", controllerRegistration.Name)
		}

		idx := slices.IndexFunc(controllerDeployments, func(controllerDeployment *gardencorev1.ControllerDeployment) bool {
			return controllerDeployment.Name == controllerRegistration.Spec.Deployment.DeploymentRefs[0].Name
		})
		if idx == -1 {
			return nil, fmt.Errorf("ControllerDeployment %s referenced in ControllerRegistration %s was not found", controllerRegistration.Spec.Deployment.DeploymentRefs[0].Name, controllerRegistration.Name)
		}

		var (
			controllerDeployment   = controllerDeployments[idx].DeepCopy()
			controllerInstallation = &gardencorev1beta1.ControllerInstallation{
				ObjectMeta: metav1.ObjectMeta{Name: controllerRegistration.Name},
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					RegistrationRef: corev1.ObjectReference{Name: controllerRegistration.Name},
					DeploymentRef:   &corev1.ObjectReference{Name: controllerDeployment.Name},
					SeedRef:         corev1.ObjectReference{Name: shoot.Name},
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

func computeWantedControllerRegistrationNames(shoot *gardencorev1beta1.Shoot, controllerRegistrations []*gardencorev1beta1.ControllerRegistration) (sets.Set[string], error) {
	var (
		result                                   = sets.New[string]()
		extensionIDToControllerRegistrationNames = make(map[string][]string)
	)

	for _, controllerRegistration := range controllerRegistrations {
		for _, resource := range controllerRegistration.Spec.Resources {
			id := gardenerutils.ExtensionsID(resource.Kind, resource.Type)
			extensionIDToControllerRegistrationNames[id] = append(extensionIDToControllerRegistrationNames[id], controllerRegistration.Name)
		}

		if controllerRegistration.Spec.Deployment != nil && ptr.Deref(controllerRegistration.Spec.Deployment.Policy, "") == gardencorev1beta1.ControllerDeploymentPolicyAlways {
			result.Insert(controllerRegistration.Name)
		}
	}

	for _, extensionID := range gardenerutils.ComputeRequiredExtensionsForShoot(shoot, nil, controllerRegistrationSliceToList(controllerRegistrations), nil, nil).UnsortedList() {
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
func (b *AutonomousBotanist) ReconcileExtensionControllerInstallations(ctx context.Context, bootstrapMode bool) error {
	var (
		reconcilerCtx = log.IntoContext(ctx, b.Logger.WithName("controllerinstallation-reconciler"))
		reconciler    = controllerinstallation.Reconciler{
			GardenClient:              b.GardenClient,
			SeedClientSet:             b.SeedClientSet,
			HelmRegistry:              oci.NewHelmRegistry(b.SeedClientSet.Client()),
			Clock:                     clock.RealClock{},
			Identity:                  &b.Shoot.GetInfo().Status.Gardener,
			GardenNamespace:           b.Shoot.ControlPlaneNamespace,
			BootstrapControlPlaneNode: bootstrapMode,
		}
	)

	for _, extension := range b.Extensions {
		b.Logger.Info("Reconciling ControllerInstallation using gardenlet's reconciliation logic", "controllerInstallationName", extension.ControllerInstallation.Name)
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
func (b *AutonomousBotanist) WaitUntilExtensionControllerInstallationsHealthy(ctx context.Context) error {
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
