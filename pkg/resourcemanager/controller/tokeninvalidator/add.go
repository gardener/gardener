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

package tokeninvalidator

import (
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	resourcemanagercmd "github.com/gardener/gardener/pkg/resourcemanager/cmd"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ControllerName is the name of the controller.
const ControllerName = "token-invalidator-controller"

// defaultControllerConfig is the default config for the controller.
var defaultControllerConfig ControllerConfig

// ControllerOptions are options for adding the controller to a Manager.
type ControllerOptions struct {
	maxConcurrentWorkers int
}

// ControllerConfig is the completed configuration for the controller.
type ControllerConfig struct {
	MaxConcurrentWorkers int
	TargetCache          cache.Cache
	TargetClusterConfig  resourcemanagercmd.TargetClusterConfig
}

// AddToManagerWithOptions adds the controller to a Manager with the given config.
func AddToManagerWithOptions(mgr manager.Manager, conf ControllerConfig) error {
	if conf.MaxConcurrentWorkers == 0 {
		return nil
	}

	c, err := crcontroller.New(ControllerName, mgr,
		crcontroller.Options{
			MaxConcurrentReconciles: conf.MaxConcurrentWorkers,
			Reconciler:              NewReconciler(mgr.GetClient()),
		},
	)
	if err != nil {
		return err
	}

	secret := &metav1.PartialObjectMetadata{}
	secret.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))

	if err := c.Watch(
		source.NewKindWithCache(secret, conf.TargetCache),
		&handler.EnqueueRequestForObject{},
		predicate.Funcs{
			CreateFunc:  func(e event.CreateEvent) bool { return isRelevantSecret(e.Object) },
			UpdateFunc:  func(e event.UpdateEvent) bool { return isRelevantSecret(e.ObjectNew) },
			DeleteFunc:  func(e event.DeleteEvent) bool { return false },
			GenericFunc: func(e event.GenericEvent) bool { return false },
		},
	); err != nil {
		return err
	}

	return c.Watch(
		source.NewKindWithCache(&corev1.ServiceAccount{}, conf.TargetCache),
		handler.EnqueueRequestsFromMapFunc(mapServiceAccountToSecrets),
		predicate.Funcs{
			CreateFunc:  func(e event.CreateEvent) bool { return false },
			UpdateFunc:  func(e event.UpdateEvent) bool { return isRelevantServiceAccountUpdate(e) },
			DeleteFunc:  func(e event.DeleteEvent) bool { return false },
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
	fs.IntVar(&o.maxConcurrentWorkers, "token-invalidator-max-concurrent-workers", 0, "number of worker threads for concurrent token invalidation reconciliations")
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
	metadata, ok := obj.(*metav1.PartialObjectMetadata)
	if !ok {
		return false
	}

	return metav1.HasAnnotation(metadata.ObjectMeta, corev1.ServiceAccountNameKey)
}

func isRelevantServiceAccountUpdate(e event.UpdateEvent) bool {
	oldSA, ok := e.ObjectOld.(*corev1.ServiceAccount)
	if !ok {
		return false
	}

	newSA, ok := e.ObjectNew.(*corev1.ServiceAccount)
	if !ok {
		return false
	}

	return !apiequality.Semantic.DeepEqual(oldSA.AutomountServiceAccountToken, newSA.AutomountServiceAccountToken) ||
		oldSA.Labels[resourcesv1alpha1.StaticTokenSkip] != newSA.Labels[resourcesv1alpha1.StaticTokenSkip]
}

func mapServiceAccountToSecrets(obj client.Object) []reconcile.Request {
	sa, ok := obj.(*corev1.ServiceAccount)
	if !ok {
		return nil
	}

	out := make([]reconcile.Request, 0, len(sa.Secrets))

	for _, secretRef := range sa.Secrets {
		out = append(out, reconcile.Request{NamespacedName: types.NamespacedName{
			Name:      secretRef.Name,
			Namespace: sa.Namespace,
		}})
	}

	return out
}
