// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"testing"

	"github.com/gardener/gardener/extensions/pkg/util/test"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
)

func TestCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webhook Cmd Suite")
}

var _ = Describe("Options", func() {
	var (
		ctrl *gomock.Controller
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("SwitchOptions", func() {
		const commandName = "test"

		Describe("#AddFlags", func() {
			It("should correctly parse the flags", func() {
				var (
					name1    = "foo"
					name2    = "bar"
					switches = NewSwitchOptions(
						Switch(name1, nil),
						Switch(name2, nil),
					)
				)

				fs := pflag.NewFlagSet(commandName, pflag.ContinueOnError)
				switches.AddFlags(fs)

				err := fs.Parse(test.NewCommandBuilder(commandName).
					Flags(
						test.StringSliceFlag(DisableFlag, name1, name2),
					).
					Command().
					Slice())

				Expect(err).NotTo(HaveOccurred())
				Expect(switches.Complete()).To(Succeed())

				Expect(switches.Disabled).To(Equal([]string{name1, name2}))
			})

			It("should error on an unknown webhook", func() {
				switches := NewSwitchOptions()

				fs := pflag.NewFlagSet(commandName, pflag.ContinueOnError)
				switches.AddFlags(fs)

				err := fs.Parse(test.NewCommandBuilder(commandName).
					Flags(
						test.StringSliceFlag(DisableFlag, "unknown"),
					).
					Command().
					Slice())

				Expect(err).NotTo(HaveOccurred())
				Expect(switches.Complete()).To(HaveOccurred())
			})
		})
	})
})
