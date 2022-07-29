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

package test

import (
	"github.com/gardener/gardener/pkg/client/kubernetes"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Serialize serializes and encodes the passed object.
func Serialize(obj client.Object) string {
	var (
		scheme        = kubernetes.SeedScheme
		groupVersions []schema.GroupVersion
	)

	for k := range scheme.AllKnownTypes() {
		groupVersions = append(groupVersions, k.GroupVersion())
	}

	var (
		ser   = json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
		codec = serializer.NewCodecFactory(scheme).CodecForVersions(ser, ser, schema.GroupVersions(groupVersions), schema.GroupVersions(groupVersions))
	)

	serializationYAML, err := runtime.Encode(codec, obj)
	Expect(err).NotTo(HaveOccurred())

	return string(serializationYAML)
}
