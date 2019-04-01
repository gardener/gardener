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

package informerfactories

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/gardener/controller-manager-library/pkg/clientsets"
	"github.com/gardener/controller-manager-library/pkg/utils"
)

var (
	registry = map[reflect.Type]SharedInformerFactoryFactory{}

	lock sync.Mutex
)

type SharedInformerFactoryFactory interface {
	Create(clientsets clientsets.Interface) (interface{}, error)
}

func MustRegister(key interface{}, sharedInformerFactoryFactory SharedInformerFactoryFactory) {
	if err := Register(key, sharedInformerFactoryFactory); err != nil {
		panic(err)
	}
}

func Register(keyspec interface{}, sharedInformerFactoryFactory SharedInformerFactoryFactory) error {
	lock.Lock()
	defer lock.Unlock()

	t, err := utils.TypeKey(keyspec)
	if err != nil {
		return err
	}
	if _, ok := registry[t]; ok {
		return fmt.Errorf("cannot register %s because already registered", t)
	}

	registry[t] = sharedInformerFactoryFactory
	return nil
}

type Interface interface {
	Get(keyspec interface{}) (interface{}, error)
	GetClientset(keyspec interface{}) (interface{}, error)
}

type sharedInfomerFactories struct {
	clientsets             clientsets.Interface
	sharedInfomerFactories map[reflect.Type]interface{}
}

var _ Interface = &sharedInfomerFactories{}

func NewForClientsets(clientsets clientsets.Interface) Interface {
	return &sharedInfomerFactories{
		clientsets:             clientsets,
		sharedInfomerFactories: map[reflect.Type]interface{}{},
	}
}

func (c *sharedInfomerFactories) Get(keyspec interface{}) (interface{}, error) {
	lock.Lock()
	defer lock.Unlock()

	t, err := utils.TypeKey(keyspec)
	if err != nil {
		return nil, err
	}
	if sharedInformerFactory, ok := c.sharedInfomerFactories[t]; ok {
		return sharedInformerFactory, nil
	}

	if sharedInformerFactoryFactory, ok := registry[t]; ok {
		sharedInformerFactory, err := sharedInformerFactoryFactory.Create(c.clientsets)
		if err != nil {
			return nil, err
		}

		c.sharedInfomerFactories[t] = sharedInformerFactory
		return sharedInformerFactory, nil
	}

	return nil, fmt.Errorf("%T not found in informerfactories registry", t)
}

func (c *sharedInfomerFactories) GetClientset(keyspec interface{}) (interface{}, error) {
	return c.clientsets.Get(keyspec)
}
