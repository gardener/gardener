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
	api "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type IngressObject struct {
	Object
}

func (this *IngressObject) Ingress() *api.Ingress {
	return this.Data().(*api.Ingress)
}

func IngressKey(namespace, name string) ObjectKey {
	return NewKey(schema.GroupKind{api.GroupName, "Ingress"}, namespace, name)
}

func Ingress(o Object) *IngressObject {
	if o.IsA(&api.Ingress{}) {
		return &IngressObject{o}
	}
	return nil
}

func GetIngress(src ResourcesSource, namespace, name string) (*IngressObject, error) {
	resources := src.Resources()
	o, err := resources.GetObjectInto(NewObjectName(namespace, name), &api.Ingress{})
	if err != nil {
		return nil, err
	}

	s := Ingress(o)
	if s == nil {
		return nil, fmt.Errorf("oops, unexpected typo for secret: %T", o.Data())
	}
	return s, nil
}

func GetCachedIngress(src ResourcesSource, namespace, name string) (*IngressObject, error) {
	resource, err := src.Resources().Get(&api.Ingress{})
	if err != nil {
		return nil, err
	}
	o, err := resource.GetCached(NewObjectName(namespace, name))
	if err != nil {
		return nil, err
	}

	s := Ingress(o)
	if s == nil {
		return nil, fmt.Errorf("oops, unexpected type for ingress: %T", o.Data())
	}
	return s, nil
}
