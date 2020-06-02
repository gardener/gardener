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
	"fmt"

	"github.com/gardener/gardener/pkg/utils"

	. "github.com/gardener/gardener/pkg/utils/secrets"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PrivateKey InfoData", func() {
	var (
		privateKeyJSON     = []byte(fmt.Sprintf(`{"privateKey":"%s"}`, utils.EncodeBase64([]byte("foo"))))
		privateKeyInfoData = &PrivateKeyInfoData{
			PrivateKey: []byte("foo"),
		}
	)

	Describe("#UnmarshalPrivateKey", func() {
		It("should properly unmarshal PrivateKeyJSONData into PrivateKeyInfoData", func() {
			infoData, err := UnmarshalPrivateKey(privateKeyJSON)
			Expect(err).NotTo(HaveOccurred())
			Expect(infoData).To(Equal(privateKeyInfoData))
		})
	})

	Describe("#Marshal", func() {
		It("should properly marshal PrivateKeyInfoData into PrivateKeyJSONData", func() {
			data, err := privateKeyInfoData.Marshal()
			Expect(err).NotTo(HaveOccurred())
			Expect(data).To(Equal(privateKeyJSON))
		})
	})
})
