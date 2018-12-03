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
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (c *Clientset) crds() apiextensionsclientset.CustomResourceDefinitionInterface {
	return c.apiextension.ApiextensionsV1beta1().CustomResourceDefinitions()
}

// ListCRDs will list all the CRDs for the given <listOptions>.
func (c *Clientset) ListCRDs(opts metav1.ListOptions) (*apiextensionsv1beta1.CustomResourceDefinitionList, error) {
	return c.apiextension.ApiextensionsV1beta1().CustomResourceDefinitions().List(opts)
}

// DeleteCRDForcefully will forcefully delete a CRD with the given <name>.
func (c *Clientset) DeleteCRDForcefully(name string) error {
	crd, err := c.crds().Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	finalizersRemoved := crd.DeepCopy()
	finalizersRemoved.Finalizers = nil

	patch, err := kutil.CreateTwoWayMergePatch(crd, finalizersRemoved)
	if err != nil {
		return err
	}
	if !kutil.IsEmptyPatch(patch) {
		if _, err = c.crds().Patch(name, types.StrategicMergePatchType, patch); err != nil {
			return err
		}
	}

	if err := c.crds().Delete(name, &forceDeleteOptions); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
