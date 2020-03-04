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
	"io"

	utilerrors "github.com/gardener/gardener/pkg/utils/errors"
	"github.com/hashicorp/go-multierror"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	memcache "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newControllerClient(config *rest.Config, options client.Options) (client.Client, error) {
	return client.New(config, options)
}

// NewControllerClient instantiates a new client.Client.
var NewControllerClient = newControllerClient

// NewApplierInternal constructs a new Applier from the given config and DiscoveryInterface.
// This method should only be used for testing.
func NewApplierInternal(config *rest.Config, discoveryClient discovery.CachedDiscoveryInterface) (*Applier, error) {
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	c, err := NewControllerClient(config, client.Options{Mapper: mapper})
	if err != nil {
		return nil, err
	}

	return &Applier{client: c, restMapper: mapper}, nil
}

// NewApplierForConfig creates and returns a new Applier for the given rest.Config.
func NewApplierForConfig(config *rest.Config) (*Applier, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	cachedDiscoveryClient := memcache.NewMemCacheClient(discoveryClient)
	return NewApplierInternal(config, cachedDiscoveryClient)
}

func (c *Applier) applyObject(ctx context.Context, desired *unstructured.Unstructured, options MergeFuncs) error {
	if desired.GetNamespace() == "" {
		desired.SetNamespace(metav1.NamespaceDefault)
	}

	key, err := client.ObjectKeyFromObject(desired)
	if err != nil {
		return err
	}

	if len(key.Name) == 0 {
		return fmt.Errorf("missing 'metadata.name' in: %+v", desired)
	}

	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(desired.GroupVersionKind())
	err = c.client.Get(ctx, key, current)
	if meta.IsNoMatchError(err) {
		c.restMapper.Reset()
		err = c.client.Get(ctx, key, current)
	}
	if err != nil {
		if apierrors.IsNotFound(err) {
			return c.client.Create(ctx, desired)
		}
		return err
	}

	if err := c.mergeObjects(desired, current, options); err != nil {
		return err
	}

	return c.client.Update(ctx, desired)
}

func (c *Applier) deleteObject(ctx context.Context, desired *unstructured.Unstructured) error {
	if desired.GetNamespace() == "" {
		desired.SetNamespace(metav1.NamespaceDefault)
	}
	if len(desired.GetName()) == 0 {
		return fmt.Errorf("missing 'metadata.name' in: %+v", desired)
	}

	err := c.client.Delete(ctx, desired)
	if err != nil && apierrors.IsNotFound(err) {
		return nil
	}
	return err

}

