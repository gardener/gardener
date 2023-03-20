// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package dnsrewriting

import (
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/gardener/gardener/plugin/pkg/shoot/dnsrewriting/apis/shootdnsrewriting"
	"github.com/gardener/gardener/plugin/pkg/shoot/dnsrewriting/apis/shootdnsrewriting/install"
	"github.com/gardener/gardener/plugin/pkg/shoot/dnsrewriting/apis/shootdnsrewriting/v1alpha1"
)

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
)

func init() {
	install.Install(scheme)
}

// LoadConfiguration loads the provided configuration.
func LoadConfiguration(config io.Reader) (*shootdnsrewriting.Configuration, error) {
	// if no config is provided, return a default Configuration
	if config == nil {
		externalConfig := &v1alpha1.Configuration{}
		scheme.Default(externalConfig)
		internalConfig := &shootdnsrewriting.Configuration{}
		if err := scheme.Convert(externalConfig, internalConfig, nil); err != nil {
			return nil, err
		}
		return internalConfig, nil
	}

	data, err := io.ReadAll(config)
	if err != nil {
		return nil, err
	}

	decodedObj, err := runtime.Decode(codecs.UniversalDecoder(), data)
	if err != nil {
		return nil, err
	}

	cfg, ok := decodedObj.(*shootdnsrewriting.Configuration)
	if !ok {
		return nil, fmt.Errorf("unexpected type: %T", decodedObj)
	}

	return cfg, nil
}
