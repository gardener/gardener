// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0package x509certificateexporter

package x509certificateexporter

import (
	"fmt"

	"go.yaml.in/yaml/v2"
)

func mapStrings(slice []string, fn func(string) string) []string {
	result := make([]string, len(slice))
	for i, v := range slice {
		result[i] = fn(v)
	}
	return result
}

func mapStringsWithVals(slice map[string]string, fn func(string, string) string) []string {
	results := make([]string, 0)
	for k, v := range slice {
		results = append(results, fn(k, v))
	}
	return results
}

func stringsToArgs(argName string, values []string) []string {
	return mapStrings(values, func(value string) string {
		return "--" + argName + "=" + value
	})
}

func mappedStringsToArgs(argName string, values map[string]string) []string {
	return mapStringsWithVals(values, func(k, v string) string {
		if v != "" {
			return fmt.Sprintf("--%s=%s=%s", argName, k, v)
		}
		return fmt.Sprintf("--%s=%s", argName, k)
	})
}

func boolToArg(flag string, enabled bool) []string {
	if bool(enabled) {
		return []string{flag}
	}
	return []string{}
}

func getExposeRelativeMetricsArg(expose bool) []string {
	return boolToArg("--expose-relative-metrics", expose)
}

func getExposePerCertErrorMetricsArg(expose bool) []string {
	return boolToArg("--expose-per-cert-error-metrics", expose)
}

func getExposeLabelsMetricsArg(expose bool) []string {
	return boolToArg("--expose-labels-metrics", expose)
}

func (a *alertingConfig) Default() {
	if a.CertificateExpirationDays == 0 {
		a.CertificateExpirationDays = defaultCertificateExpirationDays
	}
	if a.CertificateRenewalDays == 0 {
		a.CertificateRenewalDays = defaultCertificateRenewalDays
	}

	if a.ReadErrorsSeverity == "" {
		a.ReadErrorsSeverity = defaultReadErrorsSeverity
	}
	if a.CertificateErrorsSeverity == "" {
		a.CertificateErrorsSeverity = defaultCertificateErrorsSeverity
	}
	if a.RenewalSeverity == "" {
		a.RenewalSeverity = defaultRenewalSeverity
	}
	if a.ExpirationSeverity == "" {
		a.ExpirationSeverity = defaultExpirationSeverity
	}
	if a.ExpiresTodaySeverity == "" {
		a.ExpiresTodaySeverity = defaultExpiresTodaySeverity
	}
	if a.DurationForAlertEvaluation == "" {
		a.DurationForAlertEvaluation = defaultDurationForAlertEvaluation
	}
	if a.PrometheusRuleName == "" {
		a.PrometheusRuleName = defaultPrometheusRuleName
	}
}

func (a *alertingConfig) Validate() error {
	if a.CertificateExpirationDays > a.CertificateRenewalDays {
		return fmt.Errorf(
			"certificateRenewalDays must be greater than or equal to certificateExpirationDays, got %d, %d",
			a.CertificateRenewalDays, a.CertificateExpirationDays,
		)
	}
	return nil
}

func (x *x509certificateExporterConfig) IsInclusterEnabled() bool {
	return x.inCluster.Enabled
}

func (x *x509certificateExporterConfig) IsWorkerGroupsEnabled() bool {
	return len(x.workerGroups) > 0
}

func (x *x509certificateExporterConfig) Validate() (errs []error) {
	if err := x.inCluster.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("inCluster: %w", err))
	}
	if err := x.alerting.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("alerting: %w", err))
	}
	if err := x.workerGroups.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("workerGroups: %w", err))
	}
	if x.IsInclusterEnabled() && x.IsWorkerGroupsEnabled() {
		errs = append(errs, fmt.Errorf("at least one of inCluster or workerGroups must be enabled"))
	}
	return
}

func (x *x509certificateExporterConfig) Default() {
	x.inCluster.Default()
	x.alerting.Default()
}

func parseConfig(data []byte, out *x509certificateExporterConfig) error {
	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("failed to unmarshal x509certificateexporter config: %w", err)
	}

	out.Default()
	if err := out.Validate(); err != nil {
		return fmt.Errorf("x509certificateexporter config validation failed: %+v", err)
	}
	return nil
}
