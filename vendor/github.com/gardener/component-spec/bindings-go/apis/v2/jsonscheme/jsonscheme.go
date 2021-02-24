// Copyright 2020 Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
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

//go:generate go-bindata -pkg jsonscheme ../../../../language-independent/component-descriptor-v2-schema.yaml

package jsonscheme

import (
	"errors"
	"fmt"

	"github.com/ghodss/yaml"
	"github.com/xeipuuv/gojsonschema"
)

var Schema *gojsonschema.Schema

func init() {
	dataBytes, err := LanguageIndependentComponentDescriptorV2SchemaYamlBytes()
	if err != nil {
		panic(err)
	}

	data, err := yaml.YAMLToJSON(dataBytes)
	if err != nil {
		panic(err)
	}

	Schema, err = gojsonschema.NewSchema(gojsonschema.NewBytesLoader(data))
	if err != nil {
		panic(err)
	}
}

// Validate validates the given data against the component descriptor v2 jsonscheme.
func Validate(src []byte) error {
	data, err := yaml.YAMLToJSON(src)
	if err != nil {
		return err
	}
	documentLoader := gojsonschema.NewBytesLoader(data)
	res, err := Schema.Validate(documentLoader)
	if err != nil {
		return err
	}

	if !res.Valid() {
		errs := res.Errors()
		errMsg := errs[0].String()
		for i := 1; i < len(errs); i++ {
			errMsg = fmt.Sprintf("%s;%s", errMsg, errs[i].String())
		}
		return errors.New(errMsg)
	}

	return nil
}
