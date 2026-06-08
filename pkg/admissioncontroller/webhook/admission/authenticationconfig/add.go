// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package authenticationconfig

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/apis/apiserver"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	apiserverv1beta1 "k8s.io/apiserver/pkg/apis/apiserver/v1beta1"
	apiservervalidation "k8s.io/apiserver/pkg/apis/apiserver/validation"
	authenticationcel "k8s.io/apiserver/pkg/authentication/cel"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	gardencorehelper "github.com/gardener/gardener/pkg/api/core/helper"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
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
			mgr.GetAPIReader(),
			mgr.GetClient(),
			admission.NewDecoder(mgr.GetScheme()),
		),
		RecoverPanic: new(true),
	}

	mgr.GetWebhookServer().Register(WebhookPath, webhook)
	return nil
}

// NewHandler returns a new handler for validating authentication configuration.
func NewHandler(apiReader, c client.Reader, decoder admission.Decoder) admission.Handler {
	h := &handler{apiReader: apiReader}
	return &configvalidator.Handler{
		APIReader: apiReader,
		Client:    c,
		Decoder:   decoder,

		ConfigMapPurpose: "authentication configuration",
		ConfigMapDataKey: "config.yaml",
		GetConfigMapNameFromShoot: func(shoot *gardencore.Shoot) string {
			return gardencorehelper.GetShootAuthenticationConfigurationConfigMapName(shoot.Spec.Kubernetes.KubeAPIServer)
		},
		SkipValidationOnShootUpdate: func(shoot, oldShoot *gardencore.Shoot) bool {
			if gardencorehelper.HasManagedIssuer(shoot) != gardencorehelper.HasManagedIssuer(oldShoot) {
				return false
			}
			if !gardencorehelper.IsLegacyAnonymousAuthenticationSet(oldShoot.Spec.Kubernetes.KubeAPIServer) && gardencorehelper.IsLegacyAnonymousAuthenticationSet(shoot.Spec.Kubernetes.KubeAPIServer) {
				return false // Don't skip validation when the deprecated anonymous authentication is being set.
			}
			return sets.New(getIssuersFromShoot(shoot)...).Equal(sets.New(getIssuersFromShoot(oldShoot)...))
		},
		AdmitConfig: h.admitConfig,
	}
}

type handler struct {
	apiReader client.Reader
}

var decoder runtime.Decoder

func init() {
	scheme := runtime.NewScheme()
	schemeBuilder := runtime.NewSchemeBuilder(apiserverv1beta1.AddToScheme, apiserverv1alpha1.AddToScheme, apiserver.AddToScheme)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
	decoder = serializer.NewCodecFactory(scheme).UniversalDecoder()
}

func (h *handler) admitConfig(ctx context.Context, authenticationConfigurationRaw string, shoots []*gardencore.Shoot) (int32, error) {
	obj, schemaVersion, err := decoder.Decode([]byte(authenticationConfigurationRaw), nil, nil)
	if err != nil {
		return http.StatusUnprocessableEntity, fmt.Errorf("failed to decode the provided authentication configuration: %w", err)
	}

	authenticationConfig, ok := obj.(*apiserver.AuthenticationConfiguration)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("failed to cast to authentication configuration type: %v", schemaVersion)
	}

	disallowedIssuers, err := h.getDisallowedIssuers(ctx, shoots)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if errList := apiservervalidation.ValidateAuthenticationConfiguration(authenticationcel.NewDefaultCompiler(), authenticationConfig, disallowedIssuers); len(errList) != 0 {
		return http.StatusUnprocessableEntity, fmt.Errorf("provided invalid authentication configuration: %v", errList)
	}

	if authenticationConfig.Anonymous != nil {
		if anonAuthShoots := getShootsWithLegacyAnonymousAuthentication(shoots); len(anonAuthShoots) > 0 {
			return handleAnonymousAuthenticationConfigurationConflict(anonAuthShoots)
		}
	}

	return 0, nil
}

func (h *handler) getDisallowedIssuers(ctx context.Context, shoots []*gardencore.Shoot) ([]string, error) {
	var disallowedIssuers []string
	for _, shoot := range shoots {
		disallowedIssuers = append(disallowedIssuers, getIssuersFromShoot(shoot)...)
	}

	computedIssuers, err := h.computeManagedIssuers(ctx, shoots)
	if err != nil {
		return nil, err
	}
	disallowedIssuers = append(disallowedIssuers, computedIssuers...)

	return disallowedIssuers, nil
}

func (h *handler) computeManagedIssuers(ctx context.Context, shoots []*gardencore.Shoot) ([]string, error) {
	var managedShoots []*gardencore.Shoot
	for _, shoot := range shoots {
		if gardencorehelper.HasManagedIssuer(shoot) && shoot.UID != "" {
			managedShoots = append(managedShoots, shoot)
		}
	}

	if len(managedShoots) == 0 {
		return nil, nil
	}

	hostname, err := h.getServiceAccountIssuerHostname(ctx)
	if err != nil {
		return nil, err
	}
	if hostname == "" {
		return nil, nil
	}

	var issuers []string
	for _, shoot := range managedShoots {
		projectName, err := h.getProjectName(ctx, shoot.Namespace)
		if err != nil {
			return nil, err
		}
		if projectName == "" {
			continue
		}
		issuers = append(issuers, gardenerutils.ComputeManagedServiceAccountIssuerURL(hostname, projectName, string(shoot.UID)))
	}

	return issuers, nil
}

func (h *handler) getServiceAccountIssuerHostname(ctx context.Context) (string, error) {
	secretList := &corev1.SecretList{}
	if err := h.apiReader.List(ctx, secretList,
		client.InNamespace(v1beta1constants.GardenNamespace),
		client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShootServiceAccountIssuer},
	); err != nil {
		return "", fmt.Errorf("failed to list service account issuer secrets: %w", err)
	}

	if len(secretList.Items) == 0 {
		return "", nil
	}

	return string(secretList.Items[0].Data["hostname"]), nil
}

func (h *handler) getProjectName(ctx context.Context, namespace string) (string, error) {
	ns := &corev1.Namespace{}
	if err := h.apiReader.Get(ctx, types.NamespacedName{Name: namespace}, ns); err != nil {
		return "", fmt.Errorf("failed to get namespace %s: %w", namespace, err)
	}

	return ns.Labels[v1beta1constants.ProjectName], nil
}

func getIssuersFromShoot(shoot *gardencore.Shoot) []string {
	var issuers []string

	issuers = append(issuers, gardencorehelper.GetShootServiceAccountConfigAcceptedIssuers(shoot.Spec.Kubernetes.KubeAPIServer)...)
	if issuer := gardencorehelper.GetShootServiceAccountConfigIssuer(shoot.Spec.Kubernetes.KubeAPIServer); issuer != nil {
		issuers = append(issuers, *issuer)
	}

	for _, addr := range shoot.Status.AdvertisedAddresses {
		switch addr.Name {
		case v1beta1constants.AdvertisedAddressInternal,
			v1beta1constants.AdvertisedAddressExternal:
			issuers = append(issuers, addr.URL)
		}
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
