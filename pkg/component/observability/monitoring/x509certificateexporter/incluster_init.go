package x509certificateexporter

import (
	"errors"
	"fmt"
	"regexp"

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
	if *i.KubeApiBurst == 0 {
		i.KubeApiBurst = ptr.To(defaultKubeAPIBurst)
	}
	if *i.KubeApiRateLimit == 0 {
		i.KubeApiRateLimit = ptr.To(defaultKubeAPIRateLimit)
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
