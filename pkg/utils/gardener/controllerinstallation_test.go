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

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("ControllerInstallation", func() {
	Describe("#NamespaceNameForControllerInstallation", func() {
		It("should return the correct namespace name for the ControllerInstallation", func() {
			controllerInstallation := &gardencorev1beta1.ControllerInstallation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			}
			Expect(NamespaceNameForControllerInstallation(controllerInstallation)).To(Equal("extension-foo"))
		})
	})
})
