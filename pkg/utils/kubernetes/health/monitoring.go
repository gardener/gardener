// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"context"
	"fmt"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prom "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// IsPrometheusHealthy checks for health issues in a Prometheus instance.
func IsPrometheusHealthy(ctx context.Context, endpoint string, port int) (bool, error) {
	client, err := prom.NewClient(prom.Config{Address: fmt.Sprintf("http://%s:%d", endpoint, port)})
	if err != nil {
		return false, fmt.Errorf("failed to create Prometheus client: %w", err)
	}

	v1api := promv1.NewAPI(client)

	// set a maximum timeout for the query, but callers can set a shorter timeout via the context
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, warnings, err := v1api.Query(ctx, "healthcheck:up", time.Now())
	if err != nil {
		return false, fmt.Errorf("query failed: %w", err)
	}

	if len(warnings) > 0 {
		return false, fmt.Errorf("query returned warnings")
	}

	if result.Type() != model.ValVector {
		return false, fmt.Errorf("query returned an unexpected result type")
	}

	vector := result.(model.Vector)
	if len(vector) == 0 {
		return false, fmt.Errorf("recording rules are not deployed or running yet")
	} else if len(vector) > 1 {
		return false, fmt.Errorf("query returned multiple results but a single value was expected")
	}

	isHealthy := vector[0].Value == 1
	return isHealthy, nil
}

// CheckPrometheus checks whether the given Prometheus is healthy.
func CheckPrometheus(prometheus *monitoringv1.Prometheus) error {
	if err := checkMonitoringCondition(prometheus.Status.Conditions, monitoringv1.Available, prometheus.Generation); err != nil {
		return err
	}

	if replicas := ptr.Deref(prometheus.Spec.Replicas, 1); prometheus.Status.AvailableReplicas < replicas {
		return fmt.Errorf("not enough available replicas (%d/%d)", prometheus.Status.AvailableReplicas, replicas)
	}
	return nil
}

// IsPrometheusProgressing returns false if the Prometheus has been fully rolled out. Otherwise, it returns true along
// with a reason, why the Prometheus is not considered to be fully rolled out.
func IsPrometheusProgressing(prometheus *monitoringv1.Prometheus) (bool, string) {
	if err := checkMonitoringCondition(prometheus.Status.Conditions, monitoringv1.Reconciled, prometheus.Generation); err != nil {
		return true, err.Error()
	}

	desiredReplicas, updatedReplicas := ptr.Deref(prometheus.Spec.Replicas, 1), prometheus.Status.UpdatedReplicas

	if updatedReplicas < desiredReplicas {
		return true, fmt.Sprintf("%d of %d replica(s) have been updated", updatedReplicas, desiredReplicas)
	}

	return false, "Prometheus is fully rolled out"
}

// CheckAlertmanager checks whether the given Alertmanager is healthy.
func CheckAlertmanager(alertManager *monitoringv1.Alertmanager) error {
	if err := checkMonitoringCondition(alertManager.Status.Conditions, monitoringv1.Available, alertManager.Generation); err != nil {
		return err
	}

	if replicas := ptr.Deref(alertManager.Spec.Replicas, 1); alertManager.Status.AvailableReplicas < replicas {
		return fmt.Errorf("not enough available replicas (%d/%d)", alertManager.Status.AvailableReplicas, replicas)
	}
	return nil
}

// IsAlertmanagerProgressing returns false if the Alertmanager has been fully rolled out. Otherwise, it returns true along
// with a reason, why the Alertmanager is not considered to be fully rolled out.
func IsAlertmanagerProgressing(alertManager *monitoringv1.Alertmanager) (bool, string) {
	if err := checkMonitoringCondition(alertManager.Status.Conditions, monitoringv1.Reconciled, alertManager.Generation); err != nil {
		return true, err.Error()
	}

	desiredReplicas, updatedReplicas := ptr.Deref(alertManager.Spec.Replicas, 1), alertManager.Status.UpdatedReplicas

	if updatedReplicas < desiredReplicas {
		return true, fmt.Sprintf("%d of %d replica(s) have been updated", updatedReplicas, desiredReplicas)
	}

	return false, "Alertmanager is fully rolled out"
}

func getMonitoringCondition(conditions []monitoringv1.Condition, conditionType monitoringv1.ConditionType) *monitoringv1.Condition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

func checkMonitoringCondition(conditions []monitoringv1.Condition, conditionType monitoringv1.ConditionType, generation int64) error {
	if condition := getMonitoringCondition(conditions, conditionType); condition == nil {
		return fmt.Errorf("condition %q is missing", conditionType)
	} else if condition.ObservedGeneration < generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", condition.ObservedGeneration, generation)
	} else if err := checkConditionState(string(condition.Type), string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
		return err
	}

	return nil
}
