// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controlplane

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubeschedulerv1alpha1 "k8s.io/kube-scheduler/config/v1alpha1"
)

// KubeSchedulerConfigCodec contains methods for encoding and decoding *kubeschedulerv1alpha1.KubeSchedulerConfiguration
// objects to and from string.
type KubeSchedulerConfigCodec interface {
	// Encode encodes the given *kubeschedulerv1alpha1.KubeSchedulerConfiguration into a string.
	Encode(*kubeschedulerv1alpha1.KubeSchedulerConfiguration) (string, error)
	// Decode decodes a *kubeschedulerv1alpha1.KubeSchedulerConfiguration from the given string.
	Decode(string) (*kubeschedulerv1alpha1.KubeSchedulerConfiguration, error)
}

var scheme *runtime.Scheme

func init() {
	scheme = runtime.NewScheme()
	utilruntime.Must(kubeschedulerv1alpha1.AddToScheme(scheme))
}

// NewKubeSchedulerConfigCodec creates an returns a new KubeSchedulerConfigCodec.
func NewKubeSchedulerConfigCodec() KubeSchedulerConfigCodec {
	var (
		ser      = json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
		versions = schema.GroupVersions([]schema.GroupVersion{kubeschedulerv1alpha1.SchemeGroupVersion})
		codec    = serializer.NewCodecFactory(scheme).CodecForVersions(ser, ser, versions, versions)
	)

	return &kubeSchedulerConfigCodec{codec: codec}
}

type kubeSchedulerConfigCodec struct {
	codec runtime.Codec
}

// Encode encodes the given *kubeschedulerv1alpha1.KubeSchedulerConfiguration into a string.
func (c *kubeSchedulerConfigCodec) Encode(config *kubeschedulerv1alpha1.KubeSchedulerConfiguration) (string, error) {
	data, err := runtime.Encode(c.codec, config)
	if err != nil {
		return "", errors.Wrap(err, "could not encode kube-scheduler  configuration to YAML")
	}

	return string(data), nil
}

// Decode decodes a *kubeschedulerv1alpha1.KubeSchedulerConfiguration from the given string.
func (c *kubeSchedulerConfigCodec) Decode(data string) (*kubeschedulerv1alpha1.KubeSchedulerConfiguration, error) {
	config := &kubeschedulerv1alpha1.KubeSchedulerConfiguration{}
	if err := runtime.DecodeInto(c.codec, []byte(data), config); err != nil {
		return nil, errors.Wrap(err, "could not decode kube-scheduler  configuration from YAML")
	}

	return config, nil
}
