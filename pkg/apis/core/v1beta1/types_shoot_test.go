// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1beta1_test

import (
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo/v2"

	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("Shoot", func() {
	Describe("ServiceAccountConfig", func() {
		It("should not allow to reuse protobuf numbers of already removed fields", func() {
			obj := reflect.ValueOf(ServiceAccountConfig{}).Type()
			for i := 0; i < obj.NumField(); i++ {
				f := obj.Field(i)

				protobufNum := strings.Split(f.Tag.Get("protobuf"), ",")[1]
				if protobufNum == "2" {
					Fail("protobuf 2 in ServiceAccountConfig is reserved for removed signingKeySecret field")
				}
			}
		})
	})

	Describe("KubeAPIServerConfig", func() {
		It("should not allow to reuse protobuf numbers of already removed fields", func() {
			obj := reflect.ValueOf(KubeAPIServerConfig{}).Type()
			for i := 0; i < obj.NumField(); i++ {
				f := obj.Field(i)

				protobufNum := strings.Split(f.Tag.Get("protobuf"), ",")[1]
				if protobufNum == "5" {
					Fail("protobuf 5 in KubeAPIServerConfig is reserved for removed enableBasicAuthentication field")
				}
			}
		})
	})

	Describe("KubeletConfig", func() {
		It("should not allow to reuse protobuf numbers of already removed fields", func() {
			obj := reflect.ValueOf(KubeletConfig{}).Type()
			for i := 0; i < obj.NumField(); i++ {
				f := obj.Field(i)

				protobufNum := strings.Split(f.Tag.Get("protobuf"), ",")[1]
				if protobufNum == "12" {
					Fail("protobuf 12 in KubeletConfig is reserved for removed imagePullProgressDeadline field")
				}
			}
		})
	})
})
