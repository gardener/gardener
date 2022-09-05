// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package test

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/client/kubernetes"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewClientWithFieldSelectorSupport takes a fake client and a function that returns selectable fields for the type T
// and adds support for field selectors to the client.
// TODO(plkokanov): remove this once the controller-runtime fake client supports field selectors.
func NewClientWithFieldSelectorSupport[T any](c client.Client, toSelectableFieldsFunc func(t *T) fields.Set) client.Client {
	return &clientWithFieldSelectorSupport[T]{
		Client:                 c,
		toSelectableFieldsFunc: toSelectableFieldsFunc,
	}
}

type clientWithFieldSelectorSupport[T any] struct {
	client.Client
	toSelectableFieldsFunc func(t *T) fields.Set
}

func (c clientWithFieldSelectorSupport[T]) List(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) error {
	if err := c.Client.List(ctx, obj, opts...); err != nil {
		return err
	}

	listOpts := client.ListOptions{}
	listOpts.ApplyOptions(opts)

	if listOpts.FieldSelector != nil {
		objs, err := meta.ExtractList(obj)
		if err != nil {
			return err
		}
		filteredObjs, err := c.filterWithFieldSelector(objs, listOpts.FieldSelector)
		if err != nil {
			return err
		}
		err = meta.SetList(obj, filteredObjs)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c clientWithFieldSelectorSupport[T]) filterWithFieldSelector(objs []runtime.Object, sel fields.Selector) ([]runtime.Object, error) {
	outItems := make([]runtime.Object, 0, len(objs))
	for _, obj := range objs {
		// convert to internal
		internalObj := new(T)
		if err := kubernetes.GardenScheme.Convert(obj, internalObj, nil); err != nil {
			return nil, err
		}

		fieldSet := c.toSelectableFieldsFunc(internalObj)

		// complain about non-selectable fields if any
		for _, req := range sel.Requirements() {
			if !fieldSet.Has(req.Field) {
				return nil, fmt.Errorf("field selector not supported for field %q", req.Field)
			}
		}

		if !sel.Matches(fieldSet) {
			continue
		}
		outItems = append(outItems, obj.DeepCopyObject())
	}
	return outItems, nil
}
