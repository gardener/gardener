// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

var _ = Describe("Seed", func() {
	Describe("SeedSettings", func() {
		It("should not allow to reuse protobuf numbers of already removed fields", func() {
			obj := reflect.ValueOf(SeedSettings{}).Type()
			for i := 0; i < obj.NumField(); i++ {
				f := obj.Field(i)

				protobufNum := strings.Split(f.Tag.Get("protobuf"), ",")[1]
				if protobufNum == "3" {
					Fail("protobuf 3 in SeedSettings is reserved for removed shootDNS field")
				} else if protobufNum == "6" {
					Fail("protobuf 6 in SeedSettings is reserved for removed ownerChecks field")
				}
			}
		})
	})

	Describe("SeedDNS", func() {
		It("should not allow to reuse protobuf numbers of already removed fields", func() {
			obj := reflect.ValueOf(SeedDNS{}).Type()
			for i := 0; i < obj.NumField(); i++ {
				f := obj.Field(i)

				protobufNum := strings.Split(f.Tag.Get("protobuf"), ",")[1]
				if protobufNum == "1" {
					Fail("protobuf 1 in SeedDNS is reserved for removed ingressDomain field")
				}
			}
		})
	})

	Describe("SeedDNSProvider", func() {
		It("should not allow to reuse protobuf numbers of already removed fields", func() {
			obj := reflect.ValueOf(SeedDNSProvider{}).Type()
			for i := 0; i < obj.NumField(); i++ {
				f := obj.Field(i)

				protobufNum := strings.Split(f.Tag.Get("protobuf"), ",")[1]
				if protobufNum == "3" {
					Fail("protobuf 3 in SeedDNSProvider is reserved for removed domains field")
				} else if protobufNum == "4" {
					Fail("protobuf 4 in SeedDNSProvider is reserved for removed zones field")
				}
			}
		})
	})
})
