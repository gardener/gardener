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

package builder

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GardenClientMapBuilder", func() {

	var (
		fakeLogger      logrus.FieldLogger
		restConfig      *rest.Config
		uncachedObjects []client.Object
	)

	BeforeEach(func() {
		fakeLogger = logger.NewNopLogger()
		restConfig = &rest.Config{}
		uncachedObjects = []client.Object{&corev1.ConfigMap{}, &gardencorev1beta1.Shoot{}}
	})

	Context("#logger", func() {
		It("should be set correctly by WithLogger", func() {
			builder := NewGardenClientMapBuilder().WithLogger(fakeLogger)
			Expect(builder.logger).To(BeIdenticalTo(fakeLogger))
		})
	})

	Context("#restConfig", func() {
		It("should be set correctly by WithRESTConfig", func() {
			builder := NewGardenClientMapBuilder().WithRESTConfig(restConfig)
			Expect(builder.restConfig).To(BeIdenticalTo(restConfig))
		})
	})

	Context("#uncachedObjects", func() {
		It("should be set correctly by WithUncached", func() {
			builder := NewGardenClientMapBuilder().WithUncached(uncachedObjects...)
			Expect(builder.uncachedObjects).To(Equal(uncachedObjects))
		})
	})

	Context("#Build", func() {
		It("should fail if logger was not set", func() {
			clientMap, err := NewGardenClientMapBuilder().Build()
			Expect(err).To(MatchError("logger is required but not set"))
			Expect(clientMap).To(BeNil())
		})

		It("should fail if restConfig was not set", func() {
			clientMap, err := NewGardenClientMapBuilder().WithLogger(fakeLogger).Build()
			Expect(err).To(MatchError("restConfig is required but not set"))
			Expect(clientMap).To(BeNil())
		})

		It("should succeed to build ClientMap", func() {
			clientSet, err := NewGardenClientMapBuilder().
				WithRESTConfig(restConfig).
				WithLogger(fakeLogger).
				Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(clientSet).NotTo(BeNil())
		})
	})

})
