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
	"k8s.io/apimachinery/pkg/api/errors"
	"reflect"
	"sync"

	"github.com/gardener/controller-manager-library/pkg/informerfactories"

	"github.com/gardener/controller-manager-library/pkg/logger"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Internal interface {
	Interface

	I_objectType() reflect.Type
	I_listType() reflect.Type

	I_create(data ObjectData) (ObjectData, error)
	I_get(data ObjectData) error
	I_update(data ObjectData) (ObjectData, error)
	I_updateStatus(data ObjectData) (ObjectData, error)
	I_delete(data ObjectDataName) error

	I_modifyByName(name ObjectDataName, status_only, create bool, modifier Modifier) (Object, bool, error)
	I_modify(data ObjectData, status_only, read, create bool, modifier Modifier) (ObjectData, bool, error)

	I_getInformer(namespace string, optionsFunc TweakListOptionsFunc) (GenericInformer, error)
	I_lookupInformer(namespace string) (GenericInformer, error)
	I_list(namespace string, opts metav1.ListOptions) ([]Object, error)
}

// _i_resource is the implementation of the internal resource interface used by
// the abstract object.
// To avoid to potentially expose those additional methods the resource
// implementation does NOT implement the internal interface. Instead,
// it uses an internal wrapper object, that implements this interface.
// This wrapper is then passed to the abstract resource implementation
// to be used to implement a broader set of methods in a generic manner.

type _i_resource struct {
	*_resource
	lock  sync.Mutex
	cache GenericInformer
}

var _ Internal = &_i_resource{}

func (this *_i_resource) I_objectType() reflect.Type {
	return this.otype
}
func (this *_i_resource) I_listType() reflect.Type {
	return this.ltype
}

func (this *_i_resource) I_update(data ObjectData) (ObjectData, error) {
	logger.Infof("UPDATE %s/%s/%s", this.GroupKind(), data.GetNamespace(), data.GetName())
	result := this.helper.CreateData()
	return result, this.objectRequest(this.client.Put(), data).
		Body(data).
		Do().
		Into(result)
}

func (this *_i_resource) I_updateStatus(data ObjectData) (ObjectData, error) {
	logger.Infof("UPDATE STATUS %s/%s/%s", this.GroupKind(), data.GetNamespace(), data.GetName())
	result := this.helper.CreateData()
	return result, this.objectRequest(this.client.Put(), data, "status").
		Body(data).
		Do().
		Into(result)
}

func (this *_i_resource) I_create(data ObjectData) (ObjectData, error) {
	result := this.helper.CreateData()
	return result, this.resourceRequest(this.client.Post(), data).
		Body(data).
		Do().
		Into(result)
}

func (this *_i_resource) I_get(data ObjectData) error {
	return this.objectRequest(this.client.Get(), data).
		Do().
		Into(data)
}

func (this *_i_resource) I_delete(data ObjectDataName) error {
	return this.objectRequest(this.client.Delete(), data).
		Body(&metav1.DeleteOptions{}).
		Do().
		Error()
}

func (this *_i_resource) I_getInformer(namespace string, optionsFunc TweakListOptionsFunc) (GenericInformer, error) {
	if this.cache != nil {
		return this.cache, nil
	}
	this.lock.Lock()
	defer this.lock.Unlock()

	if this.cache != nil {
		return this.cache, nil
	}

	informers := this.context.SharedInformerFactory().Structured()
	if this.IsUnstructured() {
		informers = this.context.SharedInformerFactory().Unstructured()
	}
	informer, err := informers.FilteredInformerFor(this.gvk, namespace, optionsFunc)
	if err != nil {
		return nil, err
	}
	if err := informerfactories.Start(this.context.ctx, informers, informer.Informer().HasSynced); err != nil {
		return nil, err
	}

	if namespace == "" && optionsFunc == nil {
		this.cache = informer
	}
	return informer, nil
}

func (this *_i_resource) I_lookupInformer(namespace string) (GenericInformer, error) {
	if this.cache != nil {
		return this.cache, nil
	}
	this.lock.Lock()
	defer this.lock.Unlock()

	if this.cache != nil {
		return this.cache, nil
	}

	informers := this.context.SharedInformerFactory().Structured()
	if this.IsUnstructured() {
		informers = this.context.SharedInformerFactory().Unstructured()
	}
	informer, err := informers.LookupInformerFor(this.gvk, namespace)
	if err != nil {
		return nil, err
	}
	if err := informerfactories.Start(this.context.ctx, informers, informer.Informer().HasSynced); err != nil {
		return nil, err
	}

	return informer, nil
}

func (this *_i_resource) I_list(namespace string, options metav1.ListOptions) ([]Object, error) {
	result := this.helper.CreateListData()
	err := this.namespacedRequest(this.client.Get(), namespace).VersionedParams(&options, this.GetParameterCodec()).
		Do().
		Into(result)
	if err != nil {
		return nil, err
	}
	return this.handleList(result)
}

func (this *_i_resource) I_modifyByName(name ObjectDataName, status_only, create bool, modifier Modifier) (Object, bool, error) {
	data := this.helper.CreateData()
	data.SetName(name.GetName())
	data.SetNamespace(name.GetNamespace())

	data, mod, err := this.I_modify(data, status_only, true, create, modifier)
	if err == nil {
		return this.helper.ObjectAsResource(data), mod, err
	}
	return nil, mod, err
}

func (this *_i_resource) I_modify(data ObjectData, status_only, read, create bool, modifier Modifier) (ObjectData, bool, error) {
	var lasterr error
	var err error

	if read {
		err = this.I_get(data)
	}

	cnt := 10

	if create {
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, false, err
			}
			_, err := modifier(data)
			if err != nil {
				return nil, false, err
			}
			created, err := this.I_create(data)
			if err == nil {
				return created, true, nil
			}
			if !errors.IsAlreadyExists(err) {
				return nil, false, err
			}
			err = this.I_get(data)
			if err != nil {
				return nil, false, err
			}
		}
	}

	for cnt > 0 {
		mod, err := modifier(data)
		if !mod {
			if err == nil {
				return data, mod, err
			}
			return nil, mod, err
		}
		if err == nil {
			var modified ObjectData
			if status_only {
				modified, lasterr = this.I_updateStatus(data)
			} else {
				modified, lasterr = this.I_update(data)
			}
			if lasterr == nil {
				return modified, mod, nil
			}
			if !errors.IsConflict(lasterr) {
				return nil, mod, lasterr
			}
			err = this.I_get(data)
		}
		if err != nil {
			return nil, mod, err
		}
		cnt--
	}
	return data, true, lasterr
}
