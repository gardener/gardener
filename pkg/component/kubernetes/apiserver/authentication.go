// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiserverv1beta1 "k8s.io/apiserver/pkg/apis/apiserver/v1beta1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

const (
	volumeNameStructuredAuthenticationConfig = "authentication-config"

	volumeMountPathStructuredAuthenticationConfig = "/etc/kubernetes/structured/authentication"
	volumeMountPathOIDCCABundle                   = "/srv/kubernetes/oidc"

	// DataKeyConfigMapAuthenticationConfig is the key of the ConfigMap containing the authentication configuration.
	DataKeyConfigMapAuthenticationConfig = "config.yaml"
)

// reconcileConfigMapAuthenticationConfig reconciles the ConfigMap containing the authentication configuration.
func (k *kubeAPIServer) reconcileConfigMapAuthenticationConfig(ctx context.Context, configMap *corev1.ConfigMap) error {
	if versionutils.ConstraintK8sLess130.Check(k.values.Version) && k.values.AuthenticationConfiguration != nil {
		return errors.New("structured authentication is not available for versions < v1.30")
	}

	if k.values.AuthenticationConfiguration != nil && k.values.OIDC != nil {
		return errors.New("oidc configuration is incompatible with structured authentication")
	}

	if value, ok := k.values.FeatureGates["StructuredAuthenticationConfiguration"]; (ok && !value) ||
		(k.values.AuthenticationConfiguration == nil && k.values.OIDC == nil) ||
		versionutils.ConstraintK8sLess130.Check(k.values.Version) {
		return nil
	}

	authenticationConfig := ptr.Deref(k.values.AuthenticationConfiguration, "")
	if k.values.OIDC != nil {
		oidcAuthenticationConfig, err := ComputeAuthenticationConfigRawConfig(k.values.OIDC)
		if err != nil {
			return err
		}

		authenticationConfig = oidcAuthenticationConfig
	}

	configMap.Data = map[string]string{DataKeyConfigMapAuthenticationConfig: authenticationConfig}
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return client.IgnoreAlreadyExists(k.client.Client().Create(ctx, configMap))
}

// ComputeAuthenticationConfigRawConfig computes a AuthenticationConfiguration from oidcConfiguration.
// TODO(AleksandarSavchev): Remove this functionality as soon as v1.32 is the least supported Kubernetes version in Gardener.
func ComputeAuthenticationConfigRawConfig(oidc *gardencorev1beta1.OIDCConfig) (string, error) {
	authenticationConfiguration := &apiserverv1beta1.AuthenticationConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiserverv1beta1.ConfigSchemeGroupVersion.String(),
			Kind:       "AuthenticationConfiguration",
		},
		JWT: []apiserverv1beta1.JWTAuthenticator{{}},
	}

	if v := oidc.CABundle; v != nil {
		authenticationConfiguration.JWT[0].Issuer.CertificateAuthority = *v
	}

	if v := oidc.IssuerURL; v != nil {
		authenticationConfiguration.JWT[0].Issuer.URL = *v
	}

	if v := oidc.ClientID; v != nil {
		authenticationConfiguration.JWT[0].Issuer.Audiences = append(authenticationConfiguration.JWT[0].Issuer.Audiences, *v)
	}

	authenticationConfiguration.JWT[0].ClaimMappings.Username.Claim = ptr.Deref(oidc.UsernameClaim, "sub")

	usernamePrefix := ptr.Deref(oidc.UsernamePrefix, "")

	// For the authentication config, there is no defaulting for claim or prefix.
	// We use the defaulting suggested in kubernetes https://github.com/kubernetes/kubernetes/blob/a2106b5f73fe9352f7bc0520788855d57fc301e1/staging/src/k8s.io/apiserver/pkg/apis/apiserver/v1alpha1/types.go#L357-L366
	switch {
	case usernamePrefix == "-":
		authenticationConfiguration.JWT[0].ClaimMappings.Username.Prefix = ptr.To("")
	case len(usernamePrefix) == 0 && authenticationConfiguration.JWT[0].ClaimMappings.Username.Claim != "email":
		authenticationConfiguration.JWT[0].ClaimMappings.Username.Prefix = ptr.To(fmt.Sprintf("%s#", authenticationConfiguration.JWT[0].Issuer.URL))
	default:
		authenticationConfiguration.JWT[0].ClaimMappings.Username.Prefix = ptr.To(usernamePrefix)
	}

	if v := oidc.GroupsClaim; v != nil {
		authenticationConfiguration.JWT[0].ClaimMappings.Groups.Claim = *v
		authenticationConfiguration.JWT[0].ClaimMappings.Groups.Prefix = ptr.To(ptr.Deref(oidc.GroupsPrefix, ""))
	}

	for key, value := range oidc.RequiredClaims {
		claimValidationRule := apiserverv1beta1.ClaimValidationRule{
			Claim:         key,
			RequiredValue: value,
		}
		authenticationConfiguration.JWT[0].ClaimValidationRules = append(authenticationConfiguration.JWT[0].ClaimValidationRules, claimValidationRule)
	}

	data, err := runtime.Encode(ConfigCodec, authenticationConfiguration)
	if err != nil {
		return "", fmt.Errorf("unable to encode authentication configuration: %w", err)
	}

	return string(data), nil
}

