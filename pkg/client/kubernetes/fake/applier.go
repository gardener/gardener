// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package fake

import (
	"context"
	"fmt"
	"io"

	"github.com/hashicorp/go-multierror"
	"k8s.io/apimachinery/pkg/runtime"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	utilerrors "github.com/gardener/gardener/pkg/utils/errors"
)

var _ kubernetes.Applier = &Applier{}

// Applier applies objects by retrieving their current state and then either creating / updating them
// (update happens with a predefined merge logic).
type Applier struct {
	scheme *runtime.Scheme

	Objects []runtime.Object
}

// NewApplier returns a fake applier, which tries to convert manifests (from a rendered chart) to their
// respective golang type using `scheme` and saves them to `Objects` instead of applying them. If the corresponding
// gvk is not registered in the scheme, the resulting object will be of type *unstructured.Unstructured.
// This is useful for testing components that apply/delete a rendered chart (e.g. with the help of helm.ChartComponent).
func NewApplier(scheme *runtime.Scheme) *Applier {
	if scheme == nil {
		scheme = k8sscheme.Scheme
	}
	return &Applier{scheme: scheme}
}

// ApplyManifest is a function which does the same like `kubectl apply -f <file>`. It takes a bunch of manifests <m>,
// all concatenated in a byte slice, and sends them one after the other to the API server. If a resource
// already exists at the API server, it will update it. It returns an error as soon as the first error occurs.
func (a *Applier) ApplyManifest(_ context.Context, r kubernetes.UnstructuredReader, _ kubernetes.MergeFuncs) error {
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

		if typed, err := a.scheme.New(obj.GetObjectKind().GroupVersionKind()); err == nil {
			if err := a.scheme.Convert(obj, typed, nil); err == nil {
				// object successfully converted to corresponding golang type
				a.Objects = append(a.Objects, typed)
				continue
			}
		}

		// object type not registered in scheme, add unstructured object to result slice
		a.Objects = append(a.Objects, obj)
	}

	return allErrs.ErrorOrNil()
}

// DeleteManifest is not implemented yet.
func (a *Applier) DeleteManifest(context.Context, kubernetes.UnstructuredReader, ...kubernetes.DeleteManifestOption) error {
	return nil
}
