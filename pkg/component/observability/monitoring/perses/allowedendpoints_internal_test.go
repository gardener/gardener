// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package perses

import (
	"net/http"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

var _ = Describe("allowedEndpointsForPlugin", func() {
	Context("Prometheus plugin", func() {
		var entries []map[string]any

		BeforeEach(func() {
			entries = allowedEndpointsForPlugin(pluginKindPrometheus)
		})

		DescribeTable("should allow the read-only query endpoints",
			func(method, path string) {
				Expect(endpointAllowed(entries, method, path)).To(BeTrue())
			},
			Entry("query via GET", http.MethodGet, "/api/v1/query"),
			Entry("query via POST", http.MethodPost, "/api/v1/query"),
			Entry("query_range", http.MethodGet, "/api/v1/query_range"),
			Entry("label values", http.MethodGet, "/api/v1/label/instance/values"),
		)

		// Anchoring must reject paths that merely contain an allowed endpoint. Without "^...$" these would slip
		// through because Perses matches the pattern as an unanchored substring, letting a caller reach write/admin
		// endpoints.
		DescribeTable("should reject paths that only contain an allowed endpoint",
			func(method, path string) {
				Expect(endpointAllowed(entries, method, path)).To(BeFalse())
			},
			Entry("path traversal past query", http.MethodGet, "/api/v1/query/../../admin/tsdb/delete_series"),
			Entry("path traversal past metadata", http.MethodGet, "/api/v1/metadata/../admin"),
			Entry("remote-write", http.MethodPost, "/api/v1/write"),
			Entry("allowed endpoint as a suffix", http.MethodGet, "/prefix/api/v1/query"),
		)
	})

	Context("VictoriaLogs plugin", func() {
		var entries []map[string]any

		BeforeEach(func() {
			entries = allowedEndpointsForPlugin(pluginKindVictoriaLogs)
		})

		It("should allow the query endpoint", func() {
			Expect(endpointAllowed(entries, http.MethodPost, "/select/logsql/query")).To(BeTrue())
		})

		It("should reject a path that only contains an allowed endpoint", func() {
			Expect(endpointAllowed(entries, http.MethodPost, "/select/logsql/query/../../admin")).To(BeFalse())
		})
	})
})
