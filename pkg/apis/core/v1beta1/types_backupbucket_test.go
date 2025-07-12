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

var _ = Describe("BackupBucket", func() {
	Describe("BackupBucketSpec", func() {
		It("should not allow to reuse protobuf numbers of already removed fields", func() {
			obj := reflect.ValueOf(BackupBucketSpec{}).Type()
			for i := 0; i < obj.NumField(); i++ {
				f := obj.Field(i)

				protobufNum := strings.Split(f.Tag.Get("protobuf"), ",")[1]
				if protobufNum == "3" {
					Fail("protobuf 3 in BackupBucketSpec is reserved for removed secretRef field")
				}
			}
		})
	})
})
