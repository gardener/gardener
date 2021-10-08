// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package secret

import (
	"fmt"

	extensionshandler "github.com/gardener/gardener/extensions/pkg/handler"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/mapper"
	managerpredicate "github.com/gardener/gardener/pkg/resourcemanager/predicate"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ControllerName is the name of the secret controller.
const ControllerName = "secret-controller"

// defaultControllerConfig is the default config for the controller.
var defaultControllerConfig ControllerConfig

// ControllerOptions are options for adding the controller to a Manager.
type ControllerOptions struct {
	maxConcurrentWorkers int
}

// ControllerConfig is the completed configuration for the controller.
type ControllerConfig struct {
	MaxConcurrentWorkers int

	ClassFilter managerpredicate.ClassFilter
}

// AddToManagerWithOptions adds the controller to a Manager with the given config.
func AddToManagerWithOptions(mgr manager.Manager, conf ControllerConfig) error {
	secretController, err := controller.New(ControllerName, mgr, controller.Options{
		MaxConcurrentReconciles: conf.MaxConcurrentWorkers,
		Reconciler: &Reconciler{
			ClassFilter: &conf.ClassFilter,
		},
	})
	if err != nil {
		return fmt.Errorf("unable to set up secret controller: %w", err)
	}

	if err := secretController.Watch(
		&source.Kind{Type: &resourcesv1alpha1.ManagedResource{}},
		extensionshandler.EnqueueRequestsFromMapper(mapper.ManagedResourceToSecretsMapper(), extensionshandler.UpdateWithOldAndNew),
		predicate.GenerationChangedPredicate{},
	); err != nil {
		return fmt.Errorf("unable to watch ManagedResources: %w", err)
	}

	// Also watch secrets to ensure, that we properly remove the finalizer in case we missed an important
	// update event for a ManagedResource during downtime.
	if err := secretController.Watch(
		&source.Kind{Type: &corev1.Secret{}},
		&handler.EnqueueRequestForObject{},
		// Only requeue secrets from create/update events with the controller's finalizer to not flood the controller
		// with too many unnecessary requests for all secrets in cluster/namespace.
		managerpredicate.HasFinalizer(conf.ClassFilter.FinalizerName()),
	); err != nil {
		return fmt.Errorf("unable to watch Secrets: %w", err)
	}
	return nil
}

// AddToManager adds the controller to a Manager using the default config.
func AddToManager(mgr manager.Manager) error {
	return AddToManagerWithOptions(mgr, defaultControllerConfig)
}

// AddFlags adds the needed command line flags to the given FlagSet.
func (o *ControllerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.IntVar(&o.maxConcurrentWorkers, "secret-max-concurrent-workers", 5, "number of worker threads for concurrent secret reconciliation of resources")
}

// Complete completes the given command line flags and set the defaultControllerConfig accordingly.
func (o *ControllerOptions) Complete() error {
	defaultControllerConfig = ControllerConfig{
		MaxConcurrentWorkers: o.maxConcurrentWorkers,
	}
	return nil
}

// Completed returns the completed ControllerConfig.
func (o *ControllerOptions) Completed() *ControllerConfig {
	return &defaultControllerConfig
}
