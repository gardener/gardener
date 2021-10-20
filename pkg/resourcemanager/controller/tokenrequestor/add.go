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

package tokenrequestor

import (
	"fmt"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/clock"
	corev1clientset "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	crcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ControllerName is the name of the controller.
const ControllerName = "tokenrequestor-controller"

// defaultControllerConfig is the default config for the controller.
var defaultControllerConfig ControllerConfig

// ControllerOptions are options for adding the controller to a Manager.
type ControllerOptions struct {
	maxConcurrentWorkers int
}

// ControllerConfig is the completed configuration for the controller.
type ControllerConfig struct {
	MaxConcurrentWorkers int
	TargetCluster        cluster.Cluster
}

// AddToManagerWithOptions adds the controller to a Manager with the given config.
func AddToManagerWithOptions(mgr manager.Manager, conf ControllerConfig) error {
	if conf.MaxConcurrentWorkers == 0 {
		return nil
	}

	coreV1Client, err := corev1clientset.NewForConfig(conf.TargetCluster.GetConfig())
	if err != nil {
		return fmt.Errorf("could not create coreV1Client: %w", err)
	}

	ctrl, err := crcontroller.New(ControllerName, mgr, crcontroller.Options{
		MaxConcurrentReconciles: conf.MaxConcurrentWorkers,
		Reconciler: &reconciler{
			clock:              clock.RealClock{},
			targetClient:       conf.TargetCluster.GetClient(),
			targetCoreV1Client: coreV1Client,
		},
	})
	if err != nil {
		return fmt.Errorf("unable to set up tokenRequestor controller: %w", err)
	}

	return ctrl.Watch(
		&source.Kind{Type: &corev1.Secret{}},
		&handler.EnqueueRequestForObject{},
		predicate.Funcs{
			CreateFunc:  func(e event.CreateEvent) bool { return isRelevantSecret(e.Object) },
			UpdateFunc:  func(e event.UpdateEvent) bool { return isRelevantSecret(e.ObjectNew) },
			DeleteFunc:  func(e event.DeleteEvent) bool { return isRelevantSecret(e.Object) },
			GenericFunc: func(e event.GenericEvent) bool { return false },
		},
	)
}

// AddToManager adds the controller to a Manager using the default config.
func AddToManager(mgr manager.Manager) error {
	return AddToManagerWithOptions(mgr, defaultControllerConfig)
}

// AddFlags adds the needed command line flags to the given FlagSet.
func (o *ControllerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.IntVar(&o.maxConcurrentWorkers, "tokenrequestor-max-concurrent-workers", 0, "number of worker threads for concurrent tokenrequestor reconciliations (default: 0)")
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

func isRelevantSecret(obj client.Object) bool {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return false
	}
	return secret.Labels != nil && secret.Labels[resourcesv1alpha1.ResourceManagerPurpose] == resourcesv1alpha1.LabelPurposeTokenRequest
}