func (k *kubeAPIServer) handleAuthenticationSettings(deployment *appsv1.Deployment, configMapAuthenticationConfig *corev1.ConfigMap, secretOIDCCABundle *corev1.Secret) {
	if value, ok := k.values.FeatureGates["StructuredAuthenticationConfiguration"]; versionutils.ConstraintK8sLess130.Check(k.values.Version) || (ok && !value) {
		k.handleOIDCSettings(deployment, secretOIDCCABundle)
		return
	}

	if _, ok := configMapAuthenticationConfig.Data[DataKeyConfigMapAuthenticationConfig]; !ok {
		return
	}

	deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--authentication-config=%s/%s", volumeMountPathStructuredAuthenticationConfig, DataKeyConfigMapAuthenticationConfig))
	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      volumeNameStructuredAuthenticationConfig,
		MountPath: volumeMountPathStructuredAuthenticationConfig,
	})
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: volumeNameStructuredAuthenticationConfig,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapAuthenticationConfig.Name,
				},
			},
		},
	})
}

// TODO(AleksandarSavchev): Remove this functionality as soon as v1.32 is the least supported Kubernetes version in Gardener.
func (k *kubeAPIServer) handleOIDCSettings(deployment *appsv1.Deployment, secretOIDCCABundle *corev1.Secret) {
	if k.values.OIDC == nil {
		return
	}

	if k.values.OIDC.CABundle != nil {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--oidc-ca-file=%s/%s", volumeMountPathOIDCCABundle, secretOIDCCABundleDataKeyCaCrt))
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, []corev1.VolumeMount{
			{
				Name:      volumeNameOIDCCABundle,
				MountPath: volumeMountPathOIDCCABundle,
			},
		}...)
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, []corev1.Volume{
			{
				Name: volumeNameOIDCCABundle,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secretOIDCCABundle.Name,
					},
				},
			},
		}...)
	}

	if v := k.values.OIDC.IssuerURL; v != nil {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--oidc-issuer-url="+*v)
	}

	if v := k.values.OIDC.ClientID; v != nil {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--oidc-client-id="+*v)
	}

	if v := k.values.OIDC.UsernameClaim; v != nil {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--oidc-username-claim="+*v)
	}

	if v := k.values.OIDC.GroupsClaim; v != nil {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--oidc-groups-claim="+*v)
	}

	if v := k.values.OIDC.UsernamePrefix; v != nil {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--oidc-username-prefix="+*v)
	}

	if v := k.values.OIDC.GroupsPrefix; v != nil {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--oidc-groups-prefix="+*v)
	}

	if k.values.OIDC.SigningAlgs != nil {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--oidc-signing-algs="+strings.Join(k.values.OIDC.SigningAlgs, ","))
	}

	for key, value := range k.values.OIDC.RequiredClaims {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--oidc-required-claim="+fmt.Sprintf("%s=%s", key, value))
	}
}
