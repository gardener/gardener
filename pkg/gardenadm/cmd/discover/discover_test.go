// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package discover_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	. "github.com/gardener/gardener/pkg/gardenadm/cmd/discover"
)

var _ = Describe("Discover", func() {
	Describe("#NewCommand", func() {
		It("should expose 'new' and 'existing' subcommands", func() {
			cmd := NewCommand(&cmd.Options{})

			subs := make(map[string]bool, len(cmd.Commands()))
			for _, sub := range cmd.Commands() {
				subs[sub.Name()] = true
			}

			Expect(subs).To(HaveKey("new"))
			Expect(subs).To(HaveKey("existing"))
		})
	})
})
