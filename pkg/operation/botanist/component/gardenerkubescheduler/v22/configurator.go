// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v22

import (
	"bytes"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler/configurator"
	schedulerv22v1beta2 "github.com/gardener/gardener/third_party/kube-scheduler/v22/v1beta2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"
)

type v22Configurator struct {
	config *schedulerv22v1beta2.KubeSchedulerConfiguration
	codec  serializer.CodecFactory
}

// NewConfigurator creates a Configurator for Kubernetes version 1.22.
func NewConfigurator(resourceName, namespace string, config *schedulerv22v1beta2.KubeSchedulerConfiguration) (configurator.Configurator, error) {
	scheme := runtime.NewScheme()

	if err := schedulerv22v1beta2.AddToScheme(scheme); err != nil {
		return nil, err
	}

	config.LeaderElection = componentbaseconfigv1alpha1.LeaderElectionConfiguration{
		LeaseDuration:     metav1.Duration{Duration: 15 * time.Second},
		RenewDeadline:     metav1.Duration{Duration: 10 * time.Second},
		RetryPeriod:       metav1.Duration{Duration: 2 * time.Second},
		ResourceLock:      "leases",
		ResourceName:      resourceName,
		LeaderElect:       pointer.Bool(true),
		ResourceNamespace: namespace,
	}

	return &v22Configurator{
		config: config,
		codec:  serializer.NewCodecFactory(scheme, serializer.EnableStrict),
	}, nil
}

func (c *v22Configurator) Config() (string, error) {
	const mediaType = runtime.ContentTypeYAML

	componentConfigYAML := &bytes.Buffer{}

	info, ok := runtime.SerializerInfoForMediaType(c.codec.SupportedMediaTypes(), mediaType)
	if !ok {
		return "", fmt.Errorf("unable to locate encoder -- %q is not a supported media type", mediaType)
	}

	encoder := c.codec.EncoderForVersion(info.Serializer, schedulerv22v1beta2.SchemeGroupVersion)

	if err := encoder.Encode(c.config, componentConfigYAML); err != nil {
		return "", err
	}

	return componentConfigYAML.String(), nil
}
