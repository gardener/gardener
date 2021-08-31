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

package encoding

import (
	"bytes"
	"fmt"

	controllermanagerconfig "github.com/gardener/gardener/pkg/controllermanager/apis/config"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/versioning"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var scheme *runtime.Scheme

func init() {
	scheme = runtime.NewScheme()
	utilruntime.Must(controllermanagerconfigv1alpha1.AddToScheme(scheme))
	utilruntime.Must(controllermanagerconfig.AddToScheme(scheme))
}

// DecodeControllerManagerConfiguration decodes the given raw extension into an external ControllerManagerConfiguration version.
func DecodeControllerManagerConfiguration(rawConfig *runtime.RawExtension, withDefaults bool) (*controllermanagerconfigv1alpha1.ControllerManagerConfiguration, error) {
	if cfg, ok := rawConfig.Object.(*controllermanagerconfigv1alpha1.ControllerManagerConfiguration); ok {
		return cfg, nil
	}
	return DecodeControllerManagerConfigurationFromBytes(rawConfig.Raw, withDefaults)
}

// DecodeControllerManagerConfigurationFromBytes decodes the given byte slice into an external ControllerManagerConfiguration version.
func DecodeControllerManagerConfigurationFromBytes(bytes []byte, withDefaults bool) (*controllermanagerconfigv1alpha1.ControllerManagerConfiguration, error) {
	cfg := &controllermanagerconfigv1alpha1.ControllerManagerConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: controllermanagerconfigv1alpha1.SchemeGroupVersion.String(),
			Kind:       "ControllerManagerConfiguration",
		},
	}
	if _, _, err := getDecoder(withDefaults).Decode(bytes, nil, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// EncodeControllerManagerConfiguration encodes the given external GardenletConfiguration version into a raw extension.
func EncodeControllerManagerConfiguration(cfg *controllermanagerconfigv1alpha1.ControllerManagerConfiguration) (*runtime.RawExtension, error) {
	raw, err := EncodeControllerManagerConfigurationToBytes(cfg)
	if err != nil {
		return nil, err
	}
	return &runtime.RawExtension{
		Raw:    raw,
		Object: cfg,
	}, nil
}

// EncodeControllerManagerConfigurationToBytes encodes the given external ControllerManagerConfiguration version into a byte slice.
func EncodeControllerManagerConfigurationToBytes(cfg *controllermanagerconfigv1alpha1.ControllerManagerConfiguration) ([]byte, error) {
	encoder, err := getEncoder(controllermanagerconfigv1alpha1.SchemeGroupVersion, runtime.ContentTypeJSON)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := encoder.Encode(cfg, &b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func getDecoder(withDefaulter bool) runtime.Decoder {
	if withDefaulter {
		return serializer.NewCodecFactory(scheme).UniversalDecoder()
	}
	return versioning.NewCodec(nil, serializer.NewCodecFactory(scheme).UniversalDeserializer(), runtime.UnsafeObjectConvertor(scheme),
		scheme, scheme, nil, runtime.DisabledGroupVersioner, runtime.InternalGroupVersioner, scheme.Name())

}

func getEncoder(gv runtime.GroupVersioner, mediaType string) (runtime.Encoder, error) {
	codec := serializer.NewCodecFactory(scheme)
	si, ok := runtime.SerializerInfoForMediaType(codec.SupportedMediaTypes(), mediaType)
	if !ok {
		return nil, fmt.Errorf("could not find encoder for media type %q", runtime.ContentTypeJSON)
	}
	return codec.EncoderForVersion(si.Serializer, gv), nil
}
