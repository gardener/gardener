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

package controllerinstallation_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/registry/core/controllerinstallation"
)

func TestControllerInstallation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Registry ControllerInstallation Suite")
}

var _ = Describe("ToSelectableFields", func() {
	It("should return correct fields", func() {
		result := controllerinstallation.ToSelectableFields(newControllerInstallation())

		Expect(result).To(HaveLen(3))
		Expect(result.Has("metadata.name")).To(BeTrue())
		Expect(result.Get("metadata.name")).To(Equal("test"))
		Expect(result.Has(core.RegistrationRefName)).To(BeTrue())
		Expect(result.Get(core.RegistrationRefName)).To(Equal("baz"))
		Expect(result.Has(core.SeedRefName)).To(BeTrue())
		Expect(result.Get(core.SeedRefName)).To(Equal("qux"))
	})
})

var _ = Describe("GetAttrs", func() {
	It("should return error when object is not ControllerInstallation", func() {
		_, _, err := controllerinstallation.GetAttrs(&core.ControllerRegistration{})
		Expect(err).To(HaveOccurred())
	})

	It("should return correct result", func() {
		ls, fs, err := controllerinstallation.GetAttrs(newControllerInstallation())

		Expect(err).NotTo(HaveOccurred())
		Expect(ls).To(HaveLen(1))
		Expect(ls.Get("foo")).To(Equal("bar"))
		Expect(fs.Get(core.SeedRefName)).To(Equal("qux"))
		Expect(fs.Get(core.RegistrationRefName)).To(Equal("baz"))
	})
})

var _ = Describe("MatchControllerInstallation", func() {
	It("should return correct predicate", func() {
		ls, _ := labels.Parse("app=test")
		fs := fields.OneTermEqualSelector(core.SeedRefName, "foo")

		result := controllerinstallation.MatchControllerInstallation(ls, fs)

		Expect(result.Label).To(Equal(ls))
		Expect(result.Field).To(Equal(fs))
	})
})

var _ = Describe("#SeedRefNameIndexFunc", func() {
	It("should return the seed name", func() {
		result, err := controllerinstallation.SeedRefNameIndexFunc(newControllerInstallation())
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(ConsistOf("qux"))
	})
})

var _ = Describe("#RegistrationRefNameIndexFunc", func() {
	It("should return the registration name", func() {
		result, err := controllerinstallation.RegistrationRefNameIndexFunc(newControllerInstallation())
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(ConsistOf("baz"))
	})
})

func newControllerInstallation() *core.ControllerInstallation {
	return &core.ControllerInstallation{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test",
			Labels: map[string]string{"foo": "bar"},
		},
		Spec: core.ControllerInstallationSpec{
			RegistrationRef: corev1.ObjectReference{
				Name: "baz",
			},
			SeedRef: corev1.ObjectReference{
				Name: "qux",
			},
		},
	}
}
