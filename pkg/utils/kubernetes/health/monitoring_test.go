// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("Monitoring", func() {
	DescribeTable("CheckPrometheus",
		func(prometheus *monitoringv1.Prometheus, matcher types.GomegaMatcher) {
			err := health.CheckPrometheus(prometheus)
			Expect(err).To(matcher)
		},
		Entry("healthy", &monitoringv1.Prometheus{
			Spec:   monitoringv1.PrometheusSpec{CommonPrometheusFields: monitoringv1.CommonPrometheusFields{Replicas: ptr.To[int32](1)}},
			Status: monitoringv1.PrometheusStatus{Replicas: 1, AvailableReplicas: 1, Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionTrue}}},
		}, BeNil()),
		Entry("healthy with nil replicas", &monitoringv1.Prometheus{
			Status: monitoringv1.PrometheusStatus{Replicas: 1, AvailableReplicas: 1, Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionTrue}}},
		}, BeNil()),
		Entry("not observed at latest version", &monitoringv1.Prometheus{
			ObjectMeta: metav1.ObjectMeta{Generation: 1},
			Status:     monitoringv1.PrometheusStatus{Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionTrue}}},
		}, MatchError(ContainSubstring("observed generation outdated (0/1)"))),
		Entry("condition missing", &monitoringv1.Prometheus{}, MatchError(ContainSubstring(`condition "Available" is missing`))),
		Entry("condition False", &monitoringv1.Prometheus{
			Status: monitoringv1.PrometheusStatus{Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionFalse}}},
		}, MatchError(ContainSubstring(`condition "Available" has invalid status False (expected True)`))),
		Entry("not enough ready replicas", &monitoringv1.Prometheus{
			Spec:   monitoringv1.PrometheusSpec{CommonPrometheusFields: monitoringv1.CommonPrometheusFields{Replicas: ptr.To[int32](2)}},
			Status: monitoringv1.PrometheusStatus{Replicas: 1, AvailableReplicas: 1, Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionTrue}}},
		}, MatchError(ContainSubstring(`not enough available replicas (1/2)`))),
	)

	Describe("IsPrometheusProgressing", func() {
		var (
			prometheus *monitoringv1.Prometheus
		)

		BeforeEach(func() {
			prometheus = &monitoringv1.Prometheus{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 42,
				},
				Spec: monitoringv1.PrometheusSpec{
					CommonPrometheusFields: monitoringv1.CommonPrometheusFields{Replicas: ptr.To[int32](3)},
				},
				Status: monitoringv1.PrometheusStatus{
					Conditions:      []monitoringv1.Condition{{Type: monitoringv1.Reconciled, Status: monitoringv1.ConditionTrue, ObservedGeneration: 42}},
					UpdatedReplicas: 3,
				},
			}
		})

		It("should return false if it is fully rolled out", func() {
			progressing, reason := health.IsPrometheusProgressing(prometheus)
			Expect(progressing).To(BeFalse())
			Expect(reason).To(Equal("Prometheus is fully rolled out"))
		})

		It("should return true if observedGeneration is outdated", func() {
			prometheus.Status.Conditions[0].ObservedGeneration--

			progressing, reason := health.IsPrometheusProgressing(prometheus)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("observed generation outdated (41/42)"))
		})

		It("should return true if replicas still need to be updated", func() {
			prometheus.Status.UpdatedReplicas--

			progressing, reason := health.IsPrometheusProgressing(prometheus)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("2 of 3 replica(s) have been updated"))
		})

		It("should return true if replica still needs to be updated (spec.replicas=null)", func() {
			prometheus.Spec.Replicas = nil
			prometheus.Status.UpdatedReplicas = 0

			progressing, reason := health.IsPrometheusProgressing(prometheus)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("0 of 1 replica(s) have been updated"))
		})
	})

	DescribeTable("CheckAlertmanager",
		func(alertManager *monitoringv1.Alertmanager, matcher types.GomegaMatcher) {
			err := health.CheckAlertmanager(alertManager)
			Expect(err).To(matcher)
		},
		Entry("healthy", &monitoringv1.Alertmanager{
			Spec:   monitoringv1.AlertmanagerSpec{Replicas: ptr.To[int32](1)},
			Status: monitoringv1.AlertmanagerStatus{Replicas: 1, AvailableReplicas: 1, Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionTrue}}},
		}, BeNil()),
		Entry("healthy with nil replicas", &monitoringv1.Alertmanager{
			Status: monitoringv1.AlertmanagerStatus{Replicas: 1, AvailableReplicas: 1, Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionTrue}}},
		}, BeNil()),
		Entry("not observed at latest version", &monitoringv1.Alertmanager{
			ObjectMeta: metav1.ObjectMeta{Generation: 1},
			Status:     monitoringv1.AlertmanagerStatus{Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionTrue}}},
		}, MatchError(ContainSubstring("observed generation outdated (0/1)"))),
		Entry("condition missing", &monitoringv1.Alertmanager{}, MatchError(ContainSubstring(`condition "Available" is missing`))),
		Entry("condition False", &monitoringv1.Alertmanager{
			Status: monitoringv1.AlertmanagerStatus{Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionFalse}}},
		}, MatchError(ContainSubstring(`condition "Available" has invalid status False (expected True)`))),
		Entry("not enough ready replicas", &monitoringv1.Alertmanager{
			Spec:   monitoringv1.AlertmanagerSpec{Replicas: ptr.To[int32](2)},
			Status: monitoringv1.AlertmanagerStatus{Replicas: 1, AvailableReplicas: 1, Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionTrue}}},
		}, MatchError(ContainSubstring(`not enough available replicas (1/2)`))),
	)

	Describe("IsAlertmanagerProgressing", func() {
		var (
			alertManager *monitoringv1.Alertmanager
		)

		BeforeEach(func() {
			alertManager = &monitoringv1.Alertmanager{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 42,
				},
				Spec: monitoringv1.AlertmanagerSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: monitoringv1.AlertmanagerStatus{
					Conditions:      []monitoringv1.Condition{{Type: monitoringv1.Reconciled, Status: monitoringv1.ConditionTrue, ObservedGeneration: 42}},
					UpdatedReplicas: 3,
				},
			}
		})

		It("should return false if it is fully rolled out", func() {
			progressing, reason := health.IsAlertmanagerProgressing(alertManager)
			Expect(progressing).To(BeFalse())
			Expect(reason).To(Equal("Alertmanager is fully rolled out"))
		})

		It("should return true if observedGeneration is outdated", func() {
			alertManager.Status.Conditions[0].ObservedGeneration--

			progressing, reason := health.IsAlertmanagerProgressing(alertManager)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("observed generation outdated (41/42)"))
		})

		It("should return true if replicas still need to be updated", func() {
			alertManager.Status.UpdatedReplicas--

			progressing, reason := health.IsAlertmanagerProgressing(alertManager)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("2 of 3 replica(s) have been updated"))
		})

		It("should return true if replica still needs to be updated (spec.replicas=null)", func() {
			alertManager.Spec.Replicas = nil
			alertManager.Status.UpdatedReplicas = 0

			progressing, reason := health.IsAlertmanagerProgressing(alertManager)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("0 of 1 replica(s) have been updated"))
		})
	})

	Describe("IsPrometheusHealthy", func() {
		var (
			server          *httptest.Server
			endpoint        string
			port            int
			responseHandler func(w http.ResponseWriter)

			createResponseHandler = func(statusCode int, response map[string]any) func(w http.ResponseWriter) {
				return func(w http.ResponseWriter) {
					w.WriteHeader(statusCode)
					if err := json.NewEncoder(w).Encode(response); err != nil {
						http.Error(w, "failed to marshal response: "+err.Error(), http.StatusInternalServerError)
						return
					}
				}
			}
		)

		BeforeEach(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := r.URL.Path
				query := r.FormValue("query")
				if path != "/api/v1/query" || query != "healthcheck:up" {
					http.Error(w, "bad request: "+path+"?query="+query, http.StatusBadRequest)
					return
				}

				// delegate to test-specific handler
				if responseHandler != nil {
					responseHandler(w)
				}
			}))

			parsedURL, err := url.Parse(server.URL)
			Expect(err).NotTo(HaveOccurred())

			endpoint = parsedURL.Hostname()
			port, err = strconv.Atoi(parsedURL.Port())
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			if server != nil {
				server.Close()
			}

			// reset test-specific handler
			responseHandler = nil
		})

		It("should return false when healthcheck:up is 0", func() {
			response := map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "vector",
					"result": []map[string]any{
						{
							"metric": map[string]any{"__name__": "healthcheck:up", "task": "foo"},
							"value":  []any{float64(time.Now().Unix()), "0"},
						},
						{
							"metric": map[string]any{"__name__": "healthcheck:up", "task": "bar"},
							"value":  []any{float64(time.Now().Unix()), "0"},
						},
					},
				},
			}
			responseHandler = createResponseHandler(http.StatusOK, response)

			result, err := health.IsPrometheusHealthy(context.Background(), endpoint, port)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.IsHealthy).To(BeFalse())
			Expect(result.Message).To(Equal(`healthcheck:up{task="bar"} => 0, healthcheck:up{task="foo"} => 0`))
		})

		It("should return true when healthcheck:up is 1", func() {
			response := map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "vector",
					"result": []map[string]any{{
						"metric": map[string]string{"__name__": "healthcheck:up"},
						"value":  []any{float64(time.Now().Unix()), "1"},
					}},
				},
			}
			responseHandler = createResponseHandler(http.StatusOK, response)

			result, err := health.IsPrometheusHealthy(context.Background(), endpoint, port)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.IsHealthy).To(BeTrue())
			Expect(result.Message).To(Equal(""))
		})

		It("should handle prometheus error responses", func() {
			response := map[string]any{
				"status": "error",
				"error":  "invalid query",
			}
			responseHandler = createResponseHandler(http.StatusBadRequest, response)

			result, err := health.IsPrometheusHealthy(context.Background(), endpoint, port)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid query"))
			Expect(result.IsHealthy).To(BeFalse())
			Expect(result.Message).To(Equal(""))
		})

		It("should handle warnings in prometheus response", func() {
			response := map[string]any{
				"status":   "success",
				"warnings": []string{"some warning", "another warning"},
				"data": map[string]any{
					"resultType": "vector",
					"result": []map[string]any{{
						"metric": map[string]string{"__name__": "healthcheck:up"},
						"value":  []any{float64(time.Now().Unix()), "1"},
					}},
				},
			}
			responseHandler = createResponseHandler(http.StatusOK, response)

			result, err := health.IsPrometheusHealthy(context.Background(), endpoint, port)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("query returned warnings: some warning, another warning"))
			Expect(result.IsHealthy).To(BeFalse())
			Expect(result.Message).To(Equal(""))
		})

		It("should handle unexpected result type", func() {
			response := map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "matrix",
					"result":     []any{},
				},
			}
			responseHandler = createResponseHandler(http.StatusOK, response)

			result, err := health.IsPrometheusHealthy(context.Background(), endpoint, port)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("query returned an unexpected result type: matrix"))
			Expect(result.IsHealthy).To(BeFalse())
			Expect(result.Message).To(Equal(""))
		})

		It("should handle empty vector response", func() {
			response := map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "vector",
					"result":     []any{},
				},
			}
			responseHandler = createResponseHandler(http.StatusOK, response)

			result, err := health.IsPrometheusHealthy(context.Background(), endpoint, port)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.IsHealthy).To(BeFalse())
			Expect(result.Message).To(Equal("health check recording rules are not deployed or running yet"))
		})

		It("should handle vector response with unexpected samples", func() {
			response := map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "vector",
					"result": []map[string]any{
						{
							"metric": map[string]any{"__name__": "healthcheck:up", "task": "foo"},
							"value":  []any{float64(time.Now().Unix()), "2"},
						},
						{
							"metric": map[string]any{"__name__": "healthcheck:up", "task": "bar"},
							"value":  []any{float64(time.Now().Unix()), "2"},
						},
					},
				},
			}
			responseHandler = createResponseHandler(http.StatusOK, response)

			result, err := health.IsPrometheusHealthy(context.Background(), endpoint, port)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(`query returned inconsistent sample values: healthcheck:up{task="bar"} => 2, healthcheck:up{task="foo"} => 2`))
			Expect(result.IsHealthy).To(BeFalse())
			Expect(result.Message).To(Equal(""))
		})

		It("should handle vector response with multiple healthy samples", func() {
			response := map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "vector",
					"result": []map[string]any{
						{
							"metric": map[string]any{"__name__": "healthcheck:up", "task": "foo"},
							"value":  []any{float64(time.Now().Unix()), "1"},
						},
						{
							"metric": map[string]any{"__name__": "healthcheck:up"},
							"value":  []any{float64(time.Now().Unix()), "1"},
						},
					},
				},
			}
			responseHandler = createResponseHandler(http.StatusOK, response)

			result, err := health.IsPrometheusHealthy(context.Background(), endpoint, port)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(`query returned inconsistent sample values: healthcheck:up => 1, healthcheck:up{task="foo"} => 1`))
			Expect(result.IsHealthy).To(BeFalse())
			Expect(result.Message).To(Equal(""))
		})

		It("should handle vector response with healthy and unhealthy samples", func() {
			response := map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "vector",
					"result": []map[string]any{
						{
							"metric": map[string]any{"__name__": "healthcheck:up", "task": "foo"},
							"value":  []any{float64(time.Now().Unix()), "0"},
						},
						{
							"metric": map[string]any{"__name__": "healthcheck:up"},
							"value":  []any{float64(time.Now().Unix()), "1"},
						},
					},
				},
			}
			responseHandler = createResponseHandler(http.StatusOK, response)

			result, err := health.IsPrometheusHealthy(context.Background(), endpoint, port)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(`query returned inconsistent sample values: healthcheck:up => 1, healthcheck:up{task="foo"} => 0`))
			Expect(result.IsHealthy).To(BeFalse())
			Expect(result.Message).To(Equal(""))
		})

		It("should handle truncated health check failing messages", func() {
			var failingMetrics []map[string]any
			for i := range 20 {
				failingMetrics = append(failingMetrics, map[string]any{
					"metric": map[string]any{"__name__": "healthcheck:up", "task": fmt.Sprintf("task-%02d", i)},
					"value":  []any{float64(time.Now().Unix()), "0"},
				})
			}
			response := map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "vector",
					"result":     failingMetrics,
				},
			}
			responseHandler = createResponseHandler(http.StatusOK, response)

			// show the expectation is truncated to 500 characters
			expectedMessage := `healthcheck:up{task="task-00"} => 0, healthcheck:up{task="task-01"} => 0, healthcheck:up{task="task-02"} => 0, ` +
				`healthcheck:up{task="task-03"} => 0, healthcheck:up{task="task-04"} => 0, healthcheck:up{task="task-05"} => 0, ` +
				`healthcheck:up{task="task-06"} => 0, healthcheck:up{task="task-07"} => 0, healthcheck:up{task="task-08"} => 0, ` +
				`healthcheck:up{task="task-09"} => 0, healthcheck:up{task="task-10"} => 0, healthcheck:up{task="task-11"} => 0, ` +
				`healthcheck:up{task="task-12"} => 0, healthcheck:up{t...`
			Expect(expectedMessage).To(HaveLen(500))

			result, err := health.IsPrometheusHealthy(context.Background(), endpoint, port)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.IsHealthy).To(BeFalse())
			Expect(result.Message).To(Equal(expectedMessage))
		})

		It("should handle truncated health check erroring messages", func() {
			var failingMetrics []map[string]any
			for i := range 20 {
				failingMetrics = append(failingMetrics, map[string]any{
					"metric": map[string]any{"__name__": "healthcheck:up", "task": fmt.Sprintf("task-%02d", i)},
					"value":  []any{float64(time.Now().Unix()), "2"},
				})
			}
			response := map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "vector",
					"result":     failingMetrics,
				},
			}
			responseHandler = createResponseHandler(http.StatusOK, response)

			// show the expectation is truncated to 500 characters
			expectedMessage := `healthcheck:up{task="task-00"} => 2, healthcheck:up{task="task-01"} => 2, healthcheck:up{task="task-02"} => 2, ` +
				`healthcheck:up{task="task-03"} => 2, healthcheck:up{task="task-04"} => 2, healthcheck:up{task="task-05"} => 2, ` +
				`healthcheck:up{task="task-06"} => 2, healthcheck:up{task="task-07"} => 2, healthcheck:up{task="task-08"} => 2, ` +
				`healthcheck:up{task="task-09"} => 2, healthcheck:up{task="task-10"} => 2, healthcheck:up{task="task-11"} => 2, ` +
				`healthcheck:up{task="task-12"} => 2, healthcheck:up{t...`
			Expect(expectedMessage).To(HaveLen(500))

			result, err := health.IsPrometheusHealthy(context.Background(), endpoint, port)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("query returned inconsistent sample values: " + expectedMessage))
			Expect(result.IsHealthy).To(BeFalse())
			Expect(result.Message).To(Equal(""))
		})

		It("should handle timeouts", func() {
			serverCtx, serverCancel := context.WithCancel(context.Background())
			defer serverCancel()

			response := map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "vector",
					"result": []map[string]any{{
						"metric": map[string]string{"__name__": "healthcheck:up"},
						"value":  []any{float64(time.Now().Unix()), "1"},
					}},
				},
			}
			responseHandler = func(w http.ResponseWriter) {
				// cancel sleep if serverCtx is canceled
				select {
				case <-serverCtx.Done():
					return
				case <-time.After(1 * time.Second):
				}
				createResponseHandler(http.StatusOK, response)(w)
			}

			// use a short-lived context to trigger a timeout
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()

			result, err := health.IsPrometheusHealthy(ctx, endpoint, port)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprintf(`query failed: Post "http://%s:%d/api/v1/query": context deadline exceeded`, endpoint, port)))
			Expect(result.IsHealthy).To(BeFalse())
		})
	})
})
