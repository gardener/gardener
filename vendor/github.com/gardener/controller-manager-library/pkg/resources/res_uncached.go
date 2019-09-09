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
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (this *AbstractResource) Create(obj ObjectData) (Object, error) {
	if o, ok := obj.(Object); ok {
		obj = o.Data()
	}
	if err := this.helper.CheckOType(obj); err != nil {
		return nil, err
	}
	result, err := this.self.I_create(obj)
	if err != nil {
		return nil, err
	}
	return this.helper.ObjectAsResource(result), nil
}

func (this *AbstractResource) CreateOrUpdate(obj ObjectData) (Object, error) {
	if o, ok := obj.(Object); ok {
		obj = o.Data()
	}
	if err := this.helper.CheckOType(obj); err != nil {
		return nil, err
	}
	result, err := this.self.I_create(obj)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			result, err = this.self.I_update(obj)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return this.helper.ObjectAsResource(result), nil
}

func (this *AbstractResource) Update(obj ObjectData) (Object, error) {
	if o, ok := obj.(Object); ok {
		obj = o.Data()
	}
	if err := this.helper.CheckOType(obj); err != nil {
		return nil, err
	}
	result, err := this.self.I_update(obj)
	if err != nil {
		return nil, err
	}
	return this.helper.ObjectAsResource(result), nil
}

func (this *AbstractResource) Delete(obj ObjectData) error {
	if o, ok := obj.(Object); ok {
		obj = o.Data()
	}
	if err := this.helper.CheckOType(obj); err != nil {
		return err
	}
	err := this.self.I_delete(obj)
	if err != nil {
		return err
	}
	return nil
}

func (this *AbstractResource) DeleteByName(obj ObjectDataName) error {
	return this.self.I_delete(obj)
}

func (this *AbstractResource) handleList(result runtime.Object) (ret []Object, err error) {
	v := reflect.ValueOf(result)
	iv := v.Elem().FieldByName("Items")
	if iv.Kind() != reflect.Slice {
		return nil, fmt.Errorf("unknown list format %s", iv.Type())
	}
	for i := 0; i < iv.Len(); i++ {
		ret = append(ret, this.helper.ObjectAsResource(iv.Index(i).Addr().Interface().(ObjectData)))
	}
	return ret, nil
}

func (this *AbstractResource) GetInto(name ObjectName, obj ObjectData) (Object, error) {
	if o, ok := obj.(Object); ok {
		obj = o.Data()
	}
	if err := this.helper.CheckOType(obj, true); err != nil {
		return nil, err
	}
	return this.helper.Get(name.Namespace(), name.Name(), obj)
}

func (this *AbstractResource) Get_(obj interface{}) (Object, error) {
	gvk := this.GroupVersionKind()
	switch o := obj.(type) {
	case string:
		if this.Namespaced() {
			return nil, fmt.Errorf("info %s is namespaced", gvk)
		}
		return this.helper.Get("", o, nil)
	case ObjectData:
		if err := this.helper.CheckOType(o); err != nil {
			return nil, err
		}
		return this.helper.Get(o.GetNamespace(), o.GetName(), o)
	case ObjectKey:
		if o.GroupKind() != this.GroupKind() {
			return nil, fmt.Errorf("%s cannot handle group/kind '%s'", gvk, o.GroupKind())
		}
		return this.helper.Get(o.Namespace(), o.Name(), nil)
	case *ObjectKey:
		if o.GroupKind() != this.GroupKind() {
			return nil, fmt.Errorf("%s cannot handle group/kind '%s'", gvk, o.GroupKind())
		}
		return this.helper.Get(o.Namespace(), o.Name(), nil)
	case ClusterObjectKey:
		if o.GroupKind() != this.GroupKind() {
			return nil, fmt.Errorf("%s cannot handle group/kind '%s'", gvk, o.GroupKind())
		}
		return this.helper.Get(o.Namespace(), o.Name(), nil)
	case *ClusterObjectKey:
		if o.GroupKind() != this.GroupKind() {
			return nil, fmt.Errorf("%s cannot handle group/kind '%s'", gvk, o.GroupKind())
		}
		return this.helper.Get(o.Namespace(), o.Name(), nil)
	case ObjectName:
		return this.helper.Get(o.Namespace(), o.Name(), nil)
	default:
		return nil, fmt.Errorf("unsupported type '%T' for source _object", obj)
	}
}

func (this *AbstractResource) List(opts metav1.ListOptions) (ret []Object, err error) {
	return this.self.I_list(metav1.NamespaceAll)
}

////////////////////////////////////////////////////////////////////////////////

func (this *namespacedResource) GetInto(name string, obj ObjectData) (ret Object, err error) {
	if o, ok := obj.(Object); ok {
		obj = o.Data()
	}
	if err := this.resource.helper.CheckOType(obj); err != nil {
		return nil, err
	}
	return this.resource.helper.Get(this.namespace, name, obj)
}

func (this *namespacedResource) Get(name string) (ret Object, err error) {
	return this.resource.helper.Get(this.namespace, name, nil)
}

func (this *namespacedResource) List(opts metav1.ListOptions) (ret []Object, err error) {
	if !this.resource.Namespaced() {
		return nil, fmt.Errorf("resourcename %s (%s) is not namespaced", this.resource.Name(), this.resource.GroupVersionKind())
	}
	return this.resource.self.I_list(this.namespace)
}
