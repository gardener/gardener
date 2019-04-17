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
	"github.com/gardener/controller-manager-library/pkg/logger"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
)

func (this *_resource) _update(data ObjectData) (ObjectData, error) {
	logger.Infof("UPDATE %s/%s/%s", this.GroupKind(), data.GetNamespace(), data.GetName())
	result := this.createData()
	return result, this.objectRequest(this.client.Put(), data).
		Body(data).
		Do().
		Into(result)
}

func (this *_resource) _updateStatus(data ObjectData) (ObjectData, error) {
	logger.Infof("UPDATE STATUS %s/%s/%s", this.GroupKind(), data.GetNamespace(), data.GetName())
	result := this.createData()
	return result, this.objectRequest(this.client.Put(), data, "status").
		Body(data).
		Do().
		Into(result)
}

func (this *_resource) _create(data ObjectData) (ObjectData, error) {
	result := this.createData()
	return result, this.resourceRequest(this.client.Post(), data).
		Body(data).
		Do().
		Into(result)
}

func (this *_resource) _get(data ObjectData) error {
	return this.objectRequest(this.client.Get(), data).
		Do().
		Into(data)
}

func (this *_resource) _delete(data ObjectData) error {
	return this.objectRequest(this.client.Delete(), data).
		Body(&metav1.DeleteOptions{}).
		Do().
		Error()
}

func (this *_resource) Create(obj ObjectData) (Object, error) {
	if o, ok := obj.(Object); ok {
		obj = o.Data()
	}
	if err := this.checkOType(obj); err != nil {
		return nil, err
	}
	result, err := this._create(obj)
	if err != nil {
		return nil, err
	}
	return this.objectAsResource(result), nil
}

func (this *_resource) CreateOrUpdate(obj ObjectData) (Object, error) {
	if o, ok := obj.(Object); ok {
		obj = o.Data()
	}
	if err := this.checkOType(obj); err != nil {
		return nil, err
	}
	result, err := this._create(obj)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			result, err = this._update(obj)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return this.objectAsResource(result), nil
}

func (this *_resource) Update(obj ObjectData) (Object, error) {
	if o, ok := obj.(Object); ok {
		obj = o.Data()
	}
	if err := this.checkOType(obj); err != nil {
		return nil, err
	}
	result, err := this._update(obj)
	if err != nil {
		return nil, err
	}
	return this.objectAsResource(result), nil
}

func (this *_resource) Delete(obj ObjectData) error {
	if o, ok := obj.(Object); ok {
		obj = o.Data()
	}
	if err := this.checkOType(obj); err != nil {
		return err
	}
	err := this._delete(obj)
	if err != nil {
		return err
	}
	return nil
}

func (this *_resource) get(namespace, name string, result ObjectData) (Object, error) {
	if !this.Namespaced() && namespace != "" {
		return nil, fmt.Errorf("%s is not namespaced", this.GroupKind())
	}
	if this.Namespaced() && namespace == "" {
		return nil, fmt.Errorf("%s is namespaced", this.GroupKind())
	}

	if result == nil {
		result = this.createData()
	}
	err := this.namespacedRequest(this.client.Get(), namespace).
		Name(name).
		Do().
		Into(result)
	return this.objectAsResource(result), err
}

func (this *_resource) list(namespace string) ([]Object, error) {
	result := this.createListData()
	err := this.namespacedRequest(this.client.Get(), namespace).
		Do().
		Into(result)
	if err != nil {
		return nil, err
	}
	return this.handleList(result)
}

func (this *_resource) handleList(result runtime.Object) (ret []Object, err error) {
	v := reflect.ValueOf(result)
	iv := v.Elem().FieldByName("Items")
	if iv.Kind() != reflect.Slice {
		return nil, fmt.Errorf("unknown list format %s", iv.Type())
	}
	for i := 0; i < iv.Len(); i++ {
		ret = append(ret, this.objectAsResource(iv.Index(i).Addr().Interface().(ObjectData)))
	}
	return ret, nil
}

func (this *_resource) GetInto(name ObjectName, obj ObjectData) (Object, error) {
	if o, ok := obj.(Object); ok {
		obj = o.Data()
	}
	if err := this.checkOType(obj); err != nil {
		return nil, err
	}
	return this.get(name.Namespace(), name.Name(), obj)
}

func (this *_resource) Get_(obj interface{}) (Object, error) {
	switch o := obj.(type) {
	case string:
		if this.Namespaced() {
			return nil, fmt.Errorf("info %s is namespaced", this.gvk)
		}
		return this.get("", o, nil)
	case ObjectData:
		if err := this.checkOType(o); err != nil {
			return nil, err
		}
		return this.get(o.GetNamespace(), o.GetName(), o)
	case ObjectKey:
		if o.GroupKind() != this.GroupKind() {
			return nil, fmt.Errorf("%s cannot handle group/kind '%s'", this.gvk, o.GroupKind())
		}
		return this.get(o.Namespace(), o.Name(), nil)
	case *ObjectKey:
		if o.GroupKind() != this.GroupKind() {
			return nil, fmt.Errorf("%s cannot handle group/kind '%s'", this.gvk, o.GroupKind())
		}
		return this.get(o.Namespace(), o.Name(), nil)
	case ClusterObjectKey:
		if o.GroupKind() != this.GroupKind() {
			return nil, fmt.Errorf("%s cannot handle group/kind '%s'", this.gvk, o.GroupKind())
		}
		return this.get(o.Namespace(), o.Name(), nil)
	case *ClusterObjectKey:
		if o.GroupKind() != this.GroupKind() {
			return nil, fmt.Errorf("%s cannot handle group/kind '%s'", this.gvk, o.GroupKind())
		}
		return this.get(o.Namespace(), o.Name(), nil)
	case ObjectName:
		return this.get(o.Namespace(), o.Name(), nil)
	default:
		return nil, fmt.Errorf("unsupported type '%T' for source _object", obj)
	}
}

func (this *_resource) List(opts metav1.ListOptions) (ret []Object, err error) {
	return this.list(metav1.NamespaceAll)
}

////////////////////////////////////////////////////////////////////////////////

func (this *namespacedResource) GetInto(name string, obj ObjectData) (ret Object, err error) {
	if o, ok := obj.(Object); ok {
		obj = o.Data()
	}
	if err := this.resource.checkOType(obj); err != nil {
		return nil, err
	}
	return this.resource.get(this.namespace, name, obj)
}

func (this *namespacedResource) Get(name string) (ret Object, err error) {
	return this.resource.get(this.namespace, name, nil)
}

func (this *namespacedResource) List(opts metav1.ListOptions) (ret []Object, err error) {
	if !this.resource.Namespaced() {
		return nil, fmt.Errorf("resourcename %s (%s) is not namespaced", this.resource.Name(), this.resource.GroupVersionKind())
	}
	return this.resource.list(this.namespace)
}
