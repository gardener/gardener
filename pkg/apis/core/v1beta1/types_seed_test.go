// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1_test

import (
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo/v2"

	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("Seed", func() {
	Describe("SeedSpec", func() {
		It("should not allow to reuse protobuf numbers of already removed fields", func() {
			obj := reflect.ValueOf(SeedSpec{}).Type()
			for i := 0; i < obj.NumField(); i++ {
				f := obj.Field(i)

				protobufNum := strings.Split(f.Tag.Get("protobuf"), ",")[1]
				if protobufNum == "5" {
					Fail("protobuf 5 in SeedSpec is reserved for removed secretRef field")
				}
			}
		})
	})

	Describe("SeedSettings", func() {
		It("should not allow to reuse protobuf numbers of already removed fields", func() {
			obj := reflect.ValueOf(SeedSettings{}).Type()
			for i := 0; i < obj.NumField(); i++ {
				f := obj.Field(i)

				protobufNum := strings.Split(f.Tag.Get("protobuf"), ",")[1]
				switch protobufNum {
				case "3":
					Fail("protobuf 3 in SeedSettings is reserved for removed shootDNS field")
				case "6":
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
				switch protobufNum {
				case "3":
					Fail("protobuf 3 in SeedDNSProvider is reserved for removed domains field")
				case "4":
					Fail("protobuf 4 in SeedDNSProvider is reserved for removed zones field")
				}
			}
		})
	})

	Describe("SeedSettingDependencyWatchdog", func() {
		It("should not allow to reuse protobuf numbers of already removed fields", func() {
			obj := reflect.ValueOf(SeedSettingDependencyWatchdog{}).Type()
			for i := 0; i < obj.NumField(); i++ {
				f := obj.Field(i)

				protobufNum := strings.Split(f.Tag.Get("protobuf"), ",")[1]
				switch protobufNum {
				case "1":
					Fail("protobuf 1 in SeedSettingDependencyWatchdog is reserved for removed endpoint field")
				case "2":
					Fail("protobuf 2 in SeedSettingDependencyWatchdog is reserved for removed probe field")
				}
			}
		})
	})
})
