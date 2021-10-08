// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package garbagecollector

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/client-go/util/workqueue"
	crcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	resourcemanagercmd "github.com/gardener/gardener/pkg/resourcemanager/cmd"
)

// ControllerName is the name of the controller.
const ControllerName = "garbage_collector"

// defaultControllerConfig is the default config for the controller.
var defaultControllerConfig ControllerConfig

// ControllerOptions are options for adding the controller to a Manager.
type ControllerOptions struct {
	syncPeriod time.Duration
}

// ControllerConfig is the completed configuration for the controller.
type ControllerConfig struct {
	SyncPeriod         time.Duration
	TargetClientConfig resourcemanagercmd.TargetClientConfig
}

// AddToManagerWithOptions adds the controller to a Manager with the given config.
func AddToManagerWithOptions(mgr manager.Manager, conf ControllerConfig) error {
	if conf.SyncPeriod <= 0 {
		return nil
	}

	ctrl, err := crcontroller.New(ControllerName, mgr, crcontroller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler: &reconciler{
			syncPeriod:   conf.SyncPeriod,
			targetClient: conf.TargetClientConfig.Client,
		},
	})
	if err != nil {
		return fmt.Errorf("unable to set up gc controller: %w", err)
	}

	eventChannel := make(chan event.GenericEvent, 1)
	eventChannel <- event.GenericEvent{}

	return ctrl.Watch(
		&source.Channel{Source: eventChannel},
		&handler.Funcs{GenericFunc: func(_ event.GenericEvent, q workqueue.RateLimitingInterface) { q.Add(reconcile.Request{}) }},
	)
}

// AddToManager adds the controller to a Manager using the default config.
func AddToManager(mgr manager.Manager) error {
	return AddToManagerWithOptions(mgr, defaultControllerConfig)
}

// AddFlags adds the needed command line flags to the given FlagSet.
func (o *ControllerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.DurationVar(&o.syncPeriod, "garbage-collector-sync-period", 0, "duration how often the garbage collection should be performed (default: 0, i.e., gc is disabled)")
}

// Complete completes the given command line flags and set the defaultControllerConfig accordingly.
func (o *ControllerOptions) Complete() error {
	defaultControllerConfig = ControllerConfig{
		SyncPeriod: o.syncPeriod,
	}
	return nil
}

// Completed returns the completed ControllerConfig.
func (o *ControllerOptions) Completed() *ControllerConfig {
	return &defaultControllerConfig
}
