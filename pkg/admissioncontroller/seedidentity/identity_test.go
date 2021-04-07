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

package seedidentity_test

import (
	. "github.com/gardener/gardener/pkg/admissioncontroller/seedidentity"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"k8s.io/apiserver/pkg/authentication/user"
)

var _ = Describe("identity", func() {
	DescribeTable("#FromUserInfoInterface",
		func(u user.Info, expectedSeedName string, expectedIsSeedValue bool) {
			seedName, isSeed := FromUserInfoInterface(u)

			Expect(seedName).To(Equal(expectedSeedName))
			Expect(isSeed).To(Equal(expectedIsSeedValue))
		},

		Entry("nil", nil, "", false),
		Entry("no user name prefix", &user.DefaultInfo{Name: "foo"}, "", false),
		Entry("user name prefix but no groups", &user.DefaultInfo{Name: "gardener.cloud:system:seed:foo"}, "", false),
		Entry("user name prefix but seed group not present", &user.DefaultInfo{Name: "gardener.cloud:system:seed:foo", Groups: []string{"bar"}}, "", false),
		Entry("user name prefix and seed group", &user.DefaultInfo{Name: "gardener.cloud:system:seed:foo", Groups: []string{"gardener.cloud:system:seeds"}}, "foo", true),
		Entry("user name prefix and seed group (ambiguous)", &user.DefaultInfo{Name: "gardener.cloud:system:seed:<ambiguous>", Groups: []string{"gardener.cloud:system:seeds"}}, "", true),
	)
})
