// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	_ "embed"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var (
	crdScheme *runtime.Scheme
	crdCodec  runtime.Codec

	//go:embed assets/crd-dashboard.gardener.cloud_terminals.yaml
	rawCRD string
)

func init() {
	crdScheme = runtime.NewScheme()
	utilruntime.Must(apiextensionsv1.AddToScheme(crdScheme))

	var (
		ser = json.NewSerializerWithOptions(json.DefaultMetaFactory, crdScheme, crdScheme, json.SerializerOptions{
			Yaml:   true,
			Pretty: false,
			Strict: false,
		})
		versions = schema.GroupVersions([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
	)

	crdCodec = serializer.NewCodecFactory(crdScheme).CodecForVersions(ser, ser, versions, versions)
}

func (t *terminal) crd() (*apiextensionsv1.CustomResourceDefinition, error) {
	obj, err := runtime.Decode(crdCodec, []byte(rawCRD))
	if err != nil {
		return nil, err
	}

	crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		return nil, fmt.Errorf("expected *apiextensionsv1.CustomResourceDefinition but got %T", obj)
	}

	return crd, nil
}
