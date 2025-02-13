// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"sigs.k8s.io/controller-runtime/pkg/client"

	forkedyaml "github.com/gardener/gardener/third_party/gopkg.in/yaml.v2"
)

// Serialize serializes and encodes the passed object.
func Serialize(obj client.Object, scheme *runtime.Scheme) (string, error) {
	var groupVersions []schema.GroupVersion
	for k := range scheme.AllKnownTypes() {
		groupVersions = append(groupVersions, k.GroupVersion())
	}

	var (
		ser   = json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{Yaml: true})
		codec = serializer.NewCodecFactory(scheme).CodecForVersions(ser, ser, schema.GroupVersions(groupVersions), schema.GroupVersions(groupVersions))
	)

	serializationYAML, err := runtime.Encode(codec, obj)
	if err != nil {
		return "", fmt.Errorf("failed encoding the object: %w", err)
	}

	// Keep this in sync with pkg/utils/managedresources/registry.go
	// See https://github.com/gardener/gardener/pull/8312
	var anyObj any
	if err := forkedyaml.Unmarshal(serializationYAML, &anyObj); err != nil {
		return "", fmt.Errorf("failed unmarshalling the object: %w", err)
	}

	serBytes, err := forkedyaml.Marshal(anyObj)
	if err != nil {
		return "", fmt.Errorf("failed marshalling the object to YAML: %w", err)
	}

	return string(serBytes), nil
}
