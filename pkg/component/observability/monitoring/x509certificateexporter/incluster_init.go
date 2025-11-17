// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

func secretTypesAsArgs(watchedSecrets []watchableSecret) []string {
	return stringsToArgs("secret-type", func() []string {
		result := make([]string, len(watchedSecrets))
		for i, stype := range watchedSecrets {
			result[i] = stype.String()
		}
		return result
	}())
}

func configMapKeysAsArgs(configMapKeys []string) []string {
	return stringsToArgs("configmap-keys", configMapKeys)
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
	return fmt.Sprintf("--max-cache-duration=%s", duration)
}

func kubeAPIBurstAsArgs(burst *uint32) string {
	return fmt.Sprintf("--kube-api-rate-limit-burst=%d", *burst)
}

func kubeAPIRateLimitAsArgs(rate *uint32) string {
	return fmt.Sprintf("--kube-api-rate-limit-qps=%d", *rate)
}

var configMapKeyRegexp = regexp.MustCompile(`^[-._a-zA-Z0-9]+$`)

func validateConfigMapKey(key string) error {
	if len(key) == 0 {
		return ErrEmptyConfigMapKey
	}

	if key == "." || key == ".." || !configMapKeyRegexp.MatchString(key) {
		return fmt.Errorf("%w: %q", ErrKeyIsIllegal, key)
	}
	if len(key) > 253 {
		return fmt.Errorf("%w: %q", ErrConfigMapMaxKeyLenght, key)
	}
	return nil
}

func validateLabels(labelz map[string]string) error {
	for k, v := range labelz {
		if err := validation.IsQualifiedName(k); err != nil {
			return fmt.Errorf("%w: %q - %s", ErrIncludeLabelsInvalid, k, err)
		}
		if err := validation.IsValidLabelValue(v); err != nil {
			return fmt.Errorf("%w: %q:%q - %s", ErrIncludeLabelsInvalid, k, v, err)
		}
	}
	return nil
}

func validateNamespaces(namespaces []string) error {
	for _, ns := range namespaces {
		if err := validation.IsDNS1123Label(ns); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidNamespace, err)
		}
	}
	return nil
}

func validateSecretType(secretType corev1.SecretType) error {
	switch secretType {
	case corev1.SecretTypeTLS,
		corev1.SecretTypeOpaque,
		corev1.SecretTypeBasicAuth,
		corev1.SecretTypeSSHAuth,
		istioSecretType:
		return nil
	}
	return ErrInvalidSecretType
}

func (i *inClusterConfig) Default() {
	i.DefaultCommon()

	i.KubeAPIBurst = ptr.To(defaultKubeAPIBurst)
	i.KubeAPIRateLimit = ptr.To(defaultKubeAPIRateLimit)
	i.MaxCacheDuration = defaultCertCacheDuration
	i.Replicas = ptr.To(defaultReplicas)
}

func (i *inClusterConfig) Validate() error {
	if !i.Enabled {
		return nil
	}
	if len(i.SecretsToWatch) == 0 && len(i.ConfigMapKeys) == 0 {
		return ErrNoConfigMapKeyOrSecretTypes
	}
	errs := make([]error, 0)
	for idx, stype := range i.SecretsToWatch {
		if err := validateSecretType(corev1.SecretType(stype.Type)); err != nil {
			errs = append(errs, fmt.Errorf("secretTypes[%d] is invalid: %w", idx, err))
		}
	}
	for idx, cmkey := range i.ConfigMapKeys {
		if err := validateConfigMapKey(cmkey); err != nil {
			errs = append(errs, fmt.Errorf("configMapKeys[%d] is invalid: %w", idx, err))
		}
	}
	if err := validateLabels(i.IncludeLabels); err != nil {
		errs = append(errs, fmt.Errorf("includeLabels is invalid: %w", err))
	}
	if err := validateLabels(i.ExcludeLabels); err != nil {
		errs = append(errs, fmt.Errorf("excludeLabels is invalid: %w", err))
	}
	if err := validateNamespaces(i.IncludeNamespaces); err != nil {
		errs = append(errs, fmt.Errorf("includeNamespaces is invalid: %w", err))
	}
	if err := validateNamespaces(i.ExcludeNamespaces); err != nil {
		errs = append(errs, fmt.Errorf("excludeNamespaces is invalid: %w", err))
	}
	return errors.Join(errs...)
}

func (i *inClusterConfig) GetArgs() []string {
	args := make(
		[]string, 0,
		len(i.SecretsToWatch)+len(i.ConfigMapKeys)+len(i.IncludeLabels)+
			len(i.ExcludeLabels)+len(i.IncludeNamespaces)+len(i.ExcludeNamespaces)+
			// MaxCacheDuration, KubeAPIBurst, KubeAPIRateLimit, watch-kube-secrets
			4,
	)
	args = append(args, secretTypesAsArgs(i.SecretsToWatch)...)
	args = append(args, configMapKeysAsArgs(i.ConfigMapKeys)...)
	args = append(args, includedLabelsAsArgs(i.IncludeLabels)...)
	args = append(args, excludedLabelsAsArgs(i.ExcludeLabels)...)
	args = append(args, includedNamespacesAsArgs(i.IncludeNamespaces)...)
	args = append(args, excludedNamespacesAsArgs(i.ExcludeNamespaces)...)
	args = append(args, maxCacheDurationAsArgs(i.MaxCacheDuration), kubeAPIBurstAsArgs(i.KubeAPIBurst), kubeAPIRateLimitAsArgs(i.KubeAPIRateLimit))
	args = append(args, i.GetCommonArgs()...)
	args = append(args, "--watch-kube-secrets")
	sort.Strings(args)

	return args
}
