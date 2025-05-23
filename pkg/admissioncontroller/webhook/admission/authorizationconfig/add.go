// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package authorizationconfig

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/go-logr/logr"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/apis/apiserver"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	apiserverv1beta1 "k8s.io/apiserver/pkg/apis/apiserver/v1beta1"
	apiservervalidation "k8s.io/apiserver/pkg/apis/apiserver/validation"
	authorizationcel "k8s.io/apiserver/pkg/authorization/cel"
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
	HandlerName = "authorizationconfig_validator"
	// WebhookPath is the HTTP handler path for this admission webhook handler.
	WebhookPath = "/webhooks/authorization-configuration"
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

// NewHandler returns a new handler for validating authorization configuration.
func NewHandler(log logr.Logger, apiReader, c client.Reader, decoder admission.Decoder) admission.Handler {
	return &configvalidator.Handler{
		Logger:    log,
		APIReader: apiReader,
		Client:    c,
		Decoder:   decoder,

		ConfigMapPurpose: "authorization configuration",
		ConfigMapDataKey: "config.yaml",
		GetConfigMapNameFromShoot: func(shoot *gardencore.Shoot) string {
			return gardencorehelper.GetShootAuthorizationConfigurationConfigMapName(shoot.Spec.Kubernetes.KubeAPIServer)
		},
		SkipValidationOnShootUpdate: func(shoot, oldShoot *gardencore.Shoot) bool {
			return sets.New[string](getKubeconfigAuthorizerNamesFromShoot(shoot)...).Equal(sets.New[string](getKubeconfigAuthorizerNamesFromShoot(oldShoot)...))
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

func admitConfig(authorizationConfigurationRaw string, shoots []*gardencore.Shoot) (int32, error) {
	obj, schemaVersion, err := decoder.Decode([]byte(authorizationConfigurationRaw), nil, nil)
	if err != nil {
		return http.StatusUnprocessableEntity, fmt.Errorf("failed to decode the provided authorization configuration: %w", err)
	}

	authorizationConfig, ok := obj.(*apiserver.AuthorizationConfiguration)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("failed to cast to authorization configuration type: %v", schemaVersion)
	}

	var authorizerNames []string

	for i, webhook := range authorizationConfig.Authorizers {
		if webhook.Type == apiserver.TypeWebhook && webhook.Webhook != nil {
			authorizerNames = append(authorizerNames, webhook.Name)
			// We do not allow users to set the connection info in the webhook configurations, so let's first validate this:
			if !apiequality.Semantic.DeepEqual(webhook.Webhook.ConnectionInfo, apiserver.WebhookConnectionInfo{}) {
				return http.StatusUnprocessableEntity, fmt.Errorf("provided invalid authorization configuration: %v", field.ErrorList{field.Forbidden(field.NewPath("authorizers").Index(i).Child("connectionInfo"), "connectionInfo is not allowed to be set")})
			}
			// At this point, we have ensured that the connection info is not set. However, below validation function
			// expects it to be set, so let's fake it here to make the function pass (this is not persisted anywhere).
			authorizationConfig.Authorizers[i].Webhook.ConnectionInfo.Type = apiserver.AuthorizationWebhookConnectionInfoTypeInCluster
		}
	}

	for _, shoot := range shoots {
		for _, authorizerName := range authorizerNames {
			if !slices.ContainsFunc(shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization.Kubeconfigs, func(ref gardencore.AuthorizerKubeconfigReference) bool {
				return ref.AuthorizerName == authorizerName
			}) {
				return http.StatusUnprocessableEntity, fmt.Errorf("provided invalid authorization configuration: must provide kubeconfig secret name reference for webhook authorizer %q in shoot %s", authorizerName, client.ObjectKeyFromObject(shoot))
			}
		}
	}

	if errList := apiservervalidation.ValidateAuthorizationConfiguration(authorizationcel.NewDefaultCompiler(), field.NewPath(""), authorizationConfig, sets.New("Webhook"), sets.New("Webhook")); len(errList) != 0 {
		return http.StatusUnprocessableEntity, fmt.Errorf("provided invalid authorization configuration: %v", errList)
	}

	return 0, nil
}

func getKubeconfigAuthorizerNamesFromShoot(shoot *gardencore.Shoot) []string {
	var authorizerNames []string

	if shoot.Spec.Kubernetes.KubeAPIServer != nil && shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization != nil {
		for _, kubeconfig := range shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization.Kubeconfigs {
			authorizerNames = append(authorizerNames, kubeconfig.AuthorizerName)
		}
	}

	return authorizerNames
}
