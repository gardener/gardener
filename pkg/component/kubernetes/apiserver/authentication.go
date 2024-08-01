// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	genericapiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

const (
	volumeNameStructuredAuthenticationConfig = "authentication-config"

	volumeMountStructuredAuthenticationConfig = "/etc/kubernetes/structured/authentication"
	volumeMountPathOIDCCABundle               = "/srv/kubernetes/oidc"

	configMapAuthenticationConfigDataKey = "config.yaml"
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

	var authenticationConfig string
	if k.values.AuthenticationConfiguration != nil {
		authenticationConfig = *k.values.AuthenticationConfiguration
	}
	if k.values.OIDC != nil {
		oidcAuthenticationConfig, err := computeAuthenticationConfigRawConfig(k.values.OIDC)
		if err != nil {
			return err
		}

		authenticationConfig = oidcAuthenticationConfig
	}

	configMap.Data = map[string]string{configMapAuthenticationConfigDataKey: authenticationConfig}
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return client.IgnoreAlreadyExists(k.client.Client().Create(ctx, configMap))
}

func computeAuthenticationConfigRawConfig(OIDC *gardencorev1beta1.OIDCConfig) (string, error) {
	authenticationConfiguration := &genericapiserverv1alpha1.AuthenticationConfiguration{
		TypeMeta: metav1.TypeMeta{
			// TODO(AleksandarSavchev): use v1beta1 when kubernetes packages are updated to version >= v1.30
			APIVersion: "apiserver.config.k8s.io/v1alpha1",
			Kind:       "AuthenticationConfiguration",
		},
		JWT: []genericapiserverv1alpha1.JWTAuthenticator{
			{},
		},
	}
	if v := OIDC.CABundle; v != nil {
		authenticationConfiguration.JWT[0].Issuer.CertificateAuthority = *v
	}

	if v := OIDC.IssuerURL; v != nil {
		authenticationConfiguration.JWT[0].Issuer.URL = *v
	}

	if v := OIDC.ClientID; v != nil {
		authenticationConfiguration.JWT[0].Issuer.Audiences = append(authenticationConfiguration.JWT[0].Issuer.Audiences, *v)
	}

	if v := OIDC.UsernameClaim; v != nil {
		authenticationConfiguration.JWT[0].ClaimMappings.Username.Claim = *v
	} else {
		authenticationConfiguration.JWT[0].ClaimMappings.Username.Claim = "sub"
	}

	usernamePrefix := ""
	if v := OIDC.UsernamePrefix; v != nil {
		usernamePrefix = *v
	}

	switch {
	case usernamePrefix == "-":
		authenticationConfiguration.JWT[0].ClaimMappings.Username.Prefix = ptr.To("")
	case len(usernamePrefix) == 0 && authenticationConfiguration.JWT[0].ClaimMappings.Username.Claim != "email":
		authenticationConfiguration.JWT[0].ClaimMappings.Username.Prefix = ptr.To(fmt.Sprintf("%s#", authenticationConfiguration.JWT[0].Issuer.URL))
	default:
		authenticationConfiguration.JWT[0].ClaimMappings.Username.Prefix = ptr.To(usernamePrefix)
	}

	if v := OIDC.GroupsClaim; v != nil {
		authenticationConfiguration.JWT[0].ClaimMappings.Groups.Claim = *v
		if v := OIDC.GroupsPrefix; v != nil {
			authenticationConfiguration.JWT[0].ClaimMappings.Groups.Prefix = v
		} else {
			authenticationConfiguration.JWT[0].ClaimMappings.Groups.Prefix = ptr.To("")
		}
	}

	for key, value := range OIDC.RequiredClaims {
		claimValidationRule := genericapiserverv1alpha1.ClaimValidationRule{
			Claim:         key,
			RequiredValue: value,
		}
		authenticationConfiguration.JWT[0].ClaimValidationRules = append(authenticationConfiguration.JWT[0].ClaimValidationRules, claimValidationRule)
	}

	rawConfig, err := yaml.Marshal(authenticationConfiguration)
	if err != nil {
		return "", err
	}

	return string(rawConfig), nil
}

func (k *kubeAPIServer) handleAuthenticationSettings(deployment *appsv1.Deployment, configMapAuthenticationConfig *corev1.ConfigMap, secretOIDCCABundle *corev1.Secret) {
	if value, ok := k.values.FeatureGates["StructuredAuthenticationConfiguration"]; versionutils.ConstraintK8sLess130.Check(k.values.Version) || (ok && !value) {
		k.handleOIDCSettings(deployment, secretOIDCCABundle)
		return
	}

	if _, ok := configMapAuthenticationConfig.Data[configMapAuthenticationConfigDataKey]; !ok {
		return
	}

	deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--authentication-config=%s/%s", volumeMountStructuredAuthenticationConfig, configMapAuthenticationConfigDataKey))
	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      volumeNameStructuredAuthenticationConfig,
		MountPath: volumeMountStructuredAuthenticationConfig,
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
