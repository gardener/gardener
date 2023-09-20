// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package original_test

import (
	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/pointer"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	mockcomponents "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/mock"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

var _ = Describe("Original", func() {
	var (
		ctrl       *gomock.Controller
		component1 *mockcomponents.MockComponent
		component2 *mockcomponents.MockComponent
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		component1 = mockcomponents.NewMockComponent(ctrl)
		component2 = mockcomponents.NewMockComponent(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Config", func() {
		var (
			caBundle                = pointer.String("cabundle")
			criName                 = extensionsv1alpha1.CRIName("foo")
			images                  = map[string]*imagevector.Image{}
			kubeletCABundle         = []byte("kubelet-ca-bundle")
			kubeletCLIFlags         = components.ConfigurableKubeletCLIFlags{}
			kubeletConfigParameters = components.ConfigurableKubeletConfigParameters{}
			kubeletDataVolumeName   = pointer.String("datavolname")
			kubernetesVersion       = semver.MustParse("1.2.3")
			sshPublicKeys           = []string{"ssh-public-key-a", "ssh-public-key-b"}

			ctx = components.Context{
				CABundle:                caBundle,
				CRIName:                 criName,
				Images:                  images,
				KubeletCABundle:         kubeletCABundle,
				KubeletCLIFlags:         kubeletCLIFlags,
				KubeletConfigParameters: kubeletConfigParameters,
				KubeletDataVolumeName:   kubeletDataVolumeName,
				KubernetesVersion:       kubernetesVersion,
				SSHPublicKeys:           sshPublicKeys,
			}

			unit1 = extensionsv1alpha1.Unit{Name: "1"}
			unit2 = extensionsv1alpha1.Unit{Name: "2"}
			unit3 = extensionsv1alpha1.Unit{Name: "3"}
			file1 = extensionsv1alpha1.File{Path: "1"}
			file2 = extensionsv1alpha1.File{Path: "2"}
			file3 = extensionsv1alpha1.File{Path: "3"}
		)

		It("should call the Config() functions of all components and return the units", func() {
			oldComponentsFn := ComponentsFn
			defer func() { ComponentsFn = oldComponentsFn }()
			ComponentsFn = func(extensionsv1alpha1.CRIName, bool) []components.Component {
				return []components.Component{component1, component2}
			}

			gomock.InOrder(
				component1.EXPECT().Config(ctx).Return(
					[]extensionsv1alpha1.Unit{unit1},
					[]extensionsv1alpha1.File{file2, file3},
					nil,
				),
				component2.EXPECT().Config(ctx).Return(
					[]extensionsv1alpha1.Unit{unit2, unit3},
					[]extensionsv1alpha1.File{file1},
					nil,
				),
			)

			units, files, err := Config(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(units).To(Equal([]extensionsv1alpha1.Unit{unit1, unit2, unit3}))
			Expect(files).To(Equal([]extensionsv1alpha1.File{file2, file3, file1}))
		})
	})

	Describe("#Components", func() {

		It("should compute the units and files w/ docker", func() {
			var order []string
			for _, component := range Components(extensionsv1alpha1.CRINameDocker, true) {
				order = append(order, component.Name())
			}

			Expect(order).To(Equal([]string{
				"valitail",
				"var-lib-mount",
				"root-certificates",
				"docker",
				"journald",
				"kernel-config",
				"kubelet",
				"sshd-ensurer",
				"gardener-user",
			}))
		})

		It("should compute the units and files w/ docker", func() {
			var order []string
			for _, component := range Components(extensionsv1alpha1.CRINameContainerD, true) {
				order = append(order, component.Name())
			}

			Expect(order).To(Equal([]string{
				"valitail",
				"var-lib-mount",
				"root-certificates",
				"containerd",
				"journald",
				"kernel-config",
				"kubelet",
				"sshd-ensurer",
				"gardener-user",
				"containerd-initializer",
			}))
		})

		It("should compute the units and files without gardener-user because SSH is disabled", func() {
			var order []string
			for _, component := range Components(extensionsv1alpha1.CRINameContainerD, false) {
				order = append(order, component.Name())
			}

			Expect(order).To(Equal([]string{
				"valitail",
				"var-lib-mount",
				"root-certificates",
				"containerd",
				"journald",
				"kernel-config",
				"kubelet",
				"sshd-ensurer",
				"containerd-initializer",
			}))
		})
	})
})
