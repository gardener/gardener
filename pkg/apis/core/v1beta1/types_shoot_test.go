// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

	Describe("Kubernetes", func() {
		It("should not allow to reuse protobuf numbers of already removed fields", func() {
			obj := reflect.ValueOf(Kubernetes{}).Type()
			for i := 0; i < obj.NumField(); i++ {
				f := obj.Field(i)

				protobufNum := strings.Split(f.Tag.Get("protobuf"), ",")[1]
				if protobufNum == "1" {
					Fail("protobuf 1 in Kubernetes is reserved for removed allowPrivilegedContainers field")
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
