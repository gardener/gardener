// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controlplane

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
)

// KubeletConfigCodec contains methods for encoding and decoding *kubeletconfigv1beta1.KubeletConfiguration objects
// to and from *extensionsv1alpha1.FileContentInline.
type KubeletConfigCodec interface {
	// Encode encodes the given *kubeletconfigv1beta1.KubeletConfiguration into a *extensionsv1alpha1.FileContentInline.
	Encode(*kubeletconfigv1beta1.KubeletConfiguration, string) (*extensionsv1alpha1.FileContentInline, error)
	// Decode decodes a *kubeletconfigv1beta1.KubeletConfiguration from the given *extensionsv1alpha1.FileContentInline.
	Decode(*extensionsv1alpha1.FileContentInline) (*kubeletconfigv1beta1.KubeletConfiguration, error)
}

var scheme *runtime.Scheme

func init() {
	// Create and initialize scheme
	scheme = runtime.NewScheme()
	utilruntime.Must(kubeletconfigv1beta1.AddToScheme(scheme))
}

// NewKubeletConfigCodec creates an returns a new KubeletConfigCodec.
func NewKubeletConfigCodec(fciCodec FileContentInlineCodec) KubeletConfigCodec {
	// Create codec for encoding / decoding to and from YAML
	ser := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
	versions := schema.GroupVersions([]schema.GroupVersion{kubeletconfigv1beta1.SchemeGroupVersion})
	codec := serializer.NewCodecFactory(scheme).CodecForVersions(ser, ser, versions, versions)

	return &kubeletConfigCodec{
		fciCodec: fciCodec,
		codec:    codec,
	}
}

type kubeletConfigCodec struct {
	fciCodec FileContentInlineCodec
	codec    runtime.Codec
}

// Encode encodes the given *kubeletconfigv1beta1.KubeletConfiguration into a *extensionsv1alpha1.FileContentInline.
func (c *kubeletConfigCodec) Encode(kubeletConfig *kubeletconfigv1beta1.KubeletConfiguration, encoding string) (*extensionsv1alpha1.FileContentInline, error) {
	// Encode kubelet configuration to YAML
	data, err := runtime.Encode(c.codec, kubeletConfig)
	if err != nil {
		return nil, errors.Wrap(err, "could not encode kubelet configuration to YAML")
	}

	fci, err := c.fciCodec.Encode(data, encoding)
	if err != nil {
		return nil, errors.Wrap(err, "could not encode kubelet config file content data")
	}

	return fci, nil
}

// Decode decodes a *kubeletconfigv1beta1.KubeletConfiguration from the given *extensionsv1alpha1.FileContentInline.
func (c *kubeletConfigCodec) Decode(fci *extensionsv1alpha1.FileContentInline) (*kubeletconfigv1beta1.KubeletConfiguration, error) {
	data, err := c.fciCodec.Decode(fci)
	if err != nil {
		return nil, errors.Wrap(err, "could not decode kubelet config file content data")
	}

	// Decode kubelet configuration from YAML
	kubeletConfig := &kubeletconfigv1beta1.KubeletConfiguration{}
	if err := runtime.DecodeInto(c.codec, data, kubeletConfig); err != nil {
		return nil, errors.Wrap(err, "could not decode kubelet configuration from YAML")
	}

	return kubeletConfig, nil
}
