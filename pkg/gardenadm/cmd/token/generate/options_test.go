// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package generate_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/gardenadm/cmd/token/generate"
	tokenutils "github.com/gardener/gardener/pkg/gardenadm/cmd/token/utils"
)

var _ = Describe("Options", func() {
	var (
		createOptions *tokenutils.Options
		options       *Options
	)

	BeforeEach(func() {
		createOptions = &tokenutils.Options{
			Validity: time.Hour,
		}
		options = &Options{CreateOptions: createOptions}
	})

	Describe("#ParseArgs", func() {
		It("should return nil", func() {
			Expect(options.ParseArgs(nil)).To(Succeed())
		})
	})

	Describe("#Validate", func() {
		It("should return nil", func() {
			Expect(options.Validate()).To(Succeed())
		})
	})

	Describe("#Complete", func() {
		It("should return nil", func() {
			Expect(options.Complete()).To(Succeed())
		})
	})
})
