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
	"encoding/json"

	"github.com/gardener/gardener/pkg/client/kubernetes/mapping"
	apiextensions_v1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

var crdPath = []string{"apis", "apiextensions.k8s.io", "v1beta1", "customresourcedefinitions"}

// GetCRD returns a CustomResourceDefinition object. For the sake of simplicity, we do not
// use the APIExtensions client.
func (c *Client) GetCRD(name string) (*mapping.CustomResourceDefinition, error) {
	var crd apiextensions_v1beta1.CustomResourceDefinition
	body, err := c.
		RESTClient.
		Get().
		AbsPath(crdPath[0], crdPath[1], crdPath[2], crdPath[3], name).
		Do().
		Raw()
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &crd)
	if err != nil {
		return nil, err
	}
	return mapping.ApiextensionsV1beta1CustomResourceDefinition(crd), nil
}

// CleanupCRDs deletes all the TPRs/CRDs in the cluster other than those stored in the
// exceptions map <exceptions>.
func (c *Client) CleanupCRDs(exceptions map[string]bool) error {
	return c.CleanupResource(exceptions, false, crdPath...)
}

// CheckCRDCleanup will check whether all the CRDs in the cluster other than those
// stored in the exceptions map <exceptions> have been deleted. It will return an error
// in case it has not finished yet, and nil if all resources are gone.
func (c *Client) CheckCRDCleanup(exceptions map[string]bool) (bool, error) {
	return c.CheckResourceCleanup(exceptions, false, crdPath...)
}
