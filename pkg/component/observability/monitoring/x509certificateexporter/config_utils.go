package x509certificateexporter

import (
	"errors"
	"fmt"
	"regexp"

	"go.yaml.in/yaml/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"
)

type noNodeSelectorforWorkerError string

func (n noNodeSelectorforWorkerError) Error() string {
	return fmt.Sprintf("worker group %q must have node selector", n)
}

func validateSecretType(secretType corev1.SecretType) error {
	switch secretType {
	case corev1.SecretTypeTLS,
		corev1.SecretTypeOpaque,
		corev1.SecretTypeBasicAuth,
		corev1.SecretTypeSSHAuth:
		return errors.New("invalid secret type for x509certificateexporter")
	}
	return nil
}

var configMapKeyRegexp = regexp.MustCompile(`^[-._a-zA-Z0-9]+$`)

func validateConfigMapKey(key string) error {
	if len(key) == 0 {
		return errors.New("config map key cannot be empty")
	}
	if key == "." {
		return errors.New("config map key cannot be a single dot")
	}
	if key == ".." {
		return errors.New("config map key cannot be a double dot")
	}
	if !configMapKeyRegexp.MatchString(key) {
		return fmt.Errorf("invalid config map key %q", key)
	}
	if len(key) > 253 {
		return fmt.Errorf("config map key %q exceeds maximum length of 253 characters", key)
	}
	return nil
}

func validateLabels(labelz map[string]string) error {
	for k, v := range labelz {
		if err := validation.IsQualifiedName(k); err != nil {
			return fmt.Errorf("includeLabels has invalid key %q: %w", k, err)
		}
		if err := validation.IsValidLabelValue(v); err != nil {
			return fmt.Errorf("includeLabels[%q] has invalid value %q: %w", k, v, err)
		}
	}
	return nil
}

func validateNamespaces(namespaces []string) error {
	for idx, ns := range namespaces {
		if err := validation.IsDNS1123Label(ns); err != nil {
			return fmt.Errorf("namespaces[%d] is invalid: %w", idx, err)
		}
	}
	return nil
}

func (i *inClusterConfig) Default() {
	if i.Enabled == false {
		return
	}
	if *i.Replicas == 0 {
		i.Replicas = ptr.To(defaultReplicas)
	}
	if *i.KubeApiBurst == 0 {
		i.KubeApiBurst = ptr.To(defaultKubeApiBurst)
	}
	if *i.KubeApiRateLimit == 0 {
		i.KubeApiRateLimit = ptr.To(defaultKubeApiRateLimit)
	}
	if i.MaxCacheDuration == 0 {
		i.MaxCacheDuration = defaultCertCacheDuration
	}
}

func (i *inClusterConfig) Validate() error {
	if !i.Enabled {
		return nil
	}
	if (i.SecretTypes == nil || len(i.SecretTypes) == 0) && (i.ConfigMapKeys == nil || len(i.ConfigMapKeys) == 0) {
		return fmt.Errorf("at least one of secretTypes or configMapKeys must be specified when inCluster monitoring is enabled")
	}
	for idx, stype := range i.SecretTypes {
		if err := validateSecretType(corev1.SecretType(stype)); err != nil {
			return fmt.Errorf("secretTypes[%d] is invalid: %w", idx, err)
		}
	}
	for idx, cmkey := range i.ConfigMapKeys {
		if err := validateConfigMapKey(cmkey); err != nil {
			return fmt.Errorf("configMapKeys[%d] is invalid: %w", idx, err)
		}
	}
	if err := validateLabels(i.IncludeLabels); err != nil {
		return fmt.Errorf("includeLabels is invalid: %w", err)
	}
	if err := validateLabels(i.ExcludeLabels); err != nil {
		return fmt.Errorf("excludeLabels is invalid: %w", err)
	}
	if err := validateNamespaces(i.IncludeNamespaces); err != nil {
		return fmt.Errorf("includeNamespaces is invalid: %w", err)
	}
	if err := validateNamespaces(i.ExcludeNamespaces); err != nil {
		return fmt.Errorf("excludeNamespaces is invalid: %w", err)
	}
	return nil
}

func (wg *workerGroup) Validate() error {
	if wg.Selector != nil {
		return noNodeSelectorforWorkerError(fmt.Sprintf("%q", wg))
	}
	return nil
}

func (wgs *workerGroupsConfig) Validate() error {
	var (
		wgErrs    = make([]error, len(*wgs))
		noNameErr noNodeSelectorforWorkerError
	)
	if len(*wgs) == 0 {
		return nil
	}
	for _, wg := range *wgs {
		err := wg.Validate()
		wgErrs = append(wgErrs, err)
		if errors.As(err, &noNameErr) && len(*wgs) > 1 {
			return fmt.Errorf("multiple worker groups defined, but at least one is missing a node selector: %w", err)
		}
	}

	if len(wgErrs) > 0 {
		return fmt.Errorf("workerGroups validation errors: %w", wgErrs)
	}

	return nil
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
	return nil
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
		return fmt.Errorf("x509certificateexporter config validation failed: %w", err)
	}
	return nil
}
