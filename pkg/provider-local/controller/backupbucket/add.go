// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package backupbucket

import (
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	"github.com/gardener/gardener/pkg/provider-local/controller/backupoptions"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

// ControllerName is the name of the controller.
const ControllerName = "backupbucket_controller"

var (
	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = backupoptions.AddOptions{}
)

// AddOptions are options to apply when adding the backupbucket controller to the manager.
type AddOptions struct {
	// BackupBucketLocalDir is the directory of the backupbucket.
	BackupBucketLocalDir string
	// Controller are the controller.Options.
	Controller controller.Options
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
}

// AddToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func AddToManagerWithOptions(mgr manager.Manager, opts backupoptions.AddOptions) error {
	return backupbucket.Add(mgr, backupbucket.AddArgs{
		Actuator:          newActuator(opts.BackupBucketPath),
		ControllerOptions: opts.Controller,
		Predicates:        backupbucket.DefaultPredicates(opts.IgnoreOperationAnnotation),
		Type:              local.Type,
	})
}

// AddToManager adds a controller with the default Options.
func AddToManager(mgr manager.Manager) error {
	return AddToManagerWithOptions(mgr, DefaultAddOptions)
}
