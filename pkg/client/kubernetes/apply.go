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
	"bytes"
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func newControllerClient(config *rest.Config, options client.Options) (client.Client, error) {
	return client.New(config, options)
}

// NewControllerClient instantiates a new client.Client.
var NewControllerClient = newControllerClient

// NewApplierInternal constructs a new Applier from the given config and DiscoveryInterface.
// This method should only be used for testing.
// TODO(AC): Once https://github.com/kubernetes/kubernetes/issues/68865 is resolved,
// this should be adapted to use the updated RESTMapper (https://github.com/kubernetes/kubernetes/issues/75383)
// and not do the invalidation / checks on its own (depending on whether the controller-runtime/client might even automatically
// use this updated mapper then).
func NewApplierInternal(config *rest.Config, discoveryClient discovery.CachedDiscoveryInterface) (*Applier, error) {
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	c, err := NewControllerClient(config, client.Options{Mapper: mapper})
	if err != nil {
		return nil, err
	}

	return &Applier{client: c, discovery: discoveryClient}, nil
}

// NewApplierForConfig creates and returns a new Applier for the given rest.Config.
func NewApplierForConfig(config *rest.Config) (*Applier, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	cachedDiscoveryClient := cached.NewMemCacheClient(discoveryClient)
	return NewApplierInternal(config, cachedDiscoveryClient)
}

func (c *Applier) applyObject(ctx context.Context, desired *unstructured.Unstructured, options ApplierOptions) error {
	if desired.GetNamespace() == "" {
		desired.SetNamespace(metav1.NamespaceDefault)
	}

	key, err := client.ObjectKeyFromObject(desired)
	if err != nil {
		return err
	}

	if !c.discovery.Fresh() {
		c.discovery.Invalidate()
	}

	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(desired.GroupVersionKind())
	err = c.client.Get(ctx, key, current)
	if meta.IsNoMatchError(err) {
		c.discovery.Invalidate()
		err = c.client.Get(ctx, key, current)
	}
	if err != nil {
		if apierrors.IsNotFound(err) {
			return c.client.Create(ctx, desired)
		}
		return err
	}

	if err := c.mergeObjects(desired, current, options.MergeFuncs); err != nil {
		return err
	}

	return c.client.Update(ctx, desired)
}

// DefaultApplierOptions contains options for common k8s objects, e.g. Service, ServiceAccount.
var DefaultApplierOptions = ApplierOptions{
	MergeFuncs: map[Kind]MergeFunc{
		"Service": func(newObj, oldObj *unstructured.Unstructured) {
			// We do not want to overwrite a Service's `.spec.clusterIP' or '.spec.ports[*].nodePort' values.
			oldPorts := oldObj.Object["spec"].(map[string]interface{})["ports"].([]interface{})
			newPorts := newObj.Object["spec"].(map[string]interface{})["ports"].([]interface{})
			ports := []map[string]interface{}{}

			// Check whether ports of the newObj have also been present previously. If yes, take the nodePort
			// of the existing object.
			for _, newPort := range newPorts {
				np := newPort.(map[string]interface{})

				for _, oldPort := range oldPorts {
					op := oldPort.(map[string]interface{})
					// np["port"] is of type float64 (due to Helm Tiller rendering) while op["port"] is of type int64.
					// Equality can only be checked via their string representations.
					if fmt.Sprintf("%v", np["port"]) == fmt.Sprintf("%v", op["port"]) {
						if nodePort, ok := op["nodePort"]; ok {
							np["nodePort"] = nodePort
						}
					}
				}
				ports = append(ports, np)
			}

			newObj.Object["spec"].(map[string]interface{})["clusterIP"] = oldObj.Object["spec"].(map[string]interface{})["clusterIP"]
			newObj.Object["spec"].(map[string]interface{})["ports"] = ports
		},
		"ServiceAccount": func(newObj, oldObj *unstructured.Unstructured) {
			// We do not want to overwrite a ServiceAccount's `.secrets[]` list or `.imagePullSecrets[]`.
			newObj.Object["secrets"] = oldObj.Object["secrets"]
			newObj.Object["imagePullSecrets"] = oldObj.Object["imagePullSecrets"]
		},
	},
}

func (c *Applier) mergeObjects(newObj, oldObj *unstructured.Unstructured, mergeFuncs map[Kind]MergeFunc) error {
	newObj.SetResourceVersion(oldObj.GetResourceVersion())

	// We do not want to overwrite the Finalizers.
	newObj.Object["metadata"].(map[string]interface{})["finalizers"] = oldObj.Object["metadata"].(map[string]interface{})["finalizers"]

	if merge, ok := mergeFuncs[Kind(newObj.GetKind())]; ok {
		merge(newObj, oldObj)
	}

	return nil
}

// ApplyManifest is a function which does the same like `kubectl apply -f <file>`. It takes a bunch of manifests <m>,
// all concatenated in a byte slice, and sends them one after the other to the API server. If a resource
// already exists at the API server, it will update it. It returns an error as soon as the first error occurs.
func (c *Applier) ApplyManifest(ctx context.Context, r UnstructuredReader, options ApplierOptions) error {
	for obj, err := r.Read(); err == nil; obj, err = r.Read() {
		if obj == nil {
			continue
		}
		if err := c.applyObject(ctx, obj, options); err != nil {
			return err
		}
	}
	return nil
}

// UnstructuredReader an interface that all manifest readers should implement
type UnstructuredReader interface {
	Read() (*unstructured.Unstructured, error)
}

// NewManifestReader initializes a reader for yaml manifests
func NewManifestReader(manifest []byte) UnstructuredReader {
	return &manifestReader{
		decoder: yaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 1024),
	}
}

// manifestReader is an unstructured reader that contains a JSONDecoder
type manifestReader struct {
	decoder *yaml.YAMLOrJSONDecoder
}

// Read decodes yaml data into an unstructured object
func (m *manifestReader) Read() (*unstructured.Unstructured, error) {
	var (
		data map[string]interface{}
		err  error
	)

	// loop for skipping empty yaml objects
	for err = m.decoder.Decode(&data); err == nil; err = m.decoder.Decode(&data) {
		if data == nil {
			continue
		}
		return &unstructured.Unstructured{Object: data}, nil
	}
	return nil, err
}

// NewNamespaceSettingReader initializes a reader for yaml manifests with support for setting the namespace
func NewNamespaceSettingReader(mReader UnstructuredReader, namespace string) UnstructuredReader {
	return &namespaceSettingReader{
		reader:    mReader,
		namespace: namespace,
	}
}

// namespaceSettingReader is an unstructured reader that contains a JSONDecoder and a manifest reader (or other reader types)
type namespaceSettingReader struct {
	reader    UnstructuredReader
	namespace string
}

// Read decodes yaml data into an unstructured object
func (n *namespaceSettingReader) Read() (*unstructured.Unstructured, error) {
	readObj, err := n.reader.Read()
	if err != nil {
		return nil, err
	}

	readObj.SetNamespace(n.namespace)

	return readObj, nil
}
