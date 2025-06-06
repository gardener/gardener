// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package authenticationconfig

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/apis/apiserver"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	apiserverv1beta1 "k8s.io/apiserver/pkg/apis/apiserver/v1beta1"
	apiservervalidation "k8s.io/apiserver/pkg/apis/apiserver/validation"
	authenticationcel "k8s.io/apiserver/pkg/authentication/cel"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	"github.com/gardener/gardener/pkg/webhook/configvalidator"
)

const (
	// HandlerName is the name of this admission webhook handler.
	HandlerName = "authenticationconfig_validator"
	// WebhookPath is the HTTP handler path for this admission webhook handler.
	WebhookPath = "/webhooks/authentication-configuration"
)

// AddToManager adds the webhook to the given manager.
func AddToManager(mgr manager.Manager) error {
	webhook := &admission.Webhook{
		Handler: NewHandler(
			mgr.GetLogger().WithName("webhook").WithName(HandlerName),
			mgr.GetAPIReader(),
			mgr.GetClient(),
			admission.NewDecoder(mgr.GetScheme()),
		),
		RecoverPanic: ptr.To(true),
	}

	mgr.GetWebhookServer().Register(WebhookPath, webhook)
	return nil
}

// NewHandler returns a new handler for validating authentication configuration.
func NewHandler(log logr.Logger, apiReader, c client.Reader, decoder admission.Decoder) admission.Handler {
	return &configvalidator.Handler{
		Logger:    log,
		APIReader: apiReader,
		Client:    c,
		Decoder:   decoder,

		ConfigMapPurpose: "authentication configuration",
		ConfigMapDataKey: "config.yaml",
		GetConfigMapNameFromShoot: func(shoot *gardencore.Shoot) string {
			return gardencorehelper.GetShootAuthenticationConfigurationConfigMapName(shoot.Spec.Kubernetes.KubeAPIServer)
		},
		SkipValidationOnShootUpdate: func(shoot, oldShoot *gardencore.Shoot) bool {
			if !gardencorehelper.IsLegacyAnonymousAuthenticationSet(oldShoot.Spec.Kubernetes.KubeAPIServer) && gardencorehelper.IsLegacyAnonymousAuthenticationSet(shoot.Spec.Kubernetes.KubeAPIServer) {
				return false // Don't skip validation when the deprecated anonymous authentication is being set.
			}
			return sets.New(getIssuersFromShoot(shoot)...).Equal(sets.New(getIssuersFromShoot(oldShoot)...))
		},
		AdmitConfig: admitConfig,
	}
}

var decoder runtime.Decoder

func init() {
	scheme := runtime.NewScheme()
	schemeBuilder := runtime.NewSchemeBuilder(apiserverv1beta1.AddToScheme, apiserverv1alpha1.AddToScheme, apiserver.AddToScheme)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
	decoder = serializer.NewCodecFactory(scheme).UniversalDecoder()
}

func admitConfig(authenticationConfigurationRaw string, shoots []*gardencore.Shoot) (int32, error) {
	obj, schemaVersion, err := decoder.Decode([]byte(authenticationConfigurationRaw), nil, nil)
	if err != nil {
		return http.StatusUnprocessableEntity, fmt.Errorf("failed to decode the provided authentication configuration: %w", err)
	}

	authenticationConfig, ok := obj.(*apiserver.AuthenticationConfiguration)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("failed to cast to authentication configuration type: %v", schemaVersion)
	}

	if errList := apiservervalidation.ValidateAuthenticationConfiguration(authenticationcel.NewDefaultCompiler(), authenticationConfig, getDisallowedIssuers(shoots)); len(errList) != 0 {
		return http.StatusUnprocessableEntity, fmt.Errorf("provided invalid authentication configuration: %v", errList)
	}

	if authenticationConfig.Anonymous != nil {
		if anonAuthShoots := getShootsWithLegacyAnonymousAuthentication(shoots); len(anonAuthShoots) > 0 {
			return handleAnonymousAuthenticationConfigurationConflict(anonAuthShoots)
		}
	}

	return 0, nil
}

func getDisallowedIssuers(shoots []*gardencore.Shoot) []string {
	var disallowedIssuers []string
	for _, shoot := range shoots {
		disallowedIssuers = append(disallowedIssuers, getIssuersFromShoot(shoot)...)
	}
	return disallowedIssuers
}

func getIssuersFromShoot(shoot *gardencore.Shoot) []string {
	var issuers []string

	issuers = append(issuers, gardencorehelper.GetShootServiceAccountConfigAcceptedIssuers(shoot.Spec.Kubernetes.KubeAPIServer)...)
	if issuer := gardencorehelper.GetShootServiceAccountConfigIssuer(shoot.Spec.Kubernetes.KubeAPIServer); issuer != nil {
		issuers = append(issuers, *issuer)
	}

	return issuers
}

func getShootsWithLegacyAnonymousAuthentication(shoots []*gardencore.Shoot) []*gardencore.Shoot {
	var filteredShoots []*gardencore.Shoot
	for _, shoot := range shoots {
		if gardencorehelper.IsLegacyAnonymousAuthenticationSet(shoot.Spec.Kubernetes.KubeAPIServer) {
			filteredShoots = append(filteredShoots, shoot)
		}
	}
	return filteredShoots
}

func handleAnonymousAuthenticationConfigurationConflict(shoots []*gardencore.Shoot) (int32, error) {
	var shootNames []string
	for _, s := range shoots {
		shootNames = append(shootNames, s.Name)
	}
	return http.StatusUnprocessableEntity, fmt.Errorf("cannot use anonymous authentication configuration when the following shoots have the legacy configuration enabled: %s", strings.Join(shootNames, ", "))
}
