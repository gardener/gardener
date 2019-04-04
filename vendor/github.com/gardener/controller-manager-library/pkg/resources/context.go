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
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gardener/controller-manager-library/pkg/ctxutil"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	restclient "k8s.io/client-go/rest"
)

type ResourceContext interface {
	GetGroups() []schema.GroupVersion
	GetResourceInfos(gv schema.GroupVersion) []*Info

	GetClient(gv schema.GroupVersion) (restclient.Interface, error)

	Resources() Resources
	SharedInformerFactory() SharedInformerFactory

	GetPreferred(gk schema.GroupKind) (*Info, error)
	Get(gvk schema.GroupVersionKind) (*Info, error)
}

type resourceContext struct {
	*ResourceInfos
	*Clients
	Cluster
	*runtime.Scheme

	lock                  sync.Mutex
	ctx                   context.Context
	defaultResync         time.Duration
	resources             *_resources
	sharedInformerFactory *sharedInformerFactory
}

func NewResourceContext(ctx context.Context, c Cluster, scheme *runtime.Scheme, defaultResync time.Duration) (ResourceContext, error) {
	ctx = ctxutil.CancelContext(ctx)
	if scheme == nil {
		scheme = DefaultScheme()
	}
	res, err := NewResourceInfos(c)
	if err != nil {
		return nil, err
	}
	return &resourceContext{
		Scheme:        scheme,
		Cluster:       c,
		ResourceInfos: res,
		Clients:       NewClients(c.Config(), scheme),
		ctx:           ctx,
		defaultResync: defaultResync,
	}, nil

}

func (c *resourceContext) GetGVK(obj runtime.Object) (schema.GroupVersionKind, error) {
	var empty schema.GroupVersionKind

	gvks, _, err := c.ObjectKinds(obj)
	if err != nil {
		return empty, err
	}
	switch len(gvks) {
	case 0:
		return empty, fmt.Errorf("%T unknown info type", obj)
	case 1:
		return gvks[0], nil
	default:
		for _, gvk := range gvks {
			def, err := c.GetPreferred(gvk.GroupKind())
			if err != nil {
				return empty, err
			}
			if def.Version() == gvk.Version {
				return gvk, nil
			}
		}
	}
	return empty, fmt.Errorf("non.unique mapping for %T", obj)
}

// NewSharedInformerFactory constructs a new instance of sharedInformerFactory for all namespaces.
func (c *resourceContext) SharedInformerFactory() SharedInformerFactory {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.sharedInformerFactory == nil {
		c.sharedInformerFactory = newSharedInformerFactory(c, c.defaultResync)
	}
	return c.sharedInformerFactory
}

func (c *resourceContext) Resources() Resources {
	c.SharedInformerFactory()

	c.lock.Lock()
	defer c.lock.Unlock()

	if c.resources == nil {
		source := "controller"
		src := c.ctx.Value(ATTR_EVENTSOURCE)
		if src != nil {
			source = src.(string)
		}
		c.resources = newResources(c, source)
	}
	return c.resources
}
