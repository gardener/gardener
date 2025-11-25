// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"encoding/json"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
)

// ValidateInternalSecretName can be used to check whether the given secret name is valid.
// Prefix indicates this name will be used as part of generation, in which case
// trailing dashes are allowed.
var ValidateInternalSecretName = apivalidation.NameIsDNSSubdomain

// ValidateInternalSecret tests if required fields in the InternalSecret are set.
func ValidateInternalSecret(secret *core.InternalSecret) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMeta(&secret.ObjectMeta, true, ValidateInternalSecretName, field.NewPath("metadata"))

	dataPath := field.NewPath("data")
	totalSize := 0
	for key, value := range secret.Data {
		for _, msg := range validation.IsConfigMapKey(key) {
			allErrs = append(allErrs, field.Invalid(dataPath.Key(key), key, msg))
		}
		totalSize += len(value)
	}
	if totalSize > corev1.MaxSecretSize {
		allErrs = append(allErrs, field.TooLong(dataPath, "", corev1.MaxSecretSize))
	}

	switch secret.Type {
	case corev1.SecretTypeServiceAccountToken:
		// Only require Annotations[kubernetes.io/service-account.name]
		// Additional fields (like Annotations[kubernetes.io/service-account.uid] and Data[token]) might be contributed later by a controller loop
		if value := secret.Annotations[corev1.ServiceAccountNameKey]; len(value) == 0 {
			allErrs = append(allErrs, field.Required(field.NewPath("metadata", "annotations").Key(corev1.ServiceAccountNameKey), ""))
		}
	case corev1.SecretTypeOpaque, "":
	// no-op
	case corev1.SecretTypeDockercfg:
		dockercfgBytes, exists := secret.Data[corev1.DockerConfigKey]
		if !exists {
			allErrs = append(allErrs, field.Required(dataPath.Key(corev1.DockerConfigKey), ""))
			break
		}

		// make sure that the content is well-formed json.
		if err := json.Unmarshal(dockercfgBytes, &map[string]any{}); err != nil {
			allErrs = append(allErrs, field.Invalid(dataPath.Key(corev1.DockerConfigKey), "<secret contents redacted>", err.Error()))
		}
	case corev1.SecretTypeDockerConfigJson:
		dockerConfigJSONBytes, exists := secret.Data[corev1.DockerConfigJsonKey]
		if !exists {
			allErrs = append(allErrs, field.Required(dataPath.Key(corev1.DockerConfigJsonKey), ""))
			break
		}

		// make sure that the content is well-formed json.
		if err := json.Unmarshal(dockerConfigJSONBytes, &map[string]any{}); err != nil {
			allErrs = append(allErrs, field.Invalid(dataPath.Key(corev1.DockerConfigJsonKey), "<secret contents redacted>", err.Error()))
		}
	case corev1.SecretTypeBasicAuth:
		_, usernameFieldExists := secret.Data[corev1.BasicAuthUsernameKey]
		_, passwordFieldExists := secret.Data[corev1.BasicAuthPasswordKey]

		// username or password might be empty, but the field must be present
		if !usernameFieldExists && !passwordFieldExists {
			allErrs = append(allErrs, field.Required(dataPath.Key(corev1.BasicAuthUsernameKey), ""))
			allErrs = append(allErrs, field.Required(dataPath.Key(corev1.BasicAuthPasswordKey), ""))

			break
		}
	case corev1.SecretTypeSSHAuth:
		if len(secret.Data[corev1.SSHAuthPrivateKey]) == 0 {
			allErrs = append(allErrs, field.Required(dataPath.Key(corev1.SSHAuthPrivateKey), ""))
			break
		}

	case corev1.SecretTypeTLS:
		if _, exists := secret.Data[corev1.TLSCertKey]; !exists {
			allErrs = append(allErrs, field.Required(dataPath.Key(corev1.TLSCertKey), ""))
		}
		if _, exists := secret.Data[corev1.TLSPrivateKeyKey]; !exists {
			allErrs = append(allErrs, field.Required(dataPath.Key(corev1.TLSPrivateKeyKey), ""))
		}
	// TODO: Verify that the key matches the cert.
	default:
		// no-op
	}

	return allErrs
}

// ValidateInternalSecretUpdate tests if required fields in the InternalSecret are set.
func ValidateInternalSecretUpdate(newSecret, oldSecret *core.InternalSecret) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMetaUpdate(&newSecret.ObjectMeta, &oldSecret.ObjectMeta, field.NewPath("metadata"))

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSecret.Type, oldSecret.Type, field.NewPath("type"))...)
	if oldSecret.Immutable != nil && *oldSecret.Immutable {
		if newSecret.Immutable == nil || !*newSecret.Immutable {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("immutable"), "field is immutable when `immutable` is set"))
		}
		if !reflect.DeepEqual(newSecret.Data, oldSecret.Data) {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("data"), "field is immutable when `immutable` is set"))
		}
		// We don't validate StringData, as it was already converted back to Data
		// before validation is happening.
	}

	allErrs = append(allErrs, ValidateInternalSecret(newSecret)...)
	return allErrs
}
