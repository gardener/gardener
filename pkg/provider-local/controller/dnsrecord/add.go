// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord

import (
	"context"
	"slices"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

var (
	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{}

	// supportedExtensionClasses are the extension classes supported by the dnsrecord controller.
	supportedExtensionClasses = sets.New(
		extensionsv1alpha1.ExtensionClassGarden,
		extensionsv1alpha1.ExtensionClassSeed,
		extensionsv1alpha1.ExtensionClassShoot,
	)
)

// AddOptions are options to apply when adding the local dnsrecord controller to the manager.
type AddOptions struct {
	// Controller are the controller.Options.
	Controller controller.Options
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
	// ExtensionClasses are the configured extension classes for this extension deployment.
	ExtensionClasses []extensionsv1alpha1.ExtensionClass
}

// AddToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated Actuator.
func AddToManagerWithOptions(ctx context.Context, mgr manager.Manager, opts AddOptions) error {
	classes := slices.DeleteFunc(opts.ExtensionClasses, func(class extensionsv1alpha1.ExtensionClass) bool {
		return !supportedExtensionClasses.Has(class)
	})

	return dnsrecord.Add(mgr, dnsrecord.AddArgs{
		Actuator: &Actuator{
			RuntimeClient: mgr.GetClient(),
		},
		ControllerOptions: opts.Controller,
		Predicates:        dnsrecord.DefaultPredicates(ctx, mgr, opts.IgnoreOperationAnnotation),
		Type:              local.Type,
		ExtensionClasses:  classes,
	})
}

// AddToManager adds a controller with the default Options.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(ctx, mgr, DefaultAddOptions)
}
