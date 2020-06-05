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

package controller

import (
	"context"
	"time"

	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener-resource-manager/pkg/manager"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// CreateManagedResourceFromUnstructured creates a managed resource and its secret with the given name, class, and objects in the given namespace.
func CreateManagedResourceFromUnstructured(ctx context.Context, client client.Client, namespace, name, class string, objs []*unstructured.Unstructured, keepObjects bool, injectedLabels map[string]string) error {
	var data []byte
	for _, obj := range objs {
		bytes, err := obj.MarshalJSON()
		if err != nil {
			return errors.Wrapf(err, "marshal failed for '%s/%s' for secret '%s/%s'", obj.GetNamespace(), obj.GetName(), namespace, name)
		}
		data = append(data, []byte("\n---\n")...)
		data = append(data, bytes...)
	}
	return CreateManagedResource(ctx, client, namespace, name, class, name, data, keepObjects, injectedLabels, false)
}

// CreateManagedResource creates a managed resource and its secret with the given name, class, key, and data in the given namespace.
func CreateManagedResource(ctx context.Context, client client.Client, namespace, name, class, key string, data []byte, keepObjects bool, injectedLabels map[string]string, forceOverwriteAnnotations bool) error {
	if key == "" {
		key = name
	}

	// Create or update secret containing the rendered rbac manifests
	if err := manager.NewSecret(client).
		WithNamespacedName(namespace, name).
		WithKeyValues(map[string][]byte{key: data}).
		Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "could not create or update secret '%s/%s' of managed resources", namespace, name)
	}

	if err := manager.NewManagedResource(client).
		WithNamespacedName(namespace, name).
		WithClass(class).
		WithInjectedLabels(injectedLabels).
		KeepObjects(keepObjects).
		WithSecretRef(name).
		ForceOverwriteAnnotations(forceOverwriteAnnotations).
		Reconcile(ctx); err != nil {
		return errors.Wrapf(err, "could not create or update managed resource '%s/%s'", namespace, name)
	}

	return nil
}

// DeleteManagedResource deletes the managed resource and its secret with the given name in the given namespace.
func DeleteManagedResource(ctx context.Context, client client.Client, namespace string, name string) error {
	if err := manager.
		NewManagedResource(client).
		WithNamespacedName(namespace, name).
		Delete(ctx); err != nil {
		return errors.Wrapf(err, "could not delete managed resource '%s/%s'", namespace, name)
	}

	if err := manager.
		NewSecret(client).
		WithNamespacedName(namespace, name).
		Delete(ctx); err != nil {
		return errors.Wrapf(err, "could not delete secret '%s/%s' of managed resource", namespace, name)
	}

	return nil
}

// WaitUntilManagedResourceDeleted waits until the given managed resource is deleted.
func WaitUntilManagedResourceDeleted(ctx context.Context, client client.Client, namespace, name string) error {
	mr := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return kutil.WaitUntilResourceDeleted(ctx, client, mr, 2*time.Second)
}
