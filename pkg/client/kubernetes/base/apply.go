// Copyright 2018 The Gardener Authors.
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

		absPath, e := c.BuildPath(apiVersion, kind, namespace)
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

			if kind == "Service" {
				newObj.Object["spec"].(map[string]interface{})["clusterIP"] = oldObj.Object["spec"].(map[string]interface{})["clusterIP"]
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

// BuildPath creates the Kubernetes API REST URL for the given API group and kind (depending on whether the
// kind is namespaced or not).
func (c *Client) BuildPath(apiVersion, kind, namespace string) (string, error) {
	if namespace == "" {
		namespace = metav1.NamespaceDefault
	}

	apiGroup := apiVersion
	apiGroupPath := "api"
	if apiGroup != "v1" {
		apiGroupPath += "s"
	}
	apiGroupPath = path.Join(apiGroupPath, apiVersion)

	for _, apiGroup := range c.GetAPIResourceList() {
		if apiGroup.GroupVersion == apiVersion {
			for _, resource := range apiGroup.APIResources {
				if resource.Kind == kind {
					if resource.Namespaced {
						return path.Join(apiGroupPath, "namespaces", namespace, resource.Name), nil
					}
					return path.Join(apiGroupPath, resource.Name), nil
				}
			}
			return "", fmt.Errorf("%s is not registered in API group %s", kind, apiVersion)
		}
	}

	// There is no clear indicator for the kube-apiserver whether it is ready to serve requests or not. We discover
	// all the available API groups while creating the Kubernetes client, however, at that point in time it might be
	// the case that not all API groups have been registered. Thus, we need to have some retry logic regarding the
	// discovery.
	// See also: https://github.com/kubernetes/kubernetes/issues/45786
	if c.apiDiscoveryFetchNum < 5 {
		// Refresh Seed client API group discovery
		if err := c.DiscoverAPIGroups(); err != nil {
			return "", fmt.Errorf("Failure while re-discoverying the API groups (try no. %d)", c.apiDiscoveryFetchNum)
		}
		return c.BuildPath(apiVersion, kind, namespace)
	}

	return "", fmt.Errorf("Could not find API group %s", apiVersion)
}

// get performs a HTTP GET request on the given path. It will return the result object.
func (c *Client) get(path string) rest.Result {
	return c.restClient.Get().AbsPath(path).Do()
}

// post performs a HTTP POST request on the given path and with the given body (must be a byte
// slice containing valid JSON). An error will be returned if one occurrs.
func (c *Client) post(path string, body []byte) error {
	return c.restClient.Post().AbsPath(path).Body(body).Do().Error()
}

// put performs a HTTP PUT request on the given path and with the given body (must be a byte
// slice containing valid JSON). An error will be returned if one occurrs.
func (c *Client) put(path string, body []byte) error {
	return c.restClient.Put().AbsPath(path).Body(body).Do().Error()
}

// patch performs a HTTP PATCH request on the given path and with the given body (must be a byte
// slice containing valid JSON). An error will be returned if one occurrs.
// The patch type is merge patch.
func (c *Client) patch(path string, body []byte) error {
	return c.restClient.Patch(types.MergePatchType).AbsPath(path).Body(body).Do().Error()
}
