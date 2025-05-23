// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terminal_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

func TestTerminal(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component Gardener Dashboard Terminal Suite")
}

var (
	crdScheme *runtime.Scheme
	crdCodec  runtime.Codec
)

func init() {
	crdScheme = runtime.NewScheme()
	utilruntime.Must(apiextensionsv1.AddToScheme(crdScheme))

	var (
		ser      = json.NewSerializerWithOptions(json.DefaultMetaFactory, crdScheme, crdScheme, json.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
		versions = schema.GroupVersions([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
	)

	crdCodec = serializer.NewCodecFactory(crdScheme).CodecForVersions(ser, ser, versions, versions)
}
