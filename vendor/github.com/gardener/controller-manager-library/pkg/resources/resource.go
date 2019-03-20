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
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
	"sync"

	"github.com/gardener/controller-manager-library/pkg/informerfactories"
	"github.com/gardener/controller-manager-library/pkg/logger"

	"k8s.io/apimachinery/pkg/runtime/schema"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type Internal interface {
	Interface
	getClient() restclient.Interface
	namespacedRequest(*restclient.Request, string) *restclient.Request
	resourceRequest(*restclient.Request, ObjectData, ...string) *restclient.Request
	objectRequest(*restclient.Request, ObjectData, ...string) *restclient.Request
	objectType() reflect.Type
	createData() ObjectData

	_create(data ObjectData) (ObjectData, error)
	_get(data ObjectData) error
	_update(data ObjectData) (ObjectData, error)
	_updateStatus(data ObjectData) (ObjectData, error)
	_delete(data ObjectData) error
}

type _resource struct {
	lock    sync.Mutex
	context *resourceContext
	gvk     schema.GroupVersionKind
	otype   reflect.Type
	ltype   reflect.Type
	info    *Info
	cache   GenericInformer
	client  restclient.Interface
}

var _ Interface = &_resource{}

type namespacedResource struct {
	resource  *_resource
	namespace string
	lister    NamespacedLister
}

/////////////////////////////////////////////////////////////////////////////////

func (this *_resource) GetCluster() Cluster {
	return this.context.cluster
}

func (this *_resource) getInformer() (GenericInformer, error) {
	if this.cache != nil {
		return this.cache, nil
	}
	this.lock.Lock()
	defer this.lock.Unlock()

	if this.cache != nil {
		return this.cache, nil
	}

	informers := this.context.SharedInformerFactory()
	informer, err := informers.InformerFor(this.gvk)
	if err != nil {
		return nil, err
	}
	if err := informerfactories.Start(this.context.ctx, informers, informer.Informer().HasSynced); err != nil {
		return nil, err
	}

	this.cache = informer
	return this.cache, nil
}

func (this *_resource) objectType() reflect.Type {
	return this.otype
}

func (this *_resource) objectAsResource(obj ObjectData) Object {
	return &_object{obj, this.context.cluster, this}
}

func (this *_resource) GroupVersionKind() schema.GroupVersionKind {
	return this.gvk
}

func (this *_resource) Name() string {
	return this.info.Name()
}

func (this *_resource) Info() *Info {
	return this.info
}

func (this *_resource) Client() restclient.Interface {
	return this.client
}

func (this *_resource) Namespaced() bool {
	return this.info.Namespaced()
}

func (this *_resource) getClient() restclient.Interface {
	return this.client
}
func (this *_resource) ResourceContext() ResourceContext {
	return this.context
}

func (this *_resource) GroupKind() schema.GroupKind {
	return this.gvk.GroupKind()
}

func (this *_resource) AddRawEventHandler(handlers cache.ResourceEventHandlerFuncs) error {
	logger.Infof("adding resourcename for %s", this.gvk)
	informer, err := this.getInformer()
	if err != nil {
		return err
	}
	informer.AddEventHandler(&handlers)
	return nil
}

func (this *_resource) AddEventHandler(handlers ResourceEventHandlerFuncs) error {
	return this.AddRawEventHandler(*convert(this, &handlers))
}

func (this *_resource) Namespace(namespace string) Namespaced {
	return &namespacedResource{this, namespace, nil}
}

func (this *_resource) checkOType(obj ObjectData) error {
	t := reflect.TypeOf(obj)
	if t.Kind() == reflect.Ptr && t.Elem() == this.otype {
		return nil
	}
	return fmt.Errorf("wrong data type %T (expected %s)", obj, reflect.PtrTo(this.otype))
}

func (this *_resource) createData() ObjectData {
	return reflect.New(this.otype).Interface().(ObjectData)
}

func (this *_resource) createListData() runtime.Object {
	return reflect.New(this.ltype).Interface().(runtime.Object)
}

func (this *_resource) namespacedRequest(req *restclient.Request, namespace string) *restclient.Request {
	if this.Namespaced() {
		return req.Namespace(namespace).Resource(this.Name())
	}
	return req.Resource(this.Name())
}

func (this *_resource) resourceRequest(req *restclient.Request, obj ObjectData, sub ...string) *restclient.Request {
	if this.Namespaced() && obj != nil {
		req = req.Namespace(obj.GetNamespace())
	}
	return req.Resource(this.Name()).SubResource(sub...)
}

func (this *_resource) objectRequest(req *restclient.Request, obj ObjectData, sub ...string) *restclient.Request {
	return this.resourceRequest(req, obj, sub...).Name(obj.GetName())
}

func (this *_resource) Wrap(obj ObjectData) (Object, error) {
	if err := this.checkOType(obj); err != nil {
		return nil, err
	}
	return this.objectAsResource(obj), nil
}

func (this *_resource) New(name ObjectName) Object {
	data := this.createData()
	data.GetObjectKind().SetGroupVersionKind(this.gvk)
	if name != nil {
		data.SetName(name.Name())
		data.SetNamespace(name.Namespace())
	}
	return this.objectAsResource(data)
}
