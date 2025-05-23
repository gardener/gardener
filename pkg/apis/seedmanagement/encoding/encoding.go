// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package encoding

import (
	"bytes"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/versioning"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

var scheme *runtime.Scheme

func init() {
	scheme = runtime.NewScheme()
	// core schemes are needed here to properly decode the embedded SeedTemplate objects
	utilruntime.Must(gardencore.AddToScheme(scheme))
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme))
	utilruntime.Must(gardenletconfigv1alpha1.AddToScheme(scheme))
}

// DecodeGardenletConfiguration decodes the given raw extension into an external GardenletConfiguration version.
func DecodeGardenletConfiguration(rawConfig *runtime.RawExtension, withDefaults bool) (*gardenletconfigv1alpha1.GardenletConfiguration, error) {
	if cfg, ok := rawConfig.Object.(*gardenletconfigv1alpha1.GardenletConfiguration); ok {
		return cfg, nil
	}
	return DecodeGardenletConfigurationFromBytes(rawConfig.Raw, withDefaults)
}

// DecodeGardenletConfigurationFromBytes decodes the given byte slice into an external GardenletConfiguration version.
func DecodeGardenletConfigurationFromBytes(bytes []byte, withDefaults bool) (*gardenletconfigv1alpha1.GardenletConfiguration, error) {
	cfg := &gardenletconfigv1alpha1.GardenletConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
			Kind:       "GardenletConfiguration",
		},
	}
	if _, _, err := getDecoder(withDefaults).Decode(bytes, nil, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// EncodeGardenletConfiguration encodes the given external GardenletConfiguration version into a raw extension.
func EncodeGardenletConfiguration(cfg *gardenletconfigv1alpha1.GardenletConfiguration) (*runtime.RawExtension, error) {
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
func EncodeGardenletConfigurationToBytes(cfg *gardenletconfigv1alpha1.GardenletConfiguration) ([]byte, error) {
	encoder, err := getEncoder(gardenletconfigv1alpha1.SchemeGroupVersion, runtime.ContentTypeJSON)
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
