package x509certificateexporter

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"
)

func secretTypesAsArgs(secretTypes []string) []string {
	return stringsToArgs("secret-type", secretTypes)
}

func configMapKeysAsArgs(configMapKeys []string) []string {
	return stringsToArgs("configmap-key", configMapKeys)
}

func includedLabelsAsArgs(includedLabels map[string]string) []string {
	return mappedStringsToArgs("include-label", includedLabels)
}

func excludedLabelsAsArgs(excludedLabels map[string]string) []string {
	return mappedStringsToArgs("exclude-label", excludedLabels)
}

func includedNamespacesAsArgs(includedNamespaces []string) []string {
	return stringsToArgs("include-namespace", includedNamespaces)
}

func excludedNamespacesAsArgs(excludedNamespaces []string) []string {
	return stringsToArgs("exclude-namespace", excludedNamespaces)
}

func maxCacheDurationAsArgs(duration time.Duration) string {
	return fmt.Sprintf("--max-cache-duration=%d", duration)
}

func kubeAPIBurstAsArgs(burst *uint32) string {
	return fmt.Sprintf("--kube-api-rate-limit-burst=%d", burst)
}

func kubeAPIRateLimitAsArgs(rate *uint32) string {
	return fmt.Sprintf("--kube-api-rate-limit-qps=%d", rate)
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
			return fmt.Errorf("includeLabels has invalid key %q: %v", k, err)
		}
		if err := validation.IsValidLabelValue(v); err != nil {
			return fmt.Errorf("includeLabels[%q] has invalid value %q: %v", k, v, err)
		}
	}
	return nil
}

func validateNamespaces(namespaces []string) error {
	for idx, ns := range namespaces {
		if err := validation.IsDNS1123Label(ns); err != nil {
			return fmt.Errorf("namespaces[%d] is invalid: %v", idx, err)
		}
	}
	return nil
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

func (i *inClusterConfig) Default() {
	if !i.Enabled {
		return
	}
	if *i.Replicas == 0 {
		i.Replicas = ptr.To(defaultReplicas)
	}
	if *i.KubeAPIBurst == 0 {
		i.KubeAPIBurst = ptr.To(defaultKubeAPIBurst)
	}
	if *i.KubeAPIRateLimit == 0 {
		i.KubeAPIRateLimit = ptr.To(defaultKubeAPIRateLimit)
	}
	if i.MaxCacheDuration == 0 {
		i.MaxCacheDuration = defaultCertCacheDuration
	}
}

func (i *inClusterConfig) Validate() error {
	if !i.Enabled {
		return nil
	}
	if len(i.SecretTypes) == 0 && len(i.ConfigMapKeys) == 0 {
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

func (i *inClusterConfig) GetArgs() []string {
	args := make(
		[]string, 0,
		len(i.SecretTypes)+len(i.ConfigMapKeys)+len(i.IncludeLabels)+
			len(i.ExcludeLabels)+len(i.IncludeNamespaces)+len(i.ExcludeNamespaces)+
			// MaxCacheDuration, KubeAPIBurst, KubeAPIRateLimit
			3,
	)
	args = append(args, secretTypesAsArgs(i.SecretTypes)...)
	args = append(args, configMapKeysAsArgs(i.ConfigMapKeys)...)
	args = append(args, includedLabelsAsArgs(i.IncludeLabels)...)
	args = append(args, excludedLabelsAsArgs(i.ExcludeLabels)...)
	args = append(args, includedNamespacesAsArgs(i.IncludeNamespaces)...)
	args = append(args, excludedNamespacesAsArgs(i.ExcludeNamespaces)...)
	args = append(args, maxCacheDurationAsArgs(i.MaxCacheDuration), kubeAPIBurstAsArgs(i.KubeAPIBurst), kubeAPIRateLimitAsArgs(i.KubeAPIRateLimit))
	args = append(args, i.GetCommonArgs()...)
	sort.Strings(args)

	return args
}
