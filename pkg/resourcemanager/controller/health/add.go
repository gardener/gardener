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

package health

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	resourcemanagercmd "github.com/gardener/gardener/pkg/resourcemanager/cmd"
	managerpredicate "github.com/gardener/gardener/pkg/resourcemanager/predicate"
)

// ControllerName is the name of the health controller.
const ControllerName = "health-controller"

// defaultControllerConfig is the default config for the controller.
var defaultControllerConfig ControllerConfig

// ControllerOptions are options for adding the controller to a Manager.
type ControllerOptions struct {
	maxConcurrentWorkers int
	syncPeriod           time.Duration
}

// ControllerConfig is the completed configuration for the controller.
type ControllerConfig struct {
	MaxConcurrentWorkers int
	SyncPeriod           time.Duration

	ClassFilter        managerpredicate.ClassFilter
	TargetClientConfig resourcemanagercmd.TargetClientConfig
}

// AddToManagerWithOptions adds the controller to a Manager with the given config.
func AddToManagerWithOptions(mgr manager.Manager, conf ControllerConfig) error {
	healthController, err := controller.New("health-controller", mgr, controller.Options{
		MaxConcurrentReconciles: conf.MaxConcurrentWorkers,
		Reconciler: &reconciler{
			syncPeriod:   conf.SyncPeriod,
			classFilter:  &conf.ClassFilter,
			targetClient: conf.TargetClientConfig.Client,
			targetScheme: conf.TargetClientConfig.Scheme,
		},
	})
	if err != nil {
		return fmt.Errorf("unable to set up individual controller: %w", err)
	}

	if err := healthController.Watch(
		&source.Kind{Type: &resourcesv1alpha1.ManagedResource{}},
		&handler.Funcs{
			CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
				q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
					Name:      e.Object.GetName(),
					Namespace: e.Object.GetNamespace(),
				}})
			},
			UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
				q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
					Name:      e.ObjectNew.GetName(),
					Namespace: e.ObjectNew.GetNamespace(),
				}})
			},
		},
		&conf.ClassFilter, predicate.Or(
			managerpredicate.ClassChangedPredicate(),
			// start health checks immediately after MR has been reconciled
			managerpredicate.ConditionStatusChanged(resourcesv1alpha1.ResourcesApplied, managerpredicate.DefaultConditionChange),
		),
	); err != nil {
		return fmt.Errorf("unable to watch ManagedResources: %w", err)
	}
	return nil
}

// AddToManager adds the controller to a Manager using the default config.
func AddToManager(mgr manager.Manager) error {
	return AddToManagerWithOptions(mgr, defaultControllerConfig)
}

// AddFlags adds the needed command line flags to the given FlagSet.
func (o *ControllerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.DurationVar(&o.syncPeriod, "health-sync-period", time.Minute, "duration how often the health of existing resources should be synced")
	fs.IntVar(&o.maxConcurrentWorkers, "health-max-concurrent-workers", 10, "number of worker threads for concurrent health reconciliation of resources")
}

// Complete completes the given command line flags and set the defaultControllerConfig accordingly.
func (o *ControllerOptions) Complete() error {
	defaultControllerConfig = ControllerConfig{
		MaxConcurrentWorkers: o.maxConcurrentWorkers,
		SyncPeriod:           o.syncPeriod,
	}
	return nil
}

// Completed returns the completed ControllerConfig.
func (o *ControllerOptions) Completed() *ControllerConfig {
	return &defaultControllerConfig
}
