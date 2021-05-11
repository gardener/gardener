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

package loader

import (
	"fmt"
	"os"

	"github.com/gardener/gardener/pkg/scheduler/apis/config"
	"github.com/gardener/gardener/pkg/scheduler/apis/config/install"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var scheme *runtime.Scheme

func init() {
	scheme = runtime.NewScheme()
	install.Install(scheme)
}

// LoadFromFile takes a filename and de-serializes the contents into SchedulerConfiguration object.
func LoadFromFile(filename string) (*config.SchedulerConfiguration, error) {
	bytes, err := os.ReadFile(filename)
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

	configObj, gvk, err := serializer.NewCodecFactory(scheme).UniversalDecoder().Decode(data, nil, cfg)
	if err != nil {
		return nil, err
	}
	config, ok := configObj.(*config.SchedulerConfiguration)
	if !ok {
		return nil, fmt.Errorf("got unexpected config type: %v", gvk)
	}
	return config, nil
}
