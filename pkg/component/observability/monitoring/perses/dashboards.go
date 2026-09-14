// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package perses

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	persesv1 "github.com/perses/perses/pkg/model/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

var (
	//go:embed dashboards/garden
	gardenDashboards embed.FS
	//go:embed dashboards/seed
	seedDashboards embed.FS
	//go:embed dashboards/garden-seed
	gardenAndSeedDashboards embed.FS
	//go:embed dashboards/garden-shoot
	gardenAndShootDashboards embed.FS
	//go:embed dashboards/common
	commonDashboards embed.FS

	gardenDashboardsPath         = "dashboards/garden"
	seedDashboardsPath           = "dashboards/seed"
	gardenAndSeedDashboardsPath  = "dashboards/garden-seed"
	gardenAndShootDashboardsPath = "dashboards/garden-shoot"
	commonDashboardsPath         = "dashboards/common"
	commonVpaDashboardsPath      = commonDashboardsPath + "/vpa"
)

func (p *perses) dashboards() ([]client.Object, error) {
	dashboards, err := p.loadDashboards()
	if err != nil {
		return nil, err
	}

	var objs []client.Object
	for name, config := range dashboards {
		objs = append(objs, &persesv1alpha2.PersesDashboard{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: p.namespace,
				Labels:    p.getLabels(),
			},
			Spec: persesv1alpha2.PersesDashboardSpec{
				Config:           config,
				InstanceSelector: p.instanceSelector(),
			},
		})
	}

	return objs, nil
}

// loadDashboards reads the embedded dashboard resources relevant for the current role and returns them keyed by the
// PersesDashboard resource name (taken from the dashboard's metadata.name).
func (p *perses) loadDashboards() (map[string]persesv1alpha2.Dashboard, error) {
	requiredDashboards, ignorePaths := p.selectDashboards()

	dashboards := map[string]persesv1alpha2.Dashboard{}
	for dashboardPath, dashboardEmbed := range requiredDashboards {
		if err := fs.WalkDir(dashboardEmbed, dashboardPath, func(path string, dirEntry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if dirEntry.IsDir() {
				if ignorePaths.HasAny(strings.Split(path, "/")...) {
					return fs.SkipDir
				}
				return nil
			}

			data, err := dashboardEmbed.ReadFile(path)
			if err != nil {
				return fmt.Errorf("error reading %s: %w", path, err)
			}

			name, config, err := decodeDashboard(data)
			if err != nil {
				return fmt.Errorf("error decoding %s: %w", path, err)
			}

			dashboards[name] = config
			return nil
		}); err != nil {
			return nil, err
		}
	}

	return dashboards, nil
}

// decodeDashboard decodes an embedded Perses dashboard resource. It decodes into the upstream Perses Dashboard type
// whose unmarshalling validates the resource against the Perses schema (equivalent to `percli lint`), and returns the
// resource name (from metadata.name) together with the operator-flavoured dashboard config.
func decodeDashboard(data []byte) (string, persesv1alpha2.Dashboard, error) {
	var dashboard persesv1.Dashboard
	if err := yaml.Unmarshal(data, &dashboard); err != nil {
		return "", persesv1alpha2.Dashboard{}, err
	}
	return dashboard.Metadata.Name, persesv1alpha2.Dashboard{Spec: dashboard.Spec}, nil
}

// selectDashboards returns the set of embedded dashboard directories to deploy for the current role, together with the
// path segments that should be ignored (e.g. istio or vpa dashboards which are only conditionally deployed). Perses is
// only ever deployed to the garden runtime cluster and to seeds, so only these two roles are handled.
func (p *perses) selectDashboards() (map[string]embed.FS, sets.Set[string]) {
	ignorePaths := sets.New[string]()

	if p.values.IsGardenCluster {
		requiredDashboards := map[string]embed.FS{
			gardenDashboardsPath:         gardenDashboards,
			gardenAndSeedDashboardsPath:  gardenAndSeedDashboards,
			gardenAndShootDashboardsPath: gardenAndShootDashboards,
		}
		if p.values.VPAEnabled {
			requiredDashboards[commonVpaDashboardsPath] = commonDashboards
		}
		return requiredDashboards, ignorePaths
	}

	requiredDashboards := map[string]embed.FS{
		seedDashboardsPath:   seedDashboards,
		commonDashboardsPath: commonDashboards,
	}
	// If the seed is the garden cluster, the garden-seed dashboards are already deployed by gardener-operator, so the
	// gardenlet does not need to deploy them again.
	if !p.values.OnlyDeployDatasourcesAndDashboards {
		requiredDashboards[gardenAndSeedDashboardsPath] = gardenAndSeedDashboards
	}
	if !p.values.IncludeIstioDashboards {
		ignorePaths.Insert("istio")
	}
	if !p.values.VPAEnabled {
		ignorePaths.Insert("vpa")
	}
	return requiredDashboards, ignorePaths
}

func init() {
	// Fail fast at build/test time if any embedded dashboard cannot be decoded/validated, has no or an invalid resource
	// name, or defines no panels.
	for _, dashboardEmbed := range []embed.FS{gardenDashboards, seedDashboards, gardenAndSeedDashboards, gardenAndShootDashboards, commonDashboards} {
		if err := fs.WalkDir(dashboardEmbed, "dashboards", func(path string, dirEntry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if dirEntry.IsDir() {
				return nil
			}
			data, err := dashboardEmbed.ReadFile(path)
			if err != nil {
				return err
			}
			name, config, err := decodeDashboard(data)
			if err != nil {
				panic(fmt.Sprintf("embedded dashboard %s cannot be decoded: %v", path, err))
			}
			if errs := utilvalidation.IsDNS1123Subdomain(name); len(errs) > 0 {
				panic(fmt.Sprintf("embedded dashboard %s has invalid resource name %q: %v", path, name, errs))
			}
			if len(config.Panels) == 0 {
				panic(fmt.Sprintf("embedded dashboard %s (%q) defines no panels", path, name))
			}
			return nil
		}); err != nil {
			panic(err)
		}
	}
}
