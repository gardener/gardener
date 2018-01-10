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
	"path"

	"github.com/gardener/gardener/pkg/utils"
)

// ListResources will return a list of Kubernetes resources as JSON byte slice.
func (c *Client) ListResources(scoped bool, absPath []string) ([]byte, error) {
	return c.
		RESTClient.
		Get().
		AbsPath(absPath...).
		Do().
		Raw()
}

// CleanupResource will delete all resources except for those stored in the <exceptions> map.
func (c *Client) CleanupResource(exceptions map[string]bool, scoped bool, absPath ...string) error {
	body, err := c.ListResources(scoped, absPath)
	if err != nil {
		return err
	}

	outputs := utils.ConvertJSONToMap(body)
	items, err := outputs.ArrayOfObjects("items")
	if err != nil {
		return err
	}

	for _, item := range items {
		var (
			metadata        = item["metadata"].(map[string]interface{})
			name            = metadata["name"].(string)
			namespace       = ""
			exceptionScheme = name
			absPathDelete   = append(absPath, name)
		)

		if scoped {
			namespace = metadata["namespace"].(string)
			exceptionScheme = path.Join(namespace, name)
			resource := absPath[len(absPath)-1]
			absPathDelete = append(absPath[:len(absPath)-1], "namespaces", namespace, resource, name)
		}

		if _, ok := exceptions[exceptionScheme]; ok {
			continue
		}

		err := c.
			RESTClient.
			Delete().
			AbsPath(absPathDelete...).
			Do().
			Error()
		if err != nil {
			return err
		}
	}
	return nil
}

// CheckResourceCleanup will check whether all resources except for those in the <exceptions> map have been deleted.
func (c *Client) CheckResourceCleanup(exceptions map[string]bool, scoped bool, absPath ...string) (bool, error) {
	body, err := c.ListResources(scoped, absPath)
	if err != nil {
		return false, err
	}

	outputs := utils.ConvertJSONToMap(body)
	items, err := outputs.ArrayOfObjects("items")
	if err != nil {
		return false, err
	}

	for _, item := range items {
		var (
			metadata        = item["metadata"].(map[string]interface{})
			name            = metadata["name"].(string)
			namespace       = ""
			exceptionScheme = name
		)

		if scoped {
			namespace = metadata["namespace"].(string)
			exceptionScheme = path.Join(namespace, name)
		}

		if _, ok := exceptions[exceptionScheme]; !ok {
			return false, nil
		}
	}
	return true, nil
}
