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
	"path"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// Apply is a function which does the same like `kubectl apply -f <file>`. It takes a bunch of manifests <m>,
// all concatenated in a byte slice, and sends them one after the other to the API server. If a resource
// already exists at the API server, it will update it. It returns an error as soon as the first error occurs.
func (c *Client) Apply(m []byte) error {
	var (
		decoder    = yaml.NewYAMLOrJSONDecoder(bytes.NewReader(m), 1024)
		decodedObj map[string]interface{}
	)

	for err := decoder.Decode(&decodedObj); err == nil; err = decoder.Decode(&decodedObj) {
		if len(decodedObj) == 0 {
			continue
		}
		manifest, e := json.Marshal(decodedObj)
		if e != nil {
			return e
		}
		decodedObj = nil

		var metadata struct {
			metav1.TypeMeta   `json:",inline"`
			metav1.ObjectMeta `json:"metadata,omitempty"`
		}
		if e := json.Unmarshal(manifest, &metadata); e != nil {
			return e
		}

		var (
			apiVersion = metadata.APIVersion
			kind       = metadata.Kind
			name       = metadata.Name
			namespace  = metadata.Namespace
		)

		absPath, e := c.BuildPath(apiVersion, kind, namespace)
		if e != nil {
			return e
		}
		absPathName := path.Join(absPath, name)

		if postErr := c.post(absPath, manifest); postErr != nil {
			if apierrors.IsAlreadyExists(postErr) {
				switch kind {
				case "Service":
					if patchErr := c.patch(absPathName, manifest); patchErr != nil {
						return patchErr
					}
				default:
					if putErr := c.put(absPathName, manifest); putErr != nil {
						return putErr
					}
				}
			} else {
				return postErr
			}
		}
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
	return "", fmt.Errorf("Could not find API group %s", apiVersion)
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
