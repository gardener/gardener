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

package rootcapublisher

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ControllerName is the name of the root ca controller.
const ControllerName = "root-ca-publisher"

// defaultControllerConfig is the default config for the controller.
var defaultControllerConfig ControllerConfig

// ControllerOptions is the completed configuration for the controller.
type ControllerOptions struct {
	maxConcurrentWorkers int
	rootCAPath           string
}

// ControllerConfig is the completed configuration for the controller.
type ControllerConfig struct {
	MaxConcurrentWorkers int
	RootCAPath           string
	TargetCluster        cluster.Cluster
}

// AddToManagerWithOptions adds the controller to a Manager with the given config.
func AddToManagerWithOptions(mgr manager.Manager, conf ControllerConfig) error {
	if conf.RootCAPath == "" || conf.MaxConcurrentWorkers == 0 {
		return nil
	}

	rootCA, err := os.ReadFile(conf.RootCAPath)
	if err != nil {
		return fmt.Errorf("file for root ca could not be read: %w", err)
	}

	rootCAController, err := controller.New(ControllerName, mgr, controller.Options{
		MaxConcurrentReconciles: conf.MaxConcurrentWorkers,
		Reconciler: &reconciler{
			rootCA:       string(rootCA),
			targetClient: conf.TargetCluster.GetClient(),
		},
		RecoverPanic: true,
	})
	if err != nil {
		return fmt.Errorf("unable to set up root ca controller: %w", err)
	}

	if err := rootCAController.Watch(
		source.NewKindWithCache(&corev1.Namespace{}, conf.TargetCluster.GetCache()),
		&handler.EnqueueRequestForObject{},
		predicate.Funcs{
			CreateFunc:  func(e event.CreateEvent) bool { return true },
			UpdateFunc:  func(e event.UpdateEvent) bool { return isNamespaceActive(e.ObjectNew) },
			DeleteFunc:  func(e event.DeleteEvent) bool { return false },
			GenericFunc: func(e event.GenericEvent) bool { return false },
		},
	); err != nil {
		return fmt.Errorf("unable to watch Namespaces: %w", err)
	}

	configMap := &metav1.PartialObjectMetadata{}
	configMap.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	if err := rootCAController.Watch(
		source.NewKindWithCache(configMap, conf.TargetCluster.GetCache()),
		&handler.EnqueueRequestForOwner{OwnerType: &corev1.Namespace{}},
		predicate.Funcs{
			CreateFunc:  func(e event.CreateEvent) bool { return false },
			UpdateFunc:  func(e event.UpdateEvent) bool { return isRelevantConfigMap(e.ObjectNew) },
			DeleteFunc:  func(e event.DeleteEvent) bool { return isRelevantConfigMap(e.Object) },
			GenericFunc: func(e event.GenericEvent) bool { return false },
		},
	); err != nil {
		return fmt.Errorf("unable to watch ConfigMaps: %w", err)
	}

	return nil
}

// AddToManager adds the controller to a Manager using the default config.
func AddToManager(mgr manager.Manager) error {
	return AddToManagerWithOptions(mgr, defaultControllerConfig)
}

func isRelevantConfigMap(obj client.Object) bool {
	return obj.GetName() == RootCACertConfigMapName && obj.GetAnnotations()[DescriptionAnnotation] == ""
}

func isNamespaceActive(obj client.Object) bool {
	namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		return false
	}

	return namespace.Status.Phase == corev1.NamespaceActive
}

// AddFlags adds the needed command line flags to the given FlagSet.
func (o *ControllerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.rootCAPath, "root-ca-file", "", "path to a file containing the root ca bundle")
	fs.IntVar(&o.maxConcurrentWorkers, "root-ca-publisher-max-concurrent-workers", 0, "number of worker threads for concurrent rootcapublisher reconciliation of resources")
}

// Complete completes the given command line flags and set the defaultControllerConfig accordingly.
func (o *ControllerOptions) Complete() error {
	defaultControllerConfig = ControllerConfig{
		RootCAPath:           o.rootCAPath,
		MaxConcurrentWorkers: o.maxConcurrentWorkers,
	}
	return nil
}

// Completed returns the completed ControllerConfig.
func (o *ControllerOptions) Completed() *ControllerConfig {
	return &defaultControllerConfig
}
