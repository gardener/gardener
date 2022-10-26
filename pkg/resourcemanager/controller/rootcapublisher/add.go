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

// ControllerName is the name of the controller.
const ControllerName = "root-ca-publisher"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, targetCluster cluster.Cluster) error {
	if r.TargetClient == nil {
		r.TargetClient = targetCluster.GetClient()
	}

	if r.RootCA == "" && r.Config.RootCAFile != nil {
		rootCA, err := os.ReadFile(*r.Config.RootCAFile)
		if err != nil {
			return fmt.Errorf("file for root ca could not be read: %w", err)
		}
		r.RootCA = string(rootCA)
	}

	// It's not possible to overwrite the event handler when using the controller builder. Hence, we have to build up
	// the controller manually.
	c, err := controller.New(
		ControllerName,
		mgr,
		controller.Options{
			Reconciler:              r,
			MaxConcurrentReconciles: 1,
			RecoverPanic:            true,
		},
	)
	if err != nil {
		return err
	}

	if err := c.Watch(
		source.NewKindWithCache(&corev1.Namespace{}, targetCluster.GetCache()),
		&handler.EnqueueRequestForObject{},
		r.NamespacePredicate(),
	); err != nil {
		return fmt.Errorf("unable to watch Namespaces: %w", err)
	}

	configMap := &metav1.PartialObjectMetadata{}
	configMap.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	return c.Watch(
		source.NewKindWithCache(configMap, targetCluster.GetCache()),
		&handler.EnqueueRequestForOwner{OwnerType: &corev1.Namespace{}},
		r.ConfigMapPredicate(),
	)
}

// NamespacePredicate returns the predicate for Namespaces.
func (r *Reconciler) NamespacePredicate() predicate.Predicate {
	isNamespaceActive := func(obj client.Object) bool {
		namespace, ok := obj.(*corev1.Namespace)
		if !ok {
			return false
		}
		return namespace.Status.Phase == corev1.NamespaceActive
	}

	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		UpdateFunc:  func(e event.UpdateEvent) bool { return isNamespaceActive(e.ObjectNew) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
}

// ConfigMapPredicate returns the predicate for ConfigMaps.
func (r *Reconciler) ConfigMapPredicate() predicate.Predicate {
	isRelevantConfigMap := func(obj client.Object) bool {
		return obj.GetName() == RootCACertConfigMapName && obj.GetAnnotations()[DescriptionAnnotation] == ""
	}

	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return isRelevantConfigMap(e.ObjectNew) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return isRelevantConfigMap(e.Object) },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
}
