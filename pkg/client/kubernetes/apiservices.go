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
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	apiregistrationclientset "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1beta1"
)

func (c *Clientset) apiServices() apiregistrationclientset.APIServiceInterface {
	return c.apiregistration.ApiregistrationV1beta1().APIServices()
}

// ListAPIServices will list all the APIServices for the given <listOptions>.
func (c *Clientset) ListAPIServices(opts metav1.ListOptions) (*apiregistrationv1beta1.APIServiceList, error) {
	return c.apiServices().List(opts)
}

// DeleteAPIService will gracefully delete an APIService with the given <name>.
func (c *Clientset) DeleteAPIService(name string) error {
	return c.apiServices().Delete(name, &defaultDeleteOptions)
}

// DeleteAPIServiceForcefully will forcefully delete an APIService with the given <name>.
func (c *Clientset) DeleteAPIServiceForcefully(name string) error {
	apiService, err := c.apiServices().Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	finalizersRemoved := apiService.DeepCopy()
	finalizersRemoved.Finalizers = nil

	patch, err := kutil.CreateTwoWayMergePatch(apiService, finalizersRemoved)
	if err != nil {
		return err
	}
	if !kutil.IsEmptyPatch(patch) {
		if _, err = c.apiServices().Patch(name, types.StrategicMergePatchType, patch); err != nil {
			return err
		}
	}

	if err := c.apiServices().Delete(name, &forceDeleteOptions); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
