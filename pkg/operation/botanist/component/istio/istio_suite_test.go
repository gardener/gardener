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

package istio_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"
)

func TestIstio(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Botanist Component Istio Suite")
}

var chartsRootPath = filepath.Join("..", "..", "..", "..", "..", "charts")

// can't use https://pkg.go.dev/github.com/envoyproxy/go-control-plane/pkg/conversion
// directly as types differ.
func messageToStruct(msg proto.Message) *types.Struct {
	Expect(msg).NotTo(BeNil(), "valid message should be passed")

	buf := &bytes.Buffer{}
	err := (&jsonpb.Marshaler{OrigName: true}).Marshal(buf, msg)
	Expect(err).NotTo(HaveOccurred(), "marshaling of message succeeds")

	val := &types.Struct{}
	err = jsonpb.Unmarshal(buf, val)
	Expect(err).NotTo(HaveOccurred(), "unmarshaling of struct succeeds")

	return val
}

// applyJSON unmarshals a JSON string into a proto message.
func applyJSON(js []byte, pb proto.Message) error {

	reader := bytes.NewReader(js)
	m := jsonpb.Unmarshaler{}

	if err := m.Unmarshal(reader, pb); err != nil {
		m.AllowUnknownFields = true

		reader.Reset(js)

		return m.Unmarshal(reader, pb)
	}

	return nil
}

// applyYAML unmarshals a YAML string into a proto message.
// Unknown fields are allowed.
func applyYAML(yml []byte, pb proto.Message) error {
	js, err := yaml.YAMLToJSON(yml)
	if err != nil {
		return err
	}

	return applyJSON(js, pb)
}
