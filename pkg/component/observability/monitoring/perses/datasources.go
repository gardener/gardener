// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package perses

import (
	"fmt"
	"net/http"

	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	persescommon "github.com/perses/spec/go/common"
	"github.com/perses/spec/go/datasource"
	persesplugin "github.com/perses/spec/go/plugin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	victorialogsconstants "github.com/gardener/gardener/pkg/component/observability/logging/victorialogs/constants"
)

const (
	pluginKindPrometheus   = "PrometheusDatasource"
	pluginKindVictoriaLogs = "VictoriaLogsDatasource"
)

func (p *perses) datasources() []client.Object {
	var datasources []client.Object

	if p.values.IsGardenCluster {
		datasources = append(datasources,
			p.newDatasource("prometheus-garden", pluginKindPrometheus, "http://prometheus-garden:80", true),
			p.newDatasource("prometheus-longterm", pluginKindPrometheus, "http://prometheus-longterm:81", false),
		)
	} else if p.values.ClusterType == component.ClusterTypeSeed {
		datasources = append(datasources,
			p.newDatasource("prometheus-aggregate", pluginKindPrometheus, "http://prometheus-aggregate:80", !p.values.OnlyDeployDatasourcesAndDashboards),
			p.newDatasource("prometheus-seed", pluginKindPrometheus, "http://prometheus-seed:80", false),
		)
	}

	if p.values.VictoriaLogsEnabled {
		datasources = append(datasources,
			p.newDatasource("victorialogs", pluginKindVictoriaLogs, fmt.Sprintf("http://%s.%s.svc:%d", victorialogsconstants.ServiceName, p.namespace, victorialogsconstants.VictoriaLogsPort), false),
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
									// Restrict what can be proxied to the datasource. The Perses proxy forwards any
									// request under the datasource path (with any method, and attaching the
									// datasource's stored credentials) to the upstream. Without an allow-list, a user
									// reaching the un-authenticated Perses API could use the proxy to send writes
									// (e.g. Prometheus remote-write) or reach other upstream endpoints. The allow-list
									// is limited to the read-only query endpoints the corresponding Perses plugin uses.
									"allowedEndpoints": allowedEndpointsForPlugin(pluginKind),
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

// allowedEndpointsForPlugin returns the read-only upstream endpoints the given datasource plugin needs to be able to
// proxy to. Each entry pairs an endpoint pattern (regular expression) with a single HTTP method, so endpoints served
// via both GET and POST are listed twice.
func allowedEndpointsForPlugin(pluginKind string) []map[string]any {
	endpoint := func(pattern string, methods ...string) []map[string]any {
		entries := make([]map[string]any, 0, len(methods))
		for _, method := range methods {
			entries = append(entries, map[string]any{"endpointPattern": pattern, "method": method})
		}
		return entries
	}

	switch pluginKind {
	case pluginKindPrometheus:
		var endpoints []map[string]any
		// The Prometheus plugin queries via GET and POST (POST is used for large requests).
		for _, pattern := range []string{
			"/api/v1/query",
			"/api/v1/query_range",
			"/api/v1/labels",
			"/api/v1/label/[a-zA-Z0-9_]+/values",
			"/api/v1/series",
			"/api/v1/metadata",
			"/api/v1/parse_query",
		} {
			endpoints = append(endpoints, endpoint(pattern, http.MethodGet, http.MethodPost)...)
		}
		return endpoints
	case pluginKindVictoriaLogs:
		var endpoints []map[string]any
		// The VictoriaLogs plugin queries exclusively via POST.
		for _, pattern := range []string{
			"/select/logsql/query",
			"/select/logsql/stats_query_range",
			"/select/logsql/field_names",
			"/select/logsql/field_values",
		} {
			endpoints = append(endpoints, endpoint(pattern, http.MethodPost)...)
		}
		return endpoints
	}

	return nil
}
