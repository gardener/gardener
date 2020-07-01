// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/utils/retry"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TruncateLabelValue truncates a string at 63 characters so it's suitable for a label value.
func TruncateLabelValue(s string) string {
	if len(s) > 63 {
		return s[:63]
	}
	return s
}

// SetMetaDataLabel sets the key value pair in the labels section of the given Object.
// If the given Object did not yet have labels, they are initialized.
func SetMetaDataLabel(meta metav1.Object, key, value string) {
	labels := meta.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[key] = value
	meta.SetLabels(labels)
}

// SetMetaDataAnnotation sets the annotation on the given object.
// If the given Object did not yet have annotations, they are initialized.
func SetMetaDataAnnotation(meta metav1.Object, key, value string) {
	annotations := meta.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[key] = value
	meta.SetAnnotations(annotations)
}

// HasMetaDataAnnotation checks if the passed meta object has the given key, value set in the annotations section.
func HasMetaDataAnnotation(meta metav1.Object, key, value string) bool {
	val, ok := meta.GetAnnotations()[key]
	return ok && val == value
}

// HasDeletionTimestamp checks if an object has a deletion timestamp
func HasDeletionTimestamp(obj runtime.Object) (bool, error) {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return false, err
	}
	return metadata.GetDeletionTimestamp() != nil, nil
}

func nameAndNamespace(namespaceOrName string, nameOpt ...string) (namespace, name string) {
	if len(nameOpt) > 1 {
		panic(fmt.Sprintf("more than name/namespace for key specified: %s/%v", namespaceOrName, nameOpt))
	}
	if len(nameOpt) == 0 {
		name = namespaceOrName
		return
	}
	namespace = namespaceOrName
	name = nameOpt[0]
	return
}

// Key creates a new client.ObjectKey from the given parameters.
// There are only two ways to call this function:
// - If only namespaceOrName is set, then a client.ObjectKey with name set to namespaceOrName is returned.
// - If namespaceOrName and one nameOpt is given, then a client.ObjectKey with namespace set to namespaceOrName
//   and name set to nameOpt[0] is returned.
// For all other cases, this method panics.
func Key(namespaceOrName string, nameOpt ...string) client.ObjectKey {
	namespace, name := nameAndNamespace(namespaceOrName, nameOpt...)
	return client.ObjectKey{Namespace: namespace, Name: name}
}

// KeyFromObject obtains the client.ObjectKey from the given metav1.Object.
func KeyFromObject(obj metav1.Object) client.ObjectKey {
	return Key(obj.GetNamespace(), obj.GetName())
}

// ObjectMeta creates a new metav1.ObjectMeta from the given parameters.
// There are only two ways to call this function:
// - If only namespaceOrName is set, then a metav1.ObjectMeta with name set to namespaceOrName is returned.
// - If namespaceOrName and one nameOpt is given, then a metav1.ObjectMeta with namespace set to namespaceOrName
//   and name set to nameOpt[0] is returned.
// For all other cases, this method panics.
func ObjectMeta(namespaceOrName string, nameOpt ...string) metav1.ObjectMeta {
	namespace, name := nameAndNamespace(namespaceOrName, nameOpt...)
	return metav1.ObjectMeta{Namespace: namespace, Name: name}
}

// ObjectMetaFromKey returns an ObjectMeta with the namespace and name set to the values from the key.
func ObjectMetaFromKey(key client.ObjectKey) metav1.ObjectMeta {
	return ObjectMeta(key.Namespace, key.Name)
}

// WaitUntilResourceDeleted deletes the given resource and then waits until it has been deleted. It respects the
// given interval and timeout.
func WaitUntilResourceDeleted(ctx context.Context, c client.Client, obj runtime.Object, interval time.Duration) error {
	key, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return err
	}

	return retry.Until(ctx, interval, func(ctx context.Context) (done bool, err error) {
		if err := c.Get(ctx, key, obj); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.SevereError(err)
		}
		return retry.MinorError(fmt.Errorf("resource %s still exists", key.String()))
	})
}

// WaitUntilResourcesDeleted waits until the given resources are gone.
// It respects the given interval and timeout.
func WaitUntilResourcesDeleted(ctx context.Context, c client.Client, obj runtime.Object, interval time.Duration, opts ...client.ListOption) error {
	return retry.Until(ctx, interval, func(ctx context.Context) (done bool, err error) {
		if err := c.List(ctx, obj, opts...); err != nil {
			return retry.SevereError(err)
		}
		if meta.LenList(obj) == 0 {
			return retry.Ok()
		}
		var remainingItems []string
		acc := meta.NewAccessor()
		if err := meta.EachListItem(obj, func(remainingObj runtime.Object) error {
			name, err := acc.Name(remainingObj)
			if err != nil {
				return err
			}
			remainingItems = append(remainingItems, name)
			return nil
		}); err != nil {
			return retry.SevereError(err)
		}
		return retry.MinorError(fmt.Errorf("resource(s) %s still exists", remainingItems))
	})
}

// WaitUntilResourceDeletedWithDefaults deletes the given resource and then waits until it has been deleted. It
// uses a default interval and timeout
func WaitUntilResourceDeletedWithDefaults(ctx context.Context, c client.Client, obj runtime.Object) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	return WaitUntilResourceDeleted(ctx, c, obj, 5*time.Second)
}

// GetLoadBalancerIngress takes a context, a client, a namespace and a service name. It queries for a load balancer's technical name
// (ip address or hostname). It returns the value of the technical name whereby it always prefers the hostname (if given)
// over the IP address. It also returns the list of all load balancer ingresses.
func GetLoadBalancerIngress(ctx context.Context, client client.Client, namespace, name string) (string, error) {
	service := &corev1.Service{}
	if err := client.Get(ctx, Key(namespace, name), service); err != nil {
		return "", err
	}

	var (
		serviceStatusIngress = service.Status.LoadBalancer.Ingress
		length               = len(serviceStatusIngress)
	)

	switch {
	case length == 0:
		return "", errors.New("`.status.loadBalancer.ingress[]` has no elements yet, i.e. external load balancer has not been created")
	case serviceStatusIngress[length-1].Hostname != "":
		return serviceStatusIngress[length-1].Hostname, nil
	case serviceStatusIngress[length-1].IP != "":
		return serviceStatusIngress[length-1].IP, nil
	}

	return "", errors.New("`.status.loadBalancer.ingress[]` has an element which does neither contain `.ip` nor `.hostname`")
}

// LookupObject retrieves an obj for the given object key dealing with potential stale cache that still does not contain the obj.
// It first tries to retrieve the obj using the given cached client.
// If the object key is not found, then it does live lookup from the API server using the given apiReader.
func LookupObject(ctx context.Context, c client.Client, apiReader client.Reader, key client.ObjectKey, obj runtime.Object) error {
	err := c.Get(ctx, key, obj)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	// Try to get the obj, now by doing a live lookup instead of relying on the cache.
	return apiReader.Get(ctx, key, obj)
}
