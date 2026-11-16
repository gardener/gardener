// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package perses

import (
	"embed"
	"io/fs"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/sets"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
)

var _ = Describe("Embedded dashboards", func() {
	It("should only contain valid dashboards and no duplicate names across the embedded sets", func() {
		names := sets.New[string]()

		for _, dashboardEmbed := range []struct {
			path    string
			embedFS embed.FS
		}{
			{gardenDashboardsPath, gardenDashboards},
			{seedDashboardsPath, seedDashboards},
			{gardenAndSeedDashboardsPath, gardenAndSeedDashboards},
			{gardenAndShootDashboardsPath, gardenAndShootDashboards},
			{commonDashboardsPath, commonDashboards},
		} {
			Expect(fs.WalkDir(dashboardEmbed.embedFS, "dashboards", func(path string, dirEntry fs.DirEntry, err error) error {
				Expect(err).NotTo(HaveOccurred())
				if dirEntry.IsDir() {
					return nil
				}

				data, err := dashboardEmbed.embedFS.ReadFile(path)
				Expect(err).NotTo(HaveOccurred())

				name, config, err := decodeDashboard(data)
				Expect(err).NotTo(HaveOccurred(), "embedded dashboard %s cannot be decoded", path)
				Expect(utilvalidation.IsDNS1123Subdomain(name)).To(BeEmpty(), "embedded dashboard %s has invalid resource name %q", path, name)
				Expect(config.Panels).NotTo(BeEmpty(), "embedded dashboard %s (%q) defines no panels", path, name)
				Expect(names.Has(name)).To(BeFalse(), "embedded dashboard %s (%q) is defined more than once", path, name)
				names.Insert(name)

				return nil
			})).To(Succeed())
		}
	})
})
