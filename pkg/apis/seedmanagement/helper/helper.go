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

package helper

import (
	"fmt"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/versioning"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var scheme *runtime.Scheme

func init() {
	scheme = runtime.NewScheme()
	utilruntime.Must(config.AddToScheme(scheme))
	utilruntime.Must(configv1alpha1.AddToScheme(scheme))
	utilruntime.Must(gardencore.AddToScheme(scheme))
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme))
}

// DecodeGardenletConfig decodes the given raw extension into an internal GardenletConfiguration version.
func DecodeGardenletConfig(rawConfig *runtime.RawExtension, withDefaults bool) (*config.GardenletConfiguration, error) {
	cfg, err := DecodeGardenletConfigExternal(rawConfig, withDefaults)
	if err != nil {
		return nil, err
	}
	return ConvertGardenletConfig(cfg)
}

// DecodeGardenletConfig decodes the given raw extension into an external GardenletConfiguration version.
func DecodeGardenletConfigExternal(rawConfig *runtime.RawExtension, withDefaults bool) (*configv1alpha1.GardenletConfiguration, error) {
	if cfg, ok := rawConfig.Object.(*configv1alpha1.GardenletConfiguration); ok {
		return cfg, nil
	}
	cfg := &configv1alpha1.GardenletConfiguration{}
	if _, _, err := getDecoder(withDefaults).Decode(rawConfig.Raw, nil, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ConvertGardenletConfig converts the given object to an internal GardenletConfiguration version.
func ConvertGardenletConfig(obj runtime.Object) (*config.GardenletConfiguration, error) {
	obj, err := scheme.ConvertToVersion(obj, config.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	result, ok := obj.(*config.GardenletConfiguration)
	if !ok {
		return nil, fmt.Errorf("could not convert GardenletConfiguration to internal version")
	}
	return result, nil
}

// ConvertGardenletConfigExternal converts the given object to an external  GardenletConfiguration version.
func ConvertGardenletConfigExternal(obj runtime.Object) (*configv1alpha1.GardenletConfiguration, error) {
	obj, err := scheme.ConvertToVersion(obj, configv1alpha1.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	result, ok := obj.(*configv1alpha1.GardenletConfiguration)
	if !ok {
		return nil, fmt.Errorf("could not convert GardenletConfiguration to version %s", configv1alpha1.SchemeGroupVersion.String())
	}
	return result, nil
}

// ConvertSeed converts the given external Seed version to an internal version.
func ConvertSeed(obj runtime.Object) (*gardencore.Seed, error) {
	obj, err := scheme.ConvertToVersion(obj, gardencore.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	result, ok := obj.(*gardencore.Seed)
	if !ok {
		return nil, fmt.Errorf("could not convert Seed to internal version")
	}
	return result, nil
}

// ConvertSeedExternal converts the given internal Seed version to an external version.
func ConvertSeedExternal(obj runtime.Object) (*gardencorev1beta1.Seed, error) {
	obj, err := scheme.ConvertToVersion(obj, gardencorev1beta1.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	result, ok := obj.(*gardencorev1beta1.Seed)
	if !ok {
		return nil, fmt.Errorf("could not convert Seed to version %s", gardencorev1beta1.SchemeGroupVersion.String())
	}
	return result, nil
}

func getDecoder(withDefaulter bool) runtime.Decoder {
	if withDefaulter {
		return serializer.NewCodecFactory(scheme).UniversalDecoder()
	}
	return versioning.NewCodec(nil, serializer.NewCodecFactory(scheme).UniversalDeserializer(), runtime.UnsafeObjectConvertor(scheme),
		scheme, scheme, nil, runtime.DisabledGroupVersioner, runtime.InternalGroupVersioner, scheme.Name())

}

// GetBootstrap returns the value of the given Bootstrap, or None if nil.
func GetBootstrap(bootstrap *seedmanagement.Bootstrap) seedmanagement.Bootstrap {
	if bootstrap != nil {
		return *bootstrap
	}
	return seedmanagement.BootstrapNone
}
