// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

var (
	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{}
)

// AddOptions are options to apply when adding the local worker controller to the manager.
type AddOptions struct {
	// GardenCluster is the garden cluster object.
	GardenCluster cluster.Cluster
	// Controller are the controller.Options.
	Controller controller.Options
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
	// ExtensionClass defines the extension class this extension is responsible for.
	ExtensionClass extensionsv1alpha1.ExtensionClass
	// AutonomousShootCluster indicates whether the extension runs in an autonomous shoot cluster.
	AutonomousShootCluster bool
}

// AddToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func AddToManagerWithOptions(ctx context.Context, mgr manager.Manager, opts AddOptions) error {
	scheme := mgr.GetScheme()
	if err := apiextensionsscheme.AddToScheme(scheme); err != nil {
		return err
	}
	if err := machinev1alpha1.AddToScheme(scheme); err != nil {
		return err
	}

	return worker.Add(ctx, mgr, worker.AddArgs{
		Actuator:               NewActuator(mgr, opts.GardenCluster),
		ControllerOptions:      opts.Controller,
		Predicates:             worker.DefaultPredicates(ctx, mgr, opts.IgnoreOperationAnnotation),
		Type:                   local.Type,
		ExtensionClass:         opts.ExtensionClass,
		AutonomousShootCluster: opts.AutonomousShootCluster,
	})
}

// AddToManager adds a controller with the default Options.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(ctx, mgr, DefaultAddOptions)
}
