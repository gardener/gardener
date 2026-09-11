// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package perses

import (
	"net/http"
	"regexp"
	"testing"
)

// endpointAllowed reproduces how Perses evaluates the datasource proxy allow-list: it compiles each
// endpointPattern as an (unanchored) regular expression and, for a matching HTTP method, checks whether the pattern
// matches anywhere in the request path via regexp.FindAllString. See
// github.com/perses/perses/internal/api/impl/proxy/proxy.go.
func endpointAllowed(entries []map[string]any, method, path string) bool {
	for _, entry := range entries {
		if entry["method"] != method {
			continue
		}
		if len(regexp.MustCompile(entry["endpointPattern"].(string)).FindAllString(path, -1)) > 0 {
			return true
		}
	}
	return false
}

func TestAllowedEndpointsForPluginAreAnchored(t *testing.T) {
	prometheus := allowedEndpointsForPlugin(pluginKindPrometheus)

	allowed := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/query"},
		{http.MethodPost, "/api/v1/query"},
		{http.MethodGet, "/api/v1/query_range"},
		{http.MethodGet, "/api/v1/label/instance/values"},
	}
	for _, tc := range allowed {
		if !endpointAllowed(prometheus, tc.method, tc.path) {
			t.Errorf("expected %s %s to be allowed for the Prometheus plugin, but it was rejected", tc.method, tc.path)
		}
	}

	// Anchoring must reject paths that merely contain an allowed endpoint. Without "^...$" these would slip through
	// because Perses matches the pattern as an unanchored substring, letting a caller reach write/admin endpoints.
	rejected := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/query/../../admin/tsdb/delete_series"},
		{http.MethodGet, "/api/v1/metadata/../admin"},
		{http.MethodPost, "/api/v1/write"},
		{http.MethodGet, "/prefix/api/v1/query"},
	}
	for _, tc := range rejected {
		if endpointAllowed(prometheus, tc.method, tc.path) {
			t.Errorf("expected %s %s to be rejected for the Prometheus plugin, but it was allowed", tc.method, tc.path)
		}
	}

	victoriaLogs := allowedEndpointsForPlugin(pluginKindVictoriaLogs)
	if !endpointAllowed(victoriaLogs, http.MethodPost, "/select/logsql/query") {
		t.Error("expected POST /select/logsql/query to be allowed for the VictoriaLogs plugin, but it was rejected")
	}
	if endpointAllowed(victoriaLogs, http.MethodPost, "/select/logsql/query/../../admin") {
		t.Error("expected POST /select/logsql/query/../../admin to be rejected for the VictoriaLogs plugin, but it was allowed")
	}
}
