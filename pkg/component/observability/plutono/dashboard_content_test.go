// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package plutono_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/component/observability/plutono"
)

// getDashboardData reads all embedded dashboard JSON files and returns them as a map
// of filename → parsed JSON content.
func getDashboardData() map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{})
	allDashboards := GardenAndShootDashboards()
	_ = fs.WalkDir(allDashboards, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		data, readErr := fs.ReadFile(allDashboards, path)
		if readErr != nil {
			return nil
		}
		var obj map[string]interface{}
		if jsonErr := json.Unmarshal(data, &obj); jsonErr == nil {
			result[path] = obj
		}
		return nil
	})
	return result
}

// collectExpressionsFromPanel recursively walks a panel (and its nested panels)
// and collects all PromQL expression strings found in "targets".
func collectExpressionsFromPanel(panel interface{}) []string {
	var exprs []string
	m, ok := panel.(map[string]interface{})
	if !ok {
		return exprs
	}

	// Recurse into nested "panels"
	if nested, ok := m["panels"].([]interface{}); ok {
		for _, sub := range nested {
			exprs = append(exprs, collectExpressionsFromPanel(sub)...)
		}
	}

	// Collect from "targets"
	targets, ok := m["targets"].([]interface{})
	if !ok {
		return exprs
	}
	for _, t := range targets {
		target, ok := t.(map[string]interface{})
		if !ok {
			continue
		}
		if expr, ok := target["expr"].(string); ok {
			exprs = append(exprs, expr)
		}
	}
	return exprs
}

// collectAllExpressionsFromDashboard returns all PromQL expressions from all panels
// of a parsed dashboard JSON object.
func collectAllExpressionsFromDashboard(dashboard map[string]interface{}) []string {
	var exprs []string
	panels, ok := dashboard["panels"].([]interface{})
	if !ok {
		return exprs
	}
	for _, panel := range panels {
		exprs = append(exprs, collectExpressionsFromPanel(panel)...)
	}
	return exprs
}

var _ = Describe("Dashboard content", func() {

	// Regression test for https://github.com/gardener/gardener/issues/14788
	//
	// In Kubernetes 1.33 the etcd_request_duration_seconds_bucket metric no longer exposes
	// the "group" and "resource" labels. Instead it exposes a single "type" label.
	// Queries that use {group="$group",resource="$resource"} will return no data on k8s 1.33+.
	//
	// The fix is to use {type=~"$type"} instead.
	Describe("apiserver-storage-details dashboard", func() {
		var allExpressions []string

		BeforeEach(func() {
			dashboards := getDashboardData()

			// Find the apiserver-storage-details dashboard
			for path, dashboard := range dashboards {
				if strings.Contains(path, "apiserver-storage-details") {
					allExpressions = collectAllExpressionsFromDashboard(dashboard)
					break
				}
			}
		})

		It("should have at least one PromQL expression in the dashboard", func() {
			Expect(allExpressions).ToNot(BeEmpty(),
				"apiserver-storage-details dashboard must contain at least one PromQL expression")
		})

		It("should not use the removed 'group' label in etcd_request_duration_seconds_bucket queries", func() {
			for _, expr := range allExpressions {
				if strings.Contains(expr, "etcd_request_duration_seconds_bucket") {
					Expect(expr).NotTo(ContainSubstring(`group="`),
						fmt.Sprintf(
							"Query %q must not use the 'group' label "+
								"(removed in k8s 1.33, see https://github.com/gardener/gardener/issues/14788)",
							expr))
				}
			}
		})

		It("should not use the removed 'resource' label in etcd_request_duration_seconds_bucket queries", func() {
			for _, expr := range allExpressions {
				if strings.Contains(expr, "etcd_request_duration_seconds_bucket") {
					Expect(expr).NotTo(ContainSubstring(`resource="`),
						fmt.Sprintf(
							"Query %q must not use the 'resource' label "+
								"(removed in k8s 1.33, see https://github.com/gardener/gardener/issues/14788)",
							expr))
				}
			}
		})

		It("should use the 'type' label for filtering etcd_request_duration_seconds_bucket", func() {
			hasEtcdQuery := false
			for _, expr := range allExpressions {
				if strings.Contains(expr, "etcd_request_duration_seconds_bucket") {
					hasEtcdQuery = true
					Expect(expr).To(ContainSubstring("type=~"),
						fmt.Sprintf(
							"etcd_request_duration_seconds_bucket query %q must use type=~ label filter "+
								"(k8s 1.33 renamed resource/group → type, see https://github.com/gardener/gardener/issues/14788)",
							expr))
				}
			}
			Expect(hasEtcdQuery).To(BeTrue(),
				"apiserver-storage-details must contain at least one etcd_request_duration_seconds_bucket query")
		})
	})

	Describe("All dashboards with etcd_request_duration_seconds_bucket", func() {
		It("should not use the old 'group' or 'resource' label selectors", func() {
			dashboards := getDashboardData()
			for path, dashboard := range dashboards {
				exprs := collectAllExpressionsFromDashboard(dashboard)
				for _, expr := range exprs {
					if strings.Contains(expr, "etcd_request_duration_seconds_bucket") {
						Expect(expr).NotTo(ContainSubstring(`group="`),
							fmt.Sprintf(
								"Dashboard %s: query %q must not use the deprecated 'group' label "+
									"(removed in k8s 1.33)",
								path, expr))
						Expect(expr).NotTo(ContainSubstring(`resource="`),
							fmt.Sprintf(
								"Dashboard %s: query %q must not use the deprecated 'resource' label "+
									"(removed in k8s 1.33)",
								path, expr))
					}
				}
			}
		})
	})
})
