// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package proxy

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
