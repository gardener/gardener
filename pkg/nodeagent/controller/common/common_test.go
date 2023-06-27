// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package common

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var _ = Describe("Common", func() {
	type entry struct {
		name     string
		current  *v1alpha1.OperatingSystemConfig
		previous *v1alpha1.OperatingSystemConfig
		want     *OSCChanges
	}
	DescribeTable("#CalculateOSCChanges", func(current, previous *v1alpha1.OperatingSystemConfig, want *OSCChanges) {
		got := CalculateOSCChanges(current, previous)
		Expect(got).To(Equal(want))
	},
		Entry("one deleted file and unit", &v1alpha1.OperatingSystemConfig{
			Spec: v1alpha1.OperatingSystemConfigSpec{
				Units: []v1alpha1.Unit{
					{
						Name: "bla",
					},
				},
				Files: []v1alpha1.File{
					{
						Path: "/tmp/bla",
					},
				},
			},
		},
			&v1alpha1.OperatingSystemConfig{
				Spec: v1alpha1.OperatingSystemConfigSpec{
					Units: []v1alpha1.Unit{
						{
							Name: "bla",
						},
						{
							Name: "blub",
						},
					},
					Files: []v1alpha1.File{
						{
							Path: "/tmp/bla",
						},
						{
							Path: "/tmp/blub",
						},
					},
				},
			},
			&OSCChanges{
				DeletedUnits: []v1alpha1.Unit{
					{
						Name: "blub",
					},
				},
				DeletedFiles: []v1alpha1.File{
					{
						Path: "/tmp/blub",
					},
				},
			}),
		Entry("one deleted file and unit", &v1alpha1.OperatingSystemConfig{
			Spec: v1alpha1.OperatingSystemConfigSpec{
				Units: []v1alpha1.Unit{
					{
						Name: "bla",
					},
				},
				Files: []v1alpha1.File{
					{
						Path: "/tmp/bla",
					},
				},
			},
		},
			&v1alpha1.OperatingSystemConfig{
				Spec: v1alpha1.OperatingSystemConfigSpec{
					Units: []v1alpha1.Unit{
						{
							Name: "bla",
						},
						{
							Name: "blub",
						},
					},
					Files: []v1alpha1.File{
						{
							Path: "/tmp/bla",
						},
						{
							Path: "/tmp/blub",
						},
					},
				},
			},
			&OSCChanges{
				DeletedUnits: []v1alpha1.Unit{
					{
						Name: "blub",
					},
				},
				DeletedFiles: []v1alpha1.File{
					{
						Path: "/tmp/blub",
					},
				},
			}),
		Entry("one changed unit", &v1alpha1.OperatingSystemConfig{
			Spec: v1alpha1.OperatingSystemConfigSpec{
				Units: []v1alpha1.Unit{
					{
						Name:    "bla",
						Content: pointer.String("a unit content"),
					},
					{
						Name:    "blub",
						Content: pointer.String("changed unit content"),
					},
				},
			},
		},
			&v1alpha1.OperatingSystemConfig{
				Spec: v1alpha1.OperatingSystemConfigSpec{
					Units: []v1alpha1.Unit{
						{
							Name:    "bla",
							Content: pointer.String("a unit content"),
						},
						{
							Name:    "blub",
							Content: pointer.String("b unit content"),
						},
					},
				},
			},
			&OSCChanges{
				ChangedUnits: []v1alpha1.Unit{
					{
						Name:    "blub",
						Content: pointer.String("changed unit content"),
					},
				},
			},
		),

		Entry("one changed unit, one added unit, one deleted unit",
			&v1alpha1.OperatingSystemConfig{
				Spec: v1alpha1.OperatingSystemConfigSpec{
					Units: []v1alpha1.Unit{
						{
							Name:    "blub",
							Content: pointer.String("changed unit content"),
						},
						{
							Name:    "chacka",
							Content: pointer.String("added unit content"),
						},
					},
				},
			},
			&v1alpha1.OperatingSystemConfig{
				Spec: v1alpha1.OperatingSystemConfigSpec{
					Units: []v1alpha1.Unit{
						{
							Name:    "bla",
							Content: pointer.String("a unit content"),
						},
						{
							Name:    "blub",
							Content: pointer.String("b unit content"),
						},
					},
				},
			},
			&OSCChanges{
				ChangedUnits: []v1alpha1.Unit{
					{
						Name:    "blub",
						Content: pointer.String("changed unit content"),
					},
					{
						Name:    "chacka",
						Content: pointer.String("added unit content"),
					},
				},
				DeletedUnits: []v1alpha1.Unit{
					{
						Name:    "bla",
						Content: pointer.String("a unit content"),
					},
				},
			},
		),
	)
})
