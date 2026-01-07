// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"
	"slices"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/provider-local/controller/backupoptions"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

// ControllerName is the name of the controller.
const ControllerName = "backupbucket"

var (
	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = backupoptions.AddOptions{}

	// supportedExtensionClasses are the extension classes supported by the backupbucket controller.
	supportedExtensionClasses = sets.New(extensionsv1alpha1.ExtensionClassGarden, extensionsv1alpha1.ExtensionClassShoot)
)

// AddOptions are options to apply when adding the backupbucket controller to the manager.
type AddOptions struct {
	// BackupBucketLocalDir is the directory of the backupbucket.
	BackupBucketLocalDir string
	// Controller are the controller.Options.
	Controller controller.Options
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
	// ExtensionClasses are the configured extension classes for this extension deployment.
	ExtensionClasses extensionsv1alpha1.ExtensionClass
}

// AddToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func AddToManagerWithOptions(_ context.Context, mgr manager.Manager, opts backupoptions.AddOptions) error {
	classes := slices.DeleteFunc(opts.ExtensionClasses, func(class extensionsv1alpha1.ExtensionClass) bool {
		return !supportedExtensionClasses.Has(class)
	})

	return backupbucket.Add(mgr, backupbucket.AddArgs{
		Actuator:          newActuator(mgr, opts.BackupBucketPath),
		ControllerOptions: opts.Controller,
		Predicates:        backupbucket.DefaultPredicates(opts.IgnoreOperationAnnotation),
		Type:              local.Type,
		ExtensionClasses:  classes,
	})
}

// AddToManager adds a controller with the default Options.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(ctx, mgr, DefaultAddOptions)
}
