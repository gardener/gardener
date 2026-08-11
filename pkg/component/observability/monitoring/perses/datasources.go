// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package perses

import (
	"fmt"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	persescommon "github.com/perses/spec/go/common"
	"github.com/perses/spec/go/datasource"
	persesplugin "github.com/perses/spec/go/plugin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	victorialogsconstants "github.com/gardener/gardener/pkg/component/observability/logging/victorialogs/constants"
)

func (p *perses) datasources() []client.Object {
	var datasources []client.Object

	if p.values.IsGardenCluster {
		datasources = append(datasources,
			p.newDatasource("prometheus-garden", "PrometheusDatasource", "http://prometheus-garden:80", true),
			p.newDatasource("prometheus-longterm", "PrometheusDatasource", "http://prometheus-longterm:81", false),
		)
	} else if p.values.ClusterType == component.ClusterTypeSeed {
		datasources = append(datasources,
			p.newDatasource("prometheus-aggregate", "PrometheusDatasource", "http://prometheus-aggregate:80", !p.values.OnlyDeployDatasourcesAndDashboards),
			p.newDatasource("prometheus-seed", "PrometheusDatasource", "http://prometheus-seed:80", false),
		)
	}

	if p.values.VictoriaLogsEnabled {
		datasources = append(datasources,
			p.newDatasource("victorialogs", "VictoriaLogsDatasource", fmt.Sprintf("http://%s.%s.svc:%d", victorialogsconstants.ServiceName, p.namespace, victorialogsconstants.VictoriaLogsPort), false),
		)
	}

	return datasources
}

func (p *perses) instanceSelector() *metav1.LabelSelector {
	instanceName := p.persesName()
	// In OnlyDeployDatasourcesAndDashboards mode (seed-is-garden), datasources must target the garden
	// Perses instance since no separate seed instance is deployed.
	if p.values.OnlyDeployDatasourcesAndDashboards {
		instanceName = "perses-garden"
	}

	return &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      "app.kubernetes.io/instance",
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{instanceName},
		}},
	}
}

// newDatasource returns a project-scoped PersesDatasource. The perses-operator maps the resource's namespace to a
// Perses project of the same name, so the datasource is explorable within that project.
func (p *perses) newDatasource(dsName, pluginKind, url string, isDefault bool) *persesv1alpha2.PersesDatasource {
	return &persesv1alpha2.PersesDatasource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dsName,
			Namespace: p.namespace,
			Labels:    p.getLabels(),
		},
		Spec: persesv1alpha2.DatasourceSpec{
			Config: persesv1alpha2.Datasource{
				Spec: datasource.Spec{
					Display: &persescommon.Display{
						Name: dsName,
					},
					Default: isDefault,
					Plugin: persesplugin.Plugin{
						Kind: pluginKind,
						Spec: map[string]any{
							"proxy": map[string]any{
								"kind": "HTTPProxy",
								"spec": map[string]any{
									"url": url,
								},
							},
						},
					},
				},
			},
			InstanceSelector: p.instanceSelector(),
		},
	}
}
