// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubelet

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	oscutils "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/utils"
)

// ConfigCodec contains methods for encoding and decoding *kubeletconfigv1beta1.KubeletConfiguration objects
// to and from *extensionsv1alpha1.FileContentInline.
type ConfigCodec interface {
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

// NewConfigCodec creates an returns a new ConfigCodec.
func NewConfigCodec(fciCodec oscutils.FileContentInlineCodec) ConfigCodec {
	// Create codec for encoding / decoding to and from YAML
	ser := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
	versions := schema.GroupVersions([]schema.GroupVersion{kubeletconfigv1beta1.SchemeGroupVersion})
	codec := serializer.NewCodecFactory(scheme).CodecForVersions(ser, ser, versions, versions)

	return &configCodec{
		fciCodec: fciCodec,
		codec:    codec,
	}
}

type configCodec struct {
	fciCodec oscutils.FileContentInlineCodec
	codec    runtime.Codec
}

// Encode encodes the given *kubeletconfigv1beta1.KubeletConfiguration into a *extensionsv1alpha1.FileContentInline.
func (c *configCodec) Encode(kubeletConfig *kubeletconfigv1beta1.KubeletConfiguration, encoding string) (*extensionsv1alpha1.FileContentInline, error) {
	// Encode kubelet configuration to YAML
	data, err := runtime.Encode(c.codec, kubeletConfig)
	if err != nil {
		return nil, fmt.Errorf("could not encode kubelet configuration to YAML: %w", err)
	}

	fci, err := c.fciCodec.Encode(data, encoding)
	if err != nil {
		return nil, fmt.Errorf("could not encode kubelet config file content data: %w", err)
	}

	return fci, nil
}

// Decode decodes a *kubeletconfigv1beta1.KubeletConfiguration from the given *extensionsv1alpha1.FileContentInline.
func (c *configCodec) Decode(fci *extensionsv1alpha1.FileContentInline) (*kubeletconfigv1beta1.KubeletConfiguration, error) {
	data, err := c.fciCodec.Decode(fci)
	if err != nil {
		return nil, fmt.Errorf("could not decode kubelet config file content data: %w", err)
	}

	// Decode kubelet configuration from YAML
	kubeletConfig := &kubeletconfigv1beta1.KubeletConfiguration{}
	if err := runtime.DecodeInto(c.codec, data, kubeletConfig); err != nil {
		return nil, fmt.Errorf("could not decode kubelet configuration from YAML: %w", err)
	}

	return kubeletConfig, nil
}