// DefaultMergeFuncs contains options for common k8s objects, e.g. Service, ServiceAccount.
var DefaultMergeFuncs = MergeFuncs{
	corev1.SchemeGroupVersion.WithKind("Service").GroupKind(): func(newObj, oldObj *unstructured.Unstructured) {
		newSvcType, found, _ := unstructured.NestedString(newObj.Object, "spec", "type")
		if !found {
			newSvcType = string(corev1.ServiceTypeClusterIP)
			_ = unstructured.SetNestedField(newObj.Object, newSvcType, "spec", "type")
		}

		oldSvcType, found, _ := unstructured.NestedString(oldObj.Object, "spec", "type")
		if !found {
			oldSvcType = string(corev1.ServiceTypeClusterIP)

		}

		switch newSvcType {
		case string(corev1.ServiceTypeLoadBalancer), string(corev1.ServiceTypeNodePort):
			oldPorts, found, _ := unstructured.NestedSlice(oldObj.Object, "spec", "ports")
			if !found {
				// no old ports probably means that the service was of type External name before.
				break
			}

			newPorts, found, _ := unstructured.NestedSlice(newObj.Object, "spec", "ports")
			if !found {
				// no new ports is safe to ignore
				break
			}

			ports := []interface{}{}
			for _, newPort := range newPorts {
				np := newPort.(map[string]interface{})
				npName, _, _ := unstructured.NestedString(np, "name")
				npPort, _ := nestedFloat64OrInt64(np, "port")
				nodePort, ok := nestedFloat64OrInt64(np, "nodePort")

				for _, oldPortObj := range oldPorts {
					op := oldPortObj.(map[string]interface{})
					opName, _, _ := unstructured.NestedString(op, "name")
					opPort, _ := nestedFloat64OrInt64(op, "port")

					if (opName == npName || opPort == npPort) && (!ok || nodePort == 0) {
						np["nodePort"] = op["nodePort"]
					}
				}

				ports = append(ports, np)
			}

			_ = unstructured.SetNestedSlice(newObj.Object, ports, "spec", "ports")

		case string(corev1.ServiceTypeExternalName):
			// there is no ClusterIP in this case
			return
		}

		// ClusterIP is immutable unless that old service is of type ExternalName
		if oldSvcType != string(corev1.ServiceTypeExternalName) {
			newClusterIP, _, _ := unstructured.NestedString(newObj.Object, "spec", "clusterIP")
			if newClusterIP != string(corev1.ClusterIPNone) || newSvcType != string(corev1.ServiceTypeClusterIP) {
				oldClusterIP, _, _ := unstructured.NestedString(oldObj.Object, "spec", "clusterIP")
				_ = unstructured.SetNestedField(newObj.Object, oldClusterIP, "spec", "clusterIP")
			}
		}

		newETP, _, _ := unstructured.NestedString(newObj.Object, "spec", "externalTrafficPolicy")
		oldETP, _, _ := unstructured.NestedString(oldObj.Object, "spec", "externalTrafficPolicy")

		if oldSvcType == string(corev1.ServiceTypeLoadBalancer) &&
			newSvcType == string(corev1.ServiceTypeLoadBalancer) &&
			newETP == string(corev1.ServiceExternalTrafficPolicyTypeLocal) &&
			oldETP == string(corev1.ServiceExternalTrafficPolicyTypeLocal) {
			newHealthCheckPort, _ := nestedFloat64OrInt64(newObj.Object, "spec", "healthCheckNodePort")
			if newHealthCheckPort == 0 {
				oldHealthCheckPort, _ := nestedFloat64OrInt64(oldObj.Object, "spec", "healthCheckNodePort")
				_ = unstructured.SetNestedField(newObj.Object, oldHealthCheckPort, "spec", "healthCheckNodePort")
			}
		}

	},
	corev1.SchemeGroupVersion.WithKind("ServiceAccount").GroupKind(): func(newObj, oldObj *unstructured.Unstructured) {
		// We do not want to overwrite a ServiceAccount's `.secrets[]` list or `.imagePullSecrets[]`.
		newObj.Object["secrets"] = oldObj.Object["secrets"]
		newObj.Object["imagePullSecrets"] = oldObj.Object["imagePullSecrets"]
	},
	{Group: "autoscaling.k8s.io", Kind: "VerticalPodAutoscaler"}: func(newObj, oldObj *unstructured.Unstructured) {
		// Never override the status of VPA resources
		newObj.Object["status"] = oldObj.Object["status"]
	},
}

func nestedFloat64OrInt64(obj map[string]interface{}, fields ...string) (int64, bool) {
	val, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if !found || err != nil {
		return 0, found
	}

	f, ok := val.(float64)
	if ok {
		return int64(f), true
	}

	i, ok := val.(int64)
	if ok {
		return i, true
	}

	return 0, false
}

// CopyApplierOptions returns a copies of the provided applier options.
func CopyApplierOptions(in MergeFuncs) MergeFuncs {
	out := make(MergeFuncs, len(in))

	for k, v := range in {
		out[k] = v
	}

	return out
}

