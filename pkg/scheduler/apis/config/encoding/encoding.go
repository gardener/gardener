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

	schedulerconfig "github.com/gardener/gardener/pkg/scheduler/apis/config"
	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/versioning"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var scheme *runtime.Scheme

func init() {
	scheme = runtime.NewScheme()
	utilruntime.Must(schedulerconfigv1alpha1.AddToScheme(scheme))
	utilruntime.Must(schedulerconfig.AddToScheme(scheme))
}

// DecodeSchedulerConfiguration decodes the given raw extension into an external schedulerConfiguration version.
func DecodeSchedulerConfiguration(rawConfig *runtime.RawExtension, withDefaults bool) (*schedulerconfigv1alpha1.SchedulerConfiguration, error) {
	if cfg, ok := rawConfig.Object.(*schedulerconfigv1alpha1.SchedulerConfiguration); ok {
		return cfg, nil
	}
	return DecodeSchedulerConfigurationFromBytes(rawConfig.Raw, withDefaults)
}

// DecodeSchedulerConfigurationFromBytes decodes the given byte slice into an external schedulerConfiguration version.
func DecodeSchedulerConfigurationFromBytes(bytes []byte, withDefaults bool) (*schedulerconfigv1alpha1.SchedulerConfiguration, error) {
	cfg := &schedulerconfigv1alpha1.SchedulerConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: schedulerconfigv1alpha1.SchemeGroupVersion.String(),
			Kind:       "schedulerConfiguration",
		},
	}
	if _, _, err := getDecoder(withDefaults).Decode(bytes, nil, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// EncodeSchedulerConfiguration encodes the given external GardenletConfiguration version into a raw extension.
func EncodeSchedulerConfiguration(cfg *schedulerconfigv1alpha1.SchedulerConfiguration) (*runtime.RawExtension, error) {
	raw, err := EncodeSchedulerConfigurationToBytes(cfg)
	if err != nil {
		return nil, err
	}
	return &runtime.RawExtension{
		Raw:    raw,
		Object: cfg,
	}, nil
}

// EncodeSchedulerConfigurationToBytes encodes the given external schedulerConfiguration version into a byte slice.
func EncodeSchedulerConfigurationToBytes(cfg *schedulerconfigv1alpha1.SchedulerConfiguration) ([]byte, error) {
	encoder, err := getEncoder(schedulerconfigv1alpha1.SchemeGroupVersion, runtime.ContentTypeJSON)
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
