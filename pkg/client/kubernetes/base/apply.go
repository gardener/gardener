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

package kubernetesbase

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
)

// Apply is a function which does the same like `kubectl apply -f <file>`. It takes a bunch of manifests <m>,
// all concatenated in a byte slice, and sends them one after the other to the API server. If a resource
// already exists at the API server, it will update it. It returns an error as soon as the first error occurs.
func (c *Client) Apply(m []byte) error {
	var (
		decoder    = yaml.NewYAMLOrJSONDecoder(bytes.NewReader(m), 1024)
		decodedObj map[string]interface{}
		err        error
	)

	for err = decoder.Decode(&decodedObj); err == nil; err = decoder.Decode(&decodedObj) {
		if decodedObj == nil {
			continue
		}

		newObj := unstructured.Unstructured{Object: decodedObj}
		decodedObj = nil

		manifest, e := json.Marshal(newObj.UnstructuredContent())
		if e != nil {
			return e
		}

		var (
			apiVersion = newObj.GetAPIVersion()
			kind       = newObj.GetKind()
			name       = newObj.GetName()
			namespace  = newObj.GetNamespace()
		)

		absPath, e := c.buildPath(apiVersion, kind, namespace)
		if e != nil {
			return e
		}

		var (
			absPathName = path.Join(absPath, name)
			getResult   = c.get(absPathName)
			getErr      = getResult.Error()
			oldObj      unstructured.Unstructured
		)

		if apierrors.IsNotFound(getErr) {
			// Resource object was not found, i.e. it does not exist yet. We need to POST.
			if postErr := c.post(absPath, manifest); postErr != nil {
				// Handle race conditions in which the resource has been created in the meanwhile.
				if apierrors.IsAlreadyExists(postErr) {
					return c.Apply(m)
				}
				return postErr
			}
		} else {
			if getErr != nil {
				// We have received an error from the GET request which we do not expect.
				return getErr
			}

			// We have received an result from the GET request, i.e. the object does exist already.
			// We need to use the current resource version and inject it into the new manifest.
			raw, e := getResult.Raw()
			if e != nil {
				return e
			}
			if e := json.Unmarshal(raw, &oldObj); e != nil {
				return e
			}
			newObj.SetResourceVersion(oldObj.GetResourceVersion())

			// We do not want to overwrite the Finalizers.
			newObj.Object["metadata"].(map[string]interface{})["finalizers"] = oldObj.Object["metadata"].(map[string]interface{})["finalizers"]

			switch kind {
			case "Service":
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
			case "ServiceAccount":
				// We do not want to overwrite a ServiceAccount's `.secrets[]` list or `.imagePullSecrets[]`.
				newObj.Object["secrets"] = oldObj.Object["secrets"]
				newObj.Object["imagePullSecrets"] = oldObj.Object["imagePullSecrets"]
			}

			manifest, e = json.Marshal(newObj.UnstructuredContent())
			if e != nil {
				return e
			}

			if putErr := c.put(absPathName, manifest); putErr != nil {
				return putErr
			}
		}
	}
	if err != io.EOF {
		return err
	}
	return nil
}

// buildPath creates the Kubernetes API REST URL for the given API group and kind (depending on whether the
// kind is namespaced or not).
func (c *Client) buildPath(apiVersion, kind, namespace string) (string, error) {
	if namespace == "" {
		namespace = metav1.NamespaceDefault
	}

	var (
		apiGroup     = apiVersion
		apiGroupPath = "api"
		apiPath      = ""
	)

	if apiGroup != "v1" {
		apiGroupPath = "apis"
	}
	apiGroupPath = path.Join(apiGroupPath, apiVersion)

	// There is no clear indicator for the kube-apiserver whether it is ready to serve requests or not. We discover
	// all the available API groups while creating the Kubernetes client, however, at that point in time it might be
	// the case that not all API groups have been registered or/and the responsible APIService objects have been created.
	// Thus, we need to have some retry logic regarding the discovery.
	// See also: https://github.com/kubernetes/kubernetes/issues/45786

	// Perform at most four tries with the following approx. delays: +0.5s, +1.8s, +5s, +11.8s
	backoff := wait.Backoff{
		Duration: time.Second,
		Factor:   2.5,
		Jitter:   0.3,
		Steps:    4,
	}

	if err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		for _, apiGroup := range c.GetAPIResourceList() {
			if apiGroup.GroupVersion == apiVersion {
				for _, resource := range apiGroup.APIResources {
					if resource.Kind == kind {
						if resource.Namespaced {
							apiPath = path.Join(apiGroupPath, "namespaces", namespace, resource.Name)
							return true, nil
						}
						apiPath = path.Join(apiGroupPath, resource.Name)
						return true, nil
					}
				}
			}
		}
		// Refresh client API discovery
		if err := c.DiscoverAPIGroups(); err != nil {
			return false, fmt.Errorf("Failure while re-discoverying the API groups: %v", err.Error())
		}
		return false, nil
	}); err != nil {
		return "", fmt.Errorf("Could not construct the API path for apiVersion %s and kind %s: (%v)", apiVersion, kind, err)
	}

	return apiPath, nil
}

// get performs a HTTP GET request on the given path. It will return the result object.
func (c *Client) get(path string) rest.Result {
	return c.restClient.Get().AbsPath(path).Do()
}

// post performs a HTTP POST request on the given path and with the given body (must be a byte
// slice containing valid JSON). An error will be returned if one occurs.
func (c *Client) post(path string, body []byte) error {
	return c.restClient.Post().AbsPath(path).Body(body).Do().Error()
}

// put performs a HTTP PUT request on the given path and with the given body (must be a byte
// slice containing valid JSON). An error will be returned if one occurs.
func (c *Client) put(path string, body []byte) error {
	return c.restClient.Put().AbsPath(path).Body(body).Do().Error()
}

// patch performs a HTTP PATCH request on the given path and with the given body (must be a byte
// slice containing valid JSON). An error will be returned if one occurs.
// The patch type is merge patch.
func (c *Client) patch(path string, body []byte) error {
	return c.restClient.Patch(types.MergePatchType).AbsPath(path).Body(body).Do().Error()
}