func (c *Applier) mergeObjects(newObj, oldObj *unstructured.Unstructured, mergeFuncs MergeFuncs) error {
	newObj.SetResourceVersion(oldObj.GetResourceVersion())

	// We do not want to overwrite the Finalizers.
	newObj.Object["metadata"].(map[string]interface{})["finalizers"] = oldObj.Object["metadata"].(map[string]interface{})["finalizers"]

	if merge, ok := mergeFuncs[newObj.GroupVersionKind().GroupKind()]; ok {
		merge(newObj, oldObj)
	}

	return nil
}

// ApplyManifest is a function which does the same like `kubectl apply -f <file>`. It takes a bunch of manifests <m>,
// all concatenated in a byte slice, and sends them one after the other to the API server. If a resource
// already exists at the API server, it will update it. It returns an error as soon as the first error occurs.
func (c *Applier) ApplyManifest(ctx context.Context, r UnstructuredReader, options MergeFuncs) error {
	allErrs := &multierror.Error{
		ErrorFormat: utilerrors.NewErrorFormatFuncWithPrefix("failed to apply manifests"),
	}

	for {
		obj, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("could not read object: %+v", err))
			continue
		}
		if obj == nil {
			continue
		}

		if err := c.applyObject(ctx, obj, options); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("could not apply object of kind %q \"%s/%s\": %+v", obj.GetKind(), obj.GetNamespace(), obj.GetName(), err))
			continue
		}
	}

	return allErrs.ErrorOrNil()
}

// DeleteManifest is a function which does the same like `kubectl delete -f <file>`. It takes a bunch of manifests <m>,
// all concatenated in a byte slice, and sends them one after the other to the API server for deletion.
// It returns an error as soon as the first error occurs.
func (c *Applier) DeleteManifest(ctx context.Context, r UnstructuredReader) error {
	allErrs := &multierror.Error{
		ErrorFormat: utilerrors.NewErrorFormatFuncWithPrefix("failed to delete manifests"),
	}

	for {
		obj, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("could not read object: %+v", err))
			continue
		}
		if obj == nil {
			continue
		}

		if err := c.deleteObject(ctx, obj); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("could not delete object of kind %q \"%s/%s\": %+v", obj.GetKind(), obj.GetNamespace(), obj.GetName(), err))
			continue
		}
	}

	return allErrs.ErrorOrNil()
}

// UnstructuredReader an interface that all manifest readers should implement
type UnstructuredReader interface {
	Read() (*unstructured.Unstructured, error)
}

// NewManifestReader initializes a reader for yaml manifests
func NewManifestReader(manifest []byte) UnstructuredReader {
	return &manifestReader{
		decoder:  yaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 1024),
		manifest: manifest,
	}
}

// manifestReader is an unstructured reader that contains a JSONDecoder
type manifestReader struct {
	decoder  *yaml.YAMLOrJSONDecoder
	manifest []byte
}

// Read decodes yaml data into an unstructured object
func (m *manifestReader) Read() (*unstructured.Unstructured, error) {
	// loop for skipping empty yaml objects
	for {
		var data map[string]interface{}

		err := m.decoder.Decode(&data)
		if err == io.EOF {
			return nil, err
		}
		if err != nil {
			return nil, fmt.Errorf("error '%+v' decoding manifest: %s", err, string(m.manifest))
		}
		if data == nil {
			continue
		}

		return &unstructured.Unstructured{Object: data}, nil
	}
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

// NewObjectReferenceReader initializes a reader from ObjectReference
func NewObjectReferenceReader(objectReference *corev1.ObjectReference) UnstructuredReader {
	return &objectReferenceReader{
		objectReference: objectReference,
	}
}

// objectReferenceReader is an unstructured reader that contains a ObjectReference
type objectReferenceReader struct {
	objectReference *corev1.ObjectReference
}

// Read translates ObjectReference into Unstructured object
func (r *objectReferenceReader) Read() (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(r.objectReference.APIVersion)
	obj.SetKind(r.objectReference.Kind)
	obj.SetNamespace(r.objectReference.Namespace)
	obj.SetName(r.objectReference.Name)

	return obj, nil
}
