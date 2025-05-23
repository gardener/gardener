// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcereservation

import (
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/gardener/gardener/plugin/pkg/shoot/resourcereservation/apis/shootresourcereservation"
	"github.com/gardener/gardener/plugin/pkg/shoot/resourcereservation/apis/shootresourcereservation/install"
	"github.com/gardener/gardener/plugin/pkg/shoot/resourcereservation/apis/shootresourcereservation/v1alpha1"
)

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
)

func init() {
	install.Install(scheme)
}

// LoadConfiguration loads the provided configuration.
func LoadConfiguration(config io.Reader) (*shootresourcereservation.Configuration, error) {
	// if no config is provided, return a default Configuration
	if config == nil {
		externalConfig := &v1alpha1.Configuration{}
		scheme.Default(externalConfig)
		internalConfig := &shootresourcereservation.Configuration{}
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

	cfg, ok := decodedObj.(*shootresourcereservation.Configuration)
	if !ok {
		return nil, fmt.Errorf("unexpected type: %T", decodedObj)
	}

	return cfg, nil
}
