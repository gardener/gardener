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

package secrets_test

import (
	. "github.com/gardener/gardener/pkg/utils/secrets"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StaticToken Data", func() {
	var (
		staticTokenJSON     = []byte(`{"tokens":{"foo":"foo"}}`)
		staticTokenInfoData = &StaticTokenInfoData{
			Tokens: map[string]string{
				"foo": "foo",
			},
		}
	)

	Describe("#UnmarshalStaticToken", func() {
		It("should properly unmarshal StaticTokenJSONData into StaticTokenInfoData", func() {
			infoData, err := UnmarshalStaticToken(staticTokenJSON)
			Expect(err).NotTo(HaveOccurred())
			Expect(infoData).To(Equal(staticTokenInfoData))
		})
	})

	Describe("#Marshal", func() {
		It("should properly marshal StaticTokenInfoData into StaticTokenJSONData", func() {
			data, err := staticTokenInfoData.Marshal()
			Expect(err).NotTo(HaveOccurred())
			Expect(data).To(Equal(staticTokenJSON))
		})
	})

	Describe("#TypeVersion", func() {
		It("should return the correct TypeVersion", func() {
			typeVersion := staticTokenInfoData.TypeVersion()
			Expect(typeVersion).To(Equal(StaticTokenDataType))
		})
	})

	Describe("#NewStaticTokenInfoData", func() {
		It("should return new StaticTokenInfoData from the passed tokens list", func() {
			tokens := map[string]string{
				"foo": "foo",
			}
			newStaticTokenInfoData := NewStaticTokenInfoData(tokens)
			Expect(newStaticTokenInfoData).To(Equal(staticTokenInfoData))
		})
	})
})
