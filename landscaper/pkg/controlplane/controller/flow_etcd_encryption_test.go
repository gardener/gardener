// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controller

import (
	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
)

var _ = Describe("#GenerateEncryptionConfiguration", func() {
	var (
		testOperation operation

		expectedEncryptionConfiguration = apiserverconfigv1.EncryptionConfiguration{
			TypeMeta: metav1.TypeMeta{},
			Resources: []apiserverconfigv1.ResourceConfiguration{
				{
					Resources: []string{
						"secrets",
						"controllerdeployments.core.gardener.cloud",
						"controllerregistrations.core.gardener.cloud",
						"shootstates.core.gardener.cloud",
					},
				},
			},
		}
	)

	BeforeEach(func() {
		testOperation = operation{
			log:     logrus.NewEntry(logger.NewNopLogger()),
			imports: &imports.Imports{},
		}
	})

	It("should not overwrite an existing encryption configuration", func() {
		existingEncryptionConfig := apiserverconfigv1.EncryptionConfiguration{
			Resources: []apiserverconfigv1.ResourceConfiguration{
				{
					Resources: []string{
						"test-resource",
					},
				},
			},
		}

		testOperation.imports = &imports.Imports{GardenerAPIServer: imports.GardenerAPIServer{ComponentConfiguration: imports.APIServerComponentConfiguration{
			Encryption: &existingEncryptionConfig,
		},
		},
		}

		Expect(testOperation.GenerateEncryptionConfiguration(nil)).ToNot(HaveOccurred())
		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.Encryption).To(Equal(&existingEncryptionConfig))
	})

	It("should generate a new encryption configuration", func() {
		Expect(testOperation.GenerateEncryptionConfiguration(nil)).ToNot(HaveOccurred())
		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.Encryption).ToNot(BeNil())
		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.Encryption.Resources).To(HaveLen(1))
		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.Encryption.Resources[0].Resources).To(Equal(expectedEncryptionConfiguration.Resources[0].Resources))
		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.Encryption.Resources[0].Providers).To(HaveLen(2))
		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.Encryption.Resources[0].Providers[0].AESCBC).ToNot(BeNil())
		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.Encryption.Resources[0].Providers[0].AESCBC.Keys).To(HaveLen(1))
	})
})
