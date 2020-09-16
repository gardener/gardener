// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package loader

import (
	"fmt"
	"io/ioutil"

	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/gardener/gardener/pkg/scheduler/apis/config"
	"github.com/gardener/gardener/pkg/scheduler/apis/config/install"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime/serializer/versioning"
)

var (
	Codec  runtime.Codec
	Scheme *runtime.Scheme
)

func init() {
	Scheme = scheme.Scheme
	install.Install(Scheme)
	yamlSerializer := json.NewYAMLSerializer(json.DefaultMetaFactory, Scheme, Scheme)
	Codec = versioning.NewDefaultingCodecForScheme(
		Scheme,
		yamlSerializer,
		yamlSerializer,
		schema.GroupVersion{Version: "v1alpha1"},
		runtime.InternalGroupVersioner,
	)
}

// LoadFromFile takes a filename and de-serializes the contents into SchedulerConfiguration object.
func LoadFromFile(filename string) (*config.SchedulerConfiguration, error) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return Load(bytes)
}

// Load takes a byte slice and de-serializes the contents into SchedulerConfiguration object.
// Encapsulates de-serialization without assuming the source is a file.
func Load(data []byte) (*config.SchedulerConfiguration, error) {
	cfg := &config.SchedulerConfiguration{}

	if len(data) == 0 {
		return cfg, nil
	}

	configObj, gvk, err := serializer.NewCodecFactory(Scheme).UniversalDecoder().Decode(data, nil, cfg)
	if err != nil {
		return nil, err
	}
	config, ok := configObj.(*config.SchedulerConfiguration)
	if !ok {
		return nil, fmt.Errorf("got unexpected config type: %v", gvk)
	}
	return config, nil
}
