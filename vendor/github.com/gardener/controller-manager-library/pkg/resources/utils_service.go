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
	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ServiceObject struct {
	Object
}

func (this *ServiceObject) Service() *api.Service {
	return this.Data().(*api.Service)
}

func ServiceKey(namespace, name string) ObjectKey {
	return NewKey(schema.GroupKind{api.GroupName, "Service"}, namespace, name)
}

func Service(o Object) *ServiceObject {
	if o.IsA(&api.Service{}) {
		return &ServiceObject{o}
	}
	return nil
}

func (this *ServiceObject) Status() *api.ServiceStatus {
	return &this.Service().Status
}

func (this *ServiceObject) Spec() *api.ServiceSpec {
	return &this.Service().Spec
}

func GetService(src ResourcesSource, namespace, name string) (*ServiceObject, error) {
	resources := src.Resources()
	o, err := resources.GetObjectInto(NewObjectName(namespace, name), &api.Service{})
	if err != nil {
		return nil, err
	}

	s := Service(o)
	if s == nil {
		return nil, fmt.Errorf("oops, unexpected typo for secret: %T", o.Data())
	}
	return s, nil
}

func GetCachedService(src ResourcesSource, namespace, name string) (*ServiceObject, error) {
	resource, err := src.Resources().Get(&api.Service{})
	if err != nil {
		return nil, err
	}
	o, err := resource.GetCached(NewObjectName(namespace, name))
	if err != nil {
		return nil, err
	}

	s := Service(o)
	if s == nil {
		return nil, fmt.Errorf("oops, unexpected typo for service: %T", o.Data())
	}
	return s, nil
}
