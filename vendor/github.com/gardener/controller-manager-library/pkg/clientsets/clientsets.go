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

package clientsets

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/gardener/controller-manager-library/pkg/utils"
	"k8s.io/client-go/rest"
)

var (
	registry = map[reflect.Type]ClientsetFactory{}

	lock sync.Mutex
)

type ClientsetFactory interface {
	Create(config *rest.Config) (interface{}, error)
}

func MustRegister(keyspec interface{}, clientsetFactory ClientsetFactory) {
	if err := Register(keyspec, clientsetFactory); err != nil {
		panic(err)
	}
}

func Register(keyspec interface{}, clientsetFactory ClientsetFactory) error {
	lock.Lock()
	defer lock.Unlock()

	t, err := utils.TypeKey(keyspec)
	if err != nil {
		return err
	}
	if _, ok := registry[t]; ok {
		return fmt.Errorf("cannot register %s because already registered", t)
	}

	registry[t] = clientsetFactory
	return nil
}

type Interface interface {
	GetConfig() rest.Config
	Get(keyspec interface{}) (interface{}, error)
}

type clientsets struct {
	config     *rest.Config
	clientsets map[reflect.Type]interface{}
}

var _ Interface = &clientsets{}

func NewForConfig(config *rest.Config) Interface {
	return &clientsets{
		config:     config,
		clientsets: map[reflect.Type]interface{}{},
	}
}

func (c *clientsets) GetConfig() rest.Config {
	cfg := *c.config
	return cfg
}

func (c *clientsets) Get(keyspec interface{}) (interface{}, error) {
	lock.Lock()
	defer lock.Unlock()

	t, err := utils.TypeKey(keyspec)
	if err != nil {
		return nil, err
	}

	if clientset, ok := c.clientsets[t]; ok {
		return clientset, nil
	}

	if clientsetFactory, ok := registry[t]; ok {
		clientset, err := clientsetFactory.Create(c.config)
		if err != nil {
			return nil, err
		}

		c.clientsets[t] = clientset
		return clientset, nil
	}

	return nil, fmt.Errorf("%s not found in clientsets registry", t)
}
