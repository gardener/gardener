/*
 * Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package common

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/chartrenderer"
)

// ClientContext bundles the feature of providing injected scheme and client for
// the controller runtime. Additionally it offers a decoder using the scheme.
type ClientContext struct {
	scheme  *runtime.Scheme
	decoder runtime.Decoder
	client  client.Client
}

// NewClientContext offers the possibility to create a ClientContext without injection.
func NewClientContext(client client.Client, scheme *runtime.Scheme, decoder runtime.Decoder) ClientContext {
	if decoder == nil && scheme != nil {
		decoder = serializer.NewCodecFactory(scheme, serializer.EnableStrict).UniversalDecoder()
	}
	return ClientContext{client: client, scheme: scheme, decoder: decoder}
}

// Scheme returns the scheme of the context
func (cc *ClientContext) Scheme() *runtime.Scheme {
	return cc.scheme
}

// Decoder returns a decoder for the scheme of the context
func (cc *ClientContext) Decoder() runtime.Decoder {
	return cc.decoder
}

// Client returns the rest client of the context
func (cc *ClientContext) Client() client.Client {
	return cc.client
}

////////////////////////////////////////////////////////////////////////////////

// RESTConfigContext extends the ClientContext with the REST config
// usable to create more specific clients.
type RESTConfigContext struct {
	ClientContext
	restConfig *rest.Config
}

// NewRESTConfigContext creates a ClientContext with the REST config.
func NewRESTConfigContext(mgr manager.Manager) RESTConfigContext {
	return RESTConfigContext{
		ClientContext: NewClientContext(mgr.GetClient(), mgr.GetScheme(), nil),
		restConfig:    mgr.GetConfig(),
	}
}

// RESTConfig returns the rest config of the context
func (cc *RESTConfigContext) RESTConfig() *rest.Config {
	return cc.restConfig
}

////////////////////////////////////////////////////////////////////////////////

// ChartRendererContext extends the RESTConfigContext to additionally
// provide a chart renderer
type ChartRendererContext struct {
	RESTConfigContext
	factory       chartrenderer.Factory
	chartRenderer chartrenderer.Interface
}

// NewChartRendererContext creates a new chart renderer context using a
// dedicated factory for the renderer,
func NewChartRendererContext(mgr manager.Manager, factory chartrenderer.Factory, chartRenderer chartrenderer.Interface) ChartRendererContext {
	return ChartRendererContext{
		RESTConfigContext: NewRESTConfigContext(mgr),
		factory:           factory,
		chartRenderer:     chartRenderer,
	}
}

// ChartRenderer returns the chart renderer of the context
func (cc *ChartRendererContext) ChartRenderer() chartrenderer.Interface {
	return cc.chartRenderer
}
