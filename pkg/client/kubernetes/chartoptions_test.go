// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("chart options", func() {
	var (
		aopts *ApplyOptions
		dopts *DeleteOptions
	)

	BeforeEach(func() {
		aopts = &ApplyOptions{}
		dopts = &DeleteOptions{}
	})

	Describe("Values", func() {
		var vals ValueOption

		BeforeEach(func() {
			vals = Values("foo")
		})

		It("sets ApplyOptions", func() {
			vals.MutateApplyOptions(aopts)

			Expect(aopts.Values).To(Equal("foo"))
		})

		It("sets DeleteOptions", func() {
			vals.MutateDeleteOptions(dopts)

			Expect(dopts.Values).To(Equal("foo"))
		})
	})

	It("MergeFuncs sets ApplyOptions", func() {
		funcs := MergeFuncs{
			schema.GroupKind{}: func(n, o *unstructured.Unstructured) {
				n.SetName("baz")
			},
		}
		funcs.MutateApplyOptions(aopts)

		Expect(aopts.MergeFuncs).To(Equal(funcs))
	})

	Context("ForceNamespace", func() {
		It("sets ApplyOptions", func() {
			ForceNamespace.MutateApplyOptions(aopts)

			Expect(aopts.ForceNamespace).To(BeTrue())
		})

		It("sets DeleteOptions", func() {
			ForceNamespace.MutateDeleteOptions(dopts)

			Expect(dopts.ForceNamespace).To(BeTrue())
		})
	})

	Context("TolerateErrorFunc", func() {
		It("sets DeleteOptions", func() {
			var tTrue TolerateErrorFunc = func(_ error) bool { return true }
			tTrue.MutateDeleteOptions(dopts)

			Expect(dopts.TolerateErrorFuncs).To(HaveLen(1))
			Expect(dopts.TolerateErrorFuncs[0](nil)).To(BeTrue())

		})
	})
})
