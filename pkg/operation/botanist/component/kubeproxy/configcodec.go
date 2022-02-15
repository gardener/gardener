// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubeproxy

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubeproxyconfigv1alpha1 "k8s.io/kube-proxy/config/v1alpha1"
)

// ConfigCodec contains methods for encoding and decoding *kubeproxyconfigv1alpha1.KubeProxyConfiguration objects
// to and from string.
type ConfigCodec interface {
	// Encode encodes the given *kubeproxyconfigv1alpha1.KubeProxyConfiguration into a string.
	Encode(*kubeproxyconfigv1alpha1.KubeProxyConfiguration) (string, error)
	// Decode decodes a *kubeproxyconfigv1alpha1.KubeProxyConfiguration from the given string.
	Decode(string) (*kubeproxyconfigv1alpha1.KubeProxyConfiguration, error)
}

var scheme *runtime.Scheme

func init() {
	scheme = runtime.NewScheme()
	utilruntime.Must(kubeproxyconfigv1alpha1.AddToScheme(scheme))
}

// NewConfigCodec creates an returns a new ConfigCodec.
func NewConfigCodec() ConfigCodec {
	ser := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
	versions := schema.GroupVersions([]schema.GroupVersion{kubeproxyconfigv1alpha1.SchemeGroupVersion})
	codec := serializer.NewCodecFactory(scheme).CodecForVersions(ser, ser, versions, versions)

	return &kubeProxyConfigCodec{
		codec: codec,
	}
}

type kubeProxyConfigCodec struct {
	codec runtime.Codec
}

// Encode encodes the given *kubeproxyconfigv1alpha1.KubeProxyConfiguration into a string.
func (c *kubeProxyConfigCodec) Encode(kubeProxyConfig *kubeproxyconfigv1alpha1.KubeProxyConfiguration) (string, error) {
	data, err := runtime.Encode(c.codec, kubeProxyConfig)
	if err != nil {
		return "", fmt.Errorf("could not encode kube-proxy configuration to YAML: %w", err)
	}

	return string(data), nil
}

// Decode decodes a *kubeproxyconfigv1alpha1.KubeProxyConfiguration from the given string.
func (c *kubeProxyConfigCodec) Decode(data string) (*kubeproxyconfigv1alpha1.KubeProxyConfiguration, error) {
	kubeProxyConfig := &kubeproxyconfigv1alpha1.KubeProxyConfiguration{}
	if err := runtime.DecodeInto(c.codec, []byte(data), kubeProxyConfig); err != nil {
		return nil, fmt.Errorf("could not decode kube-proxy configuration from YAML: %w", err)
	}

	return kubeProxyConfig, nil
}
