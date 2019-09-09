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
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/gardener/controller-manager-library/pkg/logger"

	"k8s.io/apimachinery/pkg/runtime/schema"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type _resource struct {
	AbstractResource
	context *resourceContext
	gvk     schema.GroupVersionKind
	otype   reflect.Type
	ltype   reflect.Type
	info    *Info
	client  restclient.Interface
}

var _ Interface = &_resource{}

type namespacedResource struct {
	resource  *AbstractResource
	namespace string
	lister    NamespacedLister
}

func newResource(
	context *resourceContext,
	otype reflect.Type,
	ltype reflect.Type,
	info *Info,
	client restclient.Interface) Interface {

	r := &_resource{
		AbstractResource: AbstractResource{},
		context:          context,
		gvk:              info.GroupVersionKind(),
		otype:            otype,
		ltype:            ltype,
		info:             info,
		client:           client,
	}
	r.AbstractResource, _ = NewAbstractResource(&_i_resource{_resource: r})
	return r
}

/////////////////////////////////////////////////////////////////////////////////

func (this *_resource) GetCluster() Cluster {
	return this.context.cluster
}

func (this *_resource) Resources() Resources {
	return this.context.Resources()
}

var unstructuredType = reflect.TypeOf(unstructured.Unstructured{})
var unstructuredListType = reflect.TypeOf(unstructured.UnstructuredList{})

func (this *_resource) IsUnstructured() bool {
	return this.otype == unstructuredType
}

func (this *_resource) Info() *Info {
	return this.info
}

func (this *_resource) Client() restclient.Interface {
	return this.client
}

func (this *_resource) ResourceContext() ResourceContext {
	return this.context
}

func (this *_resource) AddRawEventHandler(handlers cache.ResourceEventHandlerFuncs) error {
	logger.Infof("adding resourcename for %s", this.gvk)
	informer, err := this.self.I_getInformer()
	if err != nil {
		return err
	}
	informer.AddEventHandler(&handlers)
	return nil
}

func (this *_resource) AddEventHandler(handlers ResourceEventHandlerFuncs) error {
	return this.AddRawEventHandler(*convert(this, &handlers))
}

func (this *_resource) namespacedRequest(req *restclient.Request, namespace string) *restclient.Request {
	if this.Namespaced() {
		return req.Namespace(namespace).Resource(this.Name())
	}
	return req.Resource(this.Name())
}

func (this *_resource) resourceRequest(req *restclient.Request, obj ObjectDataName, sub ...string) *restclient.Request {
	if this.Namespaced() && obj != nil {
		req = req.Namespace(obj.GetNamespace())
	}
	return req.Resource(this.Name()).SubResource(sub...)
}

func (this *_resource) objectRequest(req *restclient.Request, obj ObjectDataName, sub ...string) *restclient.Request {
	return this.resourceRequest(req, obj, sub...).Name(obj.GetName())
}
