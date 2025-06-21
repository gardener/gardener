// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
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

	if !k.structuredAuthenticationFeatureGateEnabled() ||
		(k.values.AuthenticationConfiguration == nil && k.values.OIDC == nil && !k.anonymousAuthConfigurableEndpointsFeatureGateEnabled()) {
		return nil
	}

	authenticationConfig := ptr.Deref(k.values.AuthenticationConfiguration, "")

	authenticationConfiguration := &apiserverv1beta1.AuthenticationConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiserverv1beta1.ConfigSchemeGroupVersion.String(),
			Kind:       "AuthenticationConfiguration",
		},
	}

	if len(authenticationConfig) > 0 {
		if err := runtime.DecodeInto(ConfigCodec, []byte(authenticationConfig), authenticationConfiguration); err != nil {
			return v1beta1helper.NewErrorWithCodes(err, gardencorev1beta1.ErrorConfigurationProblem)
		}
	}

	if k.values.OIDC != nil {
		authenticationConfiguration.JWT = ConfigureJWTAuthenticators(k.values.OIDC)
	}

	if k.anonymousAuthConfigurableEndpointsFeatureGateEnabled() && authenticationConfiguration.Anonymous == nil {
		authenticationConfiguration.Anonymous = &apiserverv1beta1.AnonymousAuthConfig{
			Enabled: ptr.Deref(k.values.AnonymousAuthenticationEnabled, false),
		}
	}

	data, err := runtime.Encode(ConfigCodec, authenticationConfiguration)
	if err != nil {
		return fmt.Errorf("unable to encode authentication configuration: %w", err)
	}

	configMap.Data = map[string]string{DataKeyConfigMapAuthenticationConfig: string(data)}
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return client.IgnoreAlreadyExists(k.client.Client().Create(ctx, configMap))
}

// ConfigureJWTAuthenticators computes JWTAuthenticator configuration from oidcConfiguration.
// TODO(AleksandarSavchev): Remove this functionality as soon as v1.32 is the least supported Kubernetes version in Gardener.
func ConfigureJWTAuthenticators(oidc *gardencorev1beta1.OIDCConfig) []apiserverv1beta1.JWTAuthenticator {
	jwts := []apiserverv1beta1.JWTAuthenticator{{}}

	if v := oidc.CABundle; v != nil {
		jwts[0].Issuer.CertificateAuthority = *v
	}

	if v := oidc.IssuerURL; v != nil {
		jwts[0].Issuer.URL = *v
	}

	if v := oidc.ClientID; v != nil {
		jwts[0].Issuer.Audiences = append(jwts[0].Issuer.Audiences, *v)
	}

	jwts[0].ClaimMappings.Username.Claim = ptr.Deref(oidc.UsernameClaim, "sub")

	usernamePrefix := ptr.Deref(oidc.UsernamePrefix, "")

	// For the authentication config, there is no defaulting for claim or prefix.
	// We use the defaulting suggested in kubernetes https://github.com/kubernetes/kubernetes/blob/a2106b5f73fe9352f7bc0520788855d57fc301e1/staging/src/k8s.io/apiserver/pkg/apis/apiserver/v1alpha1/types.go#L357-L366
	switch {
	case usernamePrefix == "-":
		jwts[0].ClaimMappings.Username.Prefix = ptr.To("")
	case len(usernamePrefix) == 0 && jwts[0].ClaimMappings.Username.Claim != "email":
		jwts[0].ClaimMappings.Username.Prefix = ptr.To(fmt.Sprintf("%s#", jwts[0].Issuer.URL))
	default:
		jwts[0].ClaimMappings.Username.Prefix = ptr.To(usernamePrefix)
	}

	if v := oidc.GroupsClaim; v != nil {
		jwts[0].ClaimMappings.Groups.Claim = *v
		jwts[0].ClaimMappings.Groups.Prefix = ptr.To(ptr.Deref(oidc.GroupsPrefix, ""))
	}

	for key, value := range oidc.RequiredClaims {
		claimValidationRule := apiserverv1beta1.ClaimValidationRule{
			Claim:         key,
			RequiredValue: value,
		}
		jwts[0].ClaimValidationRules = append(jwts[0].ClaimValidationRules, claimValidationRule)
	}

	return jwts
}

func (k *kubeAPIServer) handleAuthenticationSettings(deployment *appsv1.Deployment, configMapAuthenticationConfig *corev1.ConfigMap, secretOIDCCABundle *corev1.Secret) error {
	var structAuthConfiguresAnonymousAuthentication bool

	if !k.structuredAuthenticationFeatureGateEnabled() {
		k.handleOIDCSettings(deployment, secretOIDCCABundle)
	}

	if config, ok := configMapAuthenticationConfig.Data[DataKeyConfigMapAuthenticationConfig]; ok {
		authenticationConfiguration := &apiserverv1beta1.AuthenticationConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: apiserverv1beta1.ConfigSchemeGroupVersion.String(),
				Kind:       "AuthenticationConfiguration",
			},
		}

		if err := runtime.DecodeInto(ConfigCodec, []byte(config), authenticationConfiguration); err != nil {
			return err
		}

		structAuthConfiguresAnonymousAuthentication = authenticationConfiguration.Anonymous != nil
	}

	if !k.structuredAuthenticationFeatureGateEnabled() || !structAuthConfiguresAnonymousAuthentication {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--anonymous-auth="+strconv.FormatBool(ptr.Deref(k.values.AnonymousAuthenticationEnabled, false)))
	}

	if _, ok := configMapAuthenticationConfig.Data[DataKeyConfigMapAuthenticationConfig]; !ok {
		return nil
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

	return nil
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

func (k *kubeAPIServer) structuredAuthenticationFeatureGateEnabled() bool {
	if versionutils.ConstraintK8sLess130.Check(k.values.Version) {
		return false
	}

	featureGateEnabled, featureGateSet := k.values.FeatureGates["StructuredAuthenticationConfiguration"]
	if featureGateSet {
		return featureGateEnabled
	}

	return true
}

func (k *kubeAPIServer) anonymousAuthConfigurableEndpointsFeatureGateEnabled() bool {
	if versionutils.ConstraintK8sLess131.Check(k.values.Version) {
		return false
	}

	featureGateEnabled, featureGateSet := k.values.FeatureGates["AnonymousAuthConfigurableEndpoints"]
	if featureGateSet {
		return featureGateEnabled
	}

	if versionutils.ConstraintK8sEqual131.Check(k.values.Version) {
		// the feature is not enabled by default in v1.31
		return false
	}

	return true
}
