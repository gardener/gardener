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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"sync"
)

type Clients struct {
	lock           sync.Mutex
	scheme         *runtime.Scheme
	config         restclient.Config
	codecfactory   serializer.CodecFactory
	parametercodec runtime.ParameterCodec
	clients        map[schema.GroupVersion]restclient.Interface
}

func NewClients(config restclient.Config, scheme *runtime.Scheme) *Clients {
	client := &Clients{
		config:         config,
		scheme:         scheme,
		clients:        map[schema.GroupVersion]restclient.Interface{},
		codecfactory:   serializer.NewCodecFactory(scheme),
		parametercodec: runtime.NewParameterCodec(scheme),
	}
	client.config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: client.codecfactory}
	return client
}

func (c *Clients) NewFor(config restclient.Config) *Clients {
	return NewClients(config, c.scheme)
}

func (c *Clients) GetCodecFactory() serializer.CodecFactory {
	return c.codecfactory
}

func (c *Clients) GetParameterCodec() runtime.ParameterCodec {
	return c.parametercodec
}

func (c *Clients) GetClient(gv schema.GroupVersion) (restclient.Interface, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	var err error
	client := c.clients[gv]
	if client == nil {
		config := c.config
		config.GroupVersion = &gv
		if gv.Group == "" {
			config.APIPath = "/api"

		} else {
			config.APIPath = "/apis"
		}

		if config.UserAgent == "" {
			config.UserAgent = rest.DefaultKubernetesUserAgent()
		}

		client, err = restclient.RESTClientFor(&config)
		if err != nil {
			return nil, err
		}
		c.clients[gv] = client
	}
	return client, nil
}
