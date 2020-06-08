// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package test

import (
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/generator"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	"github.com/gobuffalo/packr"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

var (
	onlyOwnerPerm = int32(0600)
)

// DescribeTest returns a function which can be used in tests for the
// template generator implementation. It receives an instance of a template
// generator and a packr Box with the test files to be used in the tests.
var DescribeTest = func(g generator.Generator, box packr.Box) func() {
	return func() {

		ginkgo.It("should render correctly", func() {
			expectedCloudInit, err := box.Find("cloud-init")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			cloudInit, _, err := g.Generate(&generator.OperatingSystemConfig{
				Object: &extensionsv1alpha1.OperatingSystemConfig{},
				Files: []*generator.File{
					{
						Path:        "/foo",
						Content:     []byte("bar"),
						Permissions: &onlyOwnerPerm,
					},
				},

				Units: []*generator.Unit{
					{
						Name:    "docker.service",
						Content: []byte("unit"),
						DropIns: []*generator.DropIn{
							{
								Name:    "10-docker-opts.conf",
								Content: []byte("override"),
							},
						},
					},
				},
				Bootstrap: true,
			})

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cloudInit).To(gomega.Equal(expectedCloudInit))
		})
	}
}
