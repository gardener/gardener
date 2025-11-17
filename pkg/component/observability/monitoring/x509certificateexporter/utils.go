// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package x509certificateexporter

import (
	"errors"
	"fmt"

	"go.yaml.in/yaml/v4"

	"k8s.io/utils/ptr"
)

func mapStrings(slice []string, fn func(string) string) []string {
	result := make([]string, len(slice))
	for i, v := range slice {
		result[i] = fn(v)
	}
	return result
}

func mapStringsWithVals(stringMap map[string]string, fn func(string, string) string) []string {
	results := make([]string, 0)
	for k, v := range stringMap {
		results = append(results, fn(k, v))
	}
	return results
}

func stringsToArgs(argName string, values []string) []string {
	return mapStrings(values, func(value string) string {
		return fmt.Sprintf("--%s=%s", argName, value)
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
	if enabled {
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

func (s watchableSecret) String() string {
	return fmt.Sprintf("%s:%s", s.Type, s.RegEx)
}

func (c *commonExporterConfigs) GetCommonArgs() []string {
	args := []string{fmt.Sprintf("--listen-address=:%d", Port)}
	args = append(args, getExposeRelativeMetricsArg(c.ExposeRelativeMetrics)...)
	args = append(args, getTrimComponentsArg(c.TrimComponents)...)
	args = append(args, getExposePerCertErrorMetricsArg(c.ExposePerCertErrorMetrics)...)
	args = append(args, getExposeLabelsMetricsArg(c.ExposeLabelsMetrics)...)
	return args
}

func (c *commonExporterConfigs) DefaultCommon() {
	c.TrimComponents = ptr.To(uint32(0))
	c.ExposeRelativeMetrics = false
	c.ExposePerCertErrorMetrics = false
	c.ExposeLabelsMetrics = false
}

func (a *alertingConfig) Default() {
	a.CertificateExpirationDays = defaultCertificateExpirationDays
	a.CertificateRenewalDays = defaultCertificateRenewalDays

	a.ReadErrorsSeverity = defaultReadErrorsSeverity
	a.CertificateErrorsSeverity = defaultCertificateErrorsSeverity
	a.RenewalSeverity = defaultRenewalSeverity
	a.ExpirationSeverity = defaultExpirationSeverity
	a.ExpiresTodaySeverity = defaultExpiresTodaySeverity
	a.DurationForAlertEvaluation = defaultDurationForAlertEvaluation
	a.PrometheusRuleName = defaultPrometheusRuleName
}

func (a *alertingConfig) Validate() error {
	errs := []error{}
	for _, sev := range []prometheusRuleSeverity{
		a.ReadErrorsSeverity,
		a.CertificateErrorsSeverity,
		a.RenewalSeverity,
		a.ExpirationSeverity,
		a.ExpiresTodaySeverity,
	} {
		if err := sev.Validate(); err != nil {
			errs = append(errs, err)
		}
	}

	if a.CertificateExpirationDays > a.CertificateRenewalDays {
		errs = append(errs, fmt.Errorf(
			"%w, got %d, %d", ErrInvalidExpirationRenewalConf,
			a.CertificateRenewalDays, a.CertificateExpirationDays,
		))
	}
	return errors.Join(errs...)
}

func (xc *x509certificateExporterConfig) IsInclusterEnabled() bool {
	return xc.InCluster.Enabled
}

func (xc *x509certificateExporterConfig) IsWorkerGroupsEnabled() bool {
	return len(xc.WorkerGroups) > 0
}

func (xc *x509certificateExporterConfig) Validate() error {
	var errs []error
	if !xc.IsInclusterEnabled() && !xc.IsWorkerGroupsEnabled() {
		errs = append(errs, fmt.Errorf("%w: %+v", ErrEmptyExporterConfig, xc))
	}

	if err := xc.InCluster.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("%w: %v", ErrInClusterConfig, err))
	}
	if err := xc.Alerting.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("%w, %v", ErrAlertingConfig, err))
	}
	if err := xc.WorkerGroups.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("%w: %w", ErrWorkerGroupsConfig, err))
	}

	return errors.Join(errs...)
}

func (xc *x509certificateExporterConfig) Default() {
	xc.InCluster.Default()
	xc.WorkerGroups.Default()
	xc.Alerting.Default()
}

func parseConfig(data []byte, out *x509certificateExporterConfig) error {
	out.Default()
	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidExporterConfigFormat, err)
	}

	if err := out.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrConfigValidationFailed, err)
	}
	return nil
}
