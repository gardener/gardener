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
	"github.com/gardener/controller-manager-library/pkg/utils"
	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type SecretObject struct {
	Object
}

func (this *SecretObject) Secret() *api.Secret {
	return this.Data().(*api.Secret)
}

func SecretKey(namespace, name string) ObjectKey {
	return NewKey(schema.GroupKind{api.GroupName, "Secret"}, namespace, name)
}
func SecretKeyByRef(ref *api.SecretReference) ObjectKey {
	return SecretKey(ref.Namespace, ref.Name)
}

func Secret(o Object) *SecretObject {
	if o.IsA(&api.Secret{}) {
		return &SecretObject{o}
	}
	return nil
}

func GetSecret(src ResourcesSource, namespace, name string) (*SecretObject, error) {
	o, err := src.Resources().GetObjectInto(NewObjectName(namespace, name), &api.Secret{})
	if err != nil {
		return nil, err
	}

	s := Secret(o)
	if s == nil {
		return nil, fmt.Errorf("oops, unexpected type for secret: %T", o.Data())
	}
	return s, nil
}

func GetCachedSecret(src ResourcesSource, namespace, name string) (*SecretObject, error) {
	resource, err := src.Resources().Get(&api.Secret{})
	if err != nil {
		return nil, err
	}
	o, err := resource.GetCached(NewObjectName(namespace, name))
	if err != nil {
		return nil, err
	}

	s := Secret(o)
	if s == nil {
		return nil, fmt.Errorf("oops, unexpected typo for secret: %T", o.Data())
	}
	return s, nil
}

func GetSecretByRef(src ResourcesSource, ref *api.SecretReference) (*SecretObject, error) {
	return GetSecret(src, ref.Namespace, ref.Name)
}

func GetCachedSecretByRef(src ResourcesSource, ref *api.SecretReference) (*SecretObject, error) {
	return GetCachedSecret(src, ref.Namespace, ref.Name)
}

func GetSecretProperties(src ResourcesSource, namespace, name string) (utils.Properties, *SecretObject, error) {
	secret, err := GetSecret(src, namespace, name)
	if err != nil {
		return nil, nil, err
	}
	props := GetSecretPropertiesFrom(secret.Secret())
	return props, secret, nil
}

func GetCachedSecretProperties(src ResourcesSource, namespace, name string) (utils.Properties, *SecretObject, error) {
	secret, err := GetCachedSecret(src, namespace, name)
	if err != nil {
		return nil, nil, err
	}
	props := GetSecretPropertiesFrom(secret.Secret())
	return props, secret, nil
}

func GetSecretPropertiesFrom(secret *api.Secret) utils.Properties {
	props := utils.Properties{}
	for k, v := range secret.Data {
		props[k] = string(v)
	}
	return props
}

func GetSecretPropertiesByRef(src ResourcesSource, ref *api.SecretReference) (utils.Properties, *SecretObject, error) {
	return GetSecretProperties(src, ref.Namespace, ref.Name)
}

func GetCachedSecretPropertiesByRef(src ResourcesSource, ref *api.SecretReference) (utils.Properties, *SecretObject, error) {
	return GetCachedSecretProperties(src, ref.Namespace, ref.Name)
}

func (this *SecretObject) GetData() map[string][]byte {
	return this.Secret().Data
}
