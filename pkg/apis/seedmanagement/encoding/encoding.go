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

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/versioning"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var scheme *runtime.Scheme

func init() {
	scheme = runtime.NewScheme()
	// core schemes are needed here to properly decode the embedded SeedTemplate objects
	utilruntime.Must(gardencore.AddToScheme(scheme))
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme))
	utilruntime.Must(config.AddToScheme(scheme))
	utilruntime.Must(configv1alpha1.AddToScheme(scheme))
}

// DecodeGardenletConfiguration decodes the given raw extension into an external GardenletConfiguration version.
func DecodeGardenletConfiguration(rawConfig *runtime.RawExtension, withDefaults bool) (*configv1alpha1.GardenletConfiguration, error) {
	if cfg, ok := rawConfig.Object.(*configv1alpha1.GardenletConfiguration); ok {
		return cfg, nil
	}
	return DecodeGardenletConfigurationFromBytes(rawConfig.Raw, withDefaults)
}

// DecodeGardenletConfigurationFromBytes decodes the given byte slice into an external GardenletConfiguration version.
func DecodeGardenletConfigurationFromBytes(bytes []byte, withDefaults bool) (*configv1alpha1.GardenletConfiguration, error) {
	cfg := &configv1alpha1.GardenletConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: configv1alpha1.SchemeGroupVersion.String(),
			Kind:       "GardenletConfiguration",
		},
	}
	if _, _, err := getDecoder(withDefaults).Decode(bytes, nil, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// EncodeGardenletConfiguration encodes the given external GardenletConfiguration version into a raw extension.
func EncodeGardenletConfiguration(cfg *configv1alpha1.GardenletConfiguration) (*runtime.RawExtension, error) {
	raw, err := EncodeGardenletConfigurationToBytes(cfg)
	if err != nil {
		return nil, err
	}
	return &runtime.RawExtension{
		Raw:    raw,
		Object: cfg,
	}, nil
}

// EncodeGardenletConfigurationToBytes encodes the given external GardenletConfiguration version into a byte slice.
func EncodeGardenletConfigurationToBytes(cfg *configv1alpha1.GardenletConfiguration) ([]byte, error) {
	encoder, err := getEncoder(configv1alpha1.SchemeGroupVersion, runtime.ContentTypeJSON)
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
