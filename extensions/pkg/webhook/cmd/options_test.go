// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
	"go.uber.org/mock/gomock"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	. "github.com/gardener/gardener/extensions/pkg/webhook/cmd"
	"github.com/gardener/gardener/pkg/utils/test"
)

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

			It("should not add any webhook when wildcard is used", func() {
				switches := NewSwitchOptions(
					Switch("foo", func(_ manager.Manager) (*extensionswebhook.Webhook, error) {
						return nil, nil
					}),
				)

				fs := pflag.NewFlagSet(commandName, pflag.ContinueOnError)
				switches.AddFlags(fs)

				err := fs.Parse(test.NewCommandBuilder(commandName).
					Flags(
						test.StringSliceFlag(DisableFlag, "*"),
					).
					Command().
					Slice())

				Expect(err).NotTo(HaveOccurred())
				Expect(switches.Complete()).NotTo(HaveOccurred())
				Expect(switches.Completed().WebhooksFactory(nil)).To(BeEmpty())
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
