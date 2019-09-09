/*
 * Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 *
 */

package resources

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type AbstractResource struct {
	self   Internal
	helper *ResourceHelper
}

type ResourceHelper struct {
	Internal
}

func NewAbstractResource(self Internal) (AbstractResource, *ResourceHelper) {
	helper := &ResourceHelper{self}
	return AbstractResource{self, helper}, helper
}

func (this *AbstractResource) Name() string {
	return this.self.Info().Name()
}

func (this *AbstractResource) GroupVersionKind() schema.GroupVersionKind {
	return this.self.Info().GroupVersionKind()
}

func (this *AbstractResource) GroupKind() schema.GroupKind {
	return this.self.Info().GroupKind()
}

func (this *AbstractResource) Namespaced() bool {
	return this.self.Info().Namespaced()
}

func (this *AbstractResource) Wrap(obj ObjectData) (Object, error) {
	if err := this.helper.CheckOType(obj); err != nil {
		return nil, err
	}
	return this.helper.ObjectAsResource(obj), nil
}

func (this *AbstractResource) New(name ObjectName) Object {
	data := this.helper.CreateData()
	data.GetObjectKind().SetGroupVersionKind(this.GroupVersionKind())
	if name != nil {
		data.SetName(name.Name())
		data.SetNamespace(name.Namespace())
	}
	return this.helper.ObjectAsResource(data)
}

func (this *AbstractResource) Namespace(namespace string) Namespaced {
	return &namespacedResource{this, namespace, nil}
}

////////////////////////////////////////////////////////////////////////////////
// Resource Helper

func (this *ResourceHelper) ObjectAsResource(obj ObjectData) Object {
	return NewObject(obj, this.GetCluster(), this.Internal)
}

func (this *ResourceHelper) CreateData() ObjectData {
	data := reflect.New(this.I_objectType()).Interface().(ObjectData)
	if u, ok := data.(*unstructured.Unstructured); ok {
		u.SetGroupVersionKind(this.GroupVersionKind())
	}
	return data
}

func (this *ResourceHelper) CreateListData() runtime.Object {
	return reflect.New(this.I_listType()).Interface().(runtime.Object)
}

func (this *ResourceHelper) CheckOType(obj ObjectData, unstructured ...bool) error {
	t := reflect.TypeOf(obj)
	if t.Kind() == reflect.Ptr {
		if t.Elem() == this.I_objectType() {
			return nil
		}
		if len(unstructured) > 0 && unstructured[0] {
			if t.Elem() == unstructuredType {
				return nil
			}
		}
	}
	return fmt.Errorf("wrong data type %T (expected %s)", obj, reflect.PtrTo(this.I_objectType()))
}

func (this *ResourceHelper) Get(namespace, name string, result ObjectData) (Object, error) {
	if !this.Namespaced() && namespace != "" {
		return nil, fmt.Errorf("%s is not namespaced", this.GroupKind())
	}
	if this.Namespaced() && namespace == "" {
		return nil, fmt.Errorf("%s is namespaced", this.GroupKind())
	}

	if result == nil {
		result = this.CreateData()
	}
	result.SetNamespace(namespace)
	result.SetName(name)
	err := this.I_get(result)
	return this.ObjectAsResource(result), err
}
