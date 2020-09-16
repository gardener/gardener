// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"path/filepath"
	"testing"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/test"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestControllerManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Manager Suite")
}

var _ = Describe("Controller Manager", func() {
	Describe("ReadGlobalImageVector", func() {
		It("should read the global image vector with no override", func() {
			defer test.WithEnvVar(imagevector.OverrideEnv, "")()
			defer test.WithWd("../../..")()

			_, err := imagevector.ReadGlobalImageVectorWithEnvOverride(filepath.Join(common.ChartPath, DefaultImageVector))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
