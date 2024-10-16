// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package authorizationconfig

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/versioning"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/apis/apiserver"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	apiserverv1beta1 "k8s.io/apiserver/pkg/apis/apiserver/v1beta1"
	apiservervalidation "k8s.io/apiserver/pkg/apis/apiserver/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	admissionwebhook "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencoreinstall "github.com/gardener/gardener/pkg/apis/core/install"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

const authorizationConfigurationDataKey = "config.yaml"

var (
	configDecoder   runtime.Decoder
	internalDecoder runtime.Decoder

	shootGK     = schema.GroupKind{Group: "core.gardener.cloud", Kind: "Shoot"}
	configMapGK = schema.GroupKind{Group: "", Kind: "ConfigMap"}
)

func init() {
	authorizationConfigScheme := runtime.NewScheme()
	schemeBuilder := runtime.NewSchemeBuilder(
		apiserverv1beta1.AddToScheme,
		apiserverv1alpha1.AddToScheme,
		apiserver.AddToScheme,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(authorizationConfigScheme))
	configDecoder = serializer.NewCodecFactory(authorizationConfigScheme).UniversalDecoder()

	// create decoder that decodes Shoots from all known API versions to the internal version, but does not perform defaulting
	gardenCoreScheme := runtime.NewScheme()
	gardencoreinstall.Install(gardenCoreScheme)
	codecFactory := serializer.NewCodecFactory(gardenCoreScheme)
	internalDecoder = versioning.NewCodec(nil, codecFactory.UniversalDeserializer(), runtime.UnsafeObjectConvertor(gardenCoreScheme),
		gardenCoreScheme, gardenCoreScheme, nil, runtime.DisabledGroupVersioner, runtime.InternalGroupVersioner, gardenCoreScheme.Name())
}

// Handler validates authorization configurations.
type Handler struct {
	Logger    logr.Logger
	APIReader client.Reader
	Client    client.Reader
	Decoder   admission.Decoder
}

// Handle validates authorization configurations.
func (h *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	requestGK := schema.GroupKind{Group: req.Kind.Group, Kind: req.Kind.Kind}

	switch requestGK {
	case shootGK:
		return h.admitShoot(ctx, req)
	case configMapGK:
		return h.admitConfigMap(ctx, req)
	}
	return admissionwebhook.Allowed("resource is not *core.gardener.cloud/v1beta1.Shoot or *v1.ConfigMap")
}

func (h *Handler) admitShoot(ctx context.Context, request admission.Request) admission.Response {
	shoot := &gardencore.Shoot{}
	if err := runtime.DecodeInto(internalDecoder, request.Object.Raw, shoot); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if shoot.DeletionTimestamp != nil {
		// don't mutate shoot if it's already marked for deletion, otherwise gardener-apiserver will deny the user's/
		// controller's request, because we changed the spec
		return admissionwebhook.Allowed("shoot is already marked for deletion")
	}

	if request.Operation == admissionv1.Update {
		oldShoot := &gardencore.Shoot{}
		if err := runtime.DecodeInto(internalDecoder, request.OldObject.Raw, oldShoot); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		// skip verification if spec wasn't changed
		// this way we make sure, that users/gardenlet can always annotate/label the shoot if the spec doesn't change
		if apiequality.Semantic.DeepEqual(oldShoot.Spec, shoot.Spec) {
			return admissionwebhook.Allowed("shoot spec was not changed")
		}
	}

	authorizationConfigurationConfigMapName := gardencorehelper.GetShootAuthorizationConfigurationConfigMapName(shoot.Spec.Kubernetes.KubeAPIServer)
	if authorizationConfigurationConfigMapName == "" {
		return admissionwebhook.Allowed("shoot resource is not specifying any authorization configuration")
	}

	authorizationConfigurationConfigMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: shoot.Namespace, Name: authorizationConfigurationConfigMapName}}
	if err := h.APIReader.Get(ctx, client.ObjectKeyFromObject(authorizationConfigurationConfigMap), authorizationConfigurationConfigMap); err != nil {
		status := http.StatusInternalServerError
		if apierrors.IsNotFound(err) {
			status = http.StatusUnprocessableEntity
		}
		return admission.Errored(int32(status), fmt.Errorf("could not retrieve ConfigMap: %w", err)) // #nosec G115 -- `status` has two fixed values which fit into int32.
	}
	authorizationConfiguration, err := getAuthorizationConfiguration(authorizationConfigurationConfigMap)
	if err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("error getting authorization configuration from ConfigMap %s: %w", client.ObjectKeyFromObject(authorizationConfigurationConfigMap), err))
	}

	if errCode, err := validateAuthorizationConfigurationSemantics(authorizationConfiguration, shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization.Kubeconfigs, true); err != nil {
		return admission.Errored(errCode, err)
	}

	return admissionwebhook.Allowed("referenced authorization configuration is valid")
}

func (h *Handler) admitConfigMap(ctx context.Context, request admission.Request) admission.Response {
	var (
		oldConfigMap = &corev1.ConfigMap{}
		configMap    = &corev1.ConfigMap{}
	)

	if request.Operation != admissionv1.Update {
		return admissionwebhook.Allowed("operation is not update")
	}

	if err := h.Decoder.Decode(request, configMap); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// lookup if configmap is referenced by any shoot in the same namespace
	shootList := &gardencorev1beta1.ShootList{}
	if err := h.Client.List(ctx, shootList, client.InNamespace(request.Namespace)); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	var configMapIsReferenced bool
	for _, shoot := range shootList.Items {
		if v1beta1helper.GetShootAuthorizationConfigurationConfigMapName(shoot.Spec.Kubernetes.KubeAPIServer) == request.Name {
			configMapIsReferenced = true
			break
		}
	}

	if !configMapIsReferenced {
		return admissionwebhook.Allowed("ConfigMap is not referenced by a Shoot")
	}

	authorizationConfiguration, err := getAuthorizationConfiguration(configMap)
	if err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, err)
	}

	if err = h.getOldObject(request, oldConfigMap); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if apiequality.Semantic.DeepEqual(configMap.Data, oldConfigMap.Data) {
		return admissionwebhook.Allowed("authorization configuration or kubeconfig secret names not changed")
	}

	if errCode, err := validateAuthorizationConfigurationSemantics(authorizationConfiguration, nil, false); err != nil {
		return admission.Errored(errCode, err)
	}

	return admissionwebhook.Allowed("ConfigMap change is valid")
}

func (h *Handler) getOldObject(request admission.Request, oldObj runtime.Object) error {
	if len(request.OldObject.Raw) != 0 {
		return h.Decoder.DecodeRaw(request.OldObject, oldObj)
	}
	return errors.New("could not find old object")
}

func validateAuthorizationConfigurationSemantics(authorizationConfiguration string, kubeconfigReferences []gardencore.AuthorizerKubeconfigReference, verifyKubeconfigReferences bool) (errCode int32, err error) {
	authConfigObj, schemaVersion, err := configDecoder.Decode([]byte(authorizationConfiguration), nil, nil)
	if err != nil {
		return http.StatusUnprocessableEntity, fmt.Errorf("failed to decode the provided authorization configuration: %w", err)
	}
	authConfig, ok := authConfigObj.(*apiserver.AuthorizationConfiguration)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("failure to cast to authorization configuration type: %v", schemaVersion)
	}

	var authorizerNames []string

	for i, webhook := range authConfig.Authorizers {
		if webhook.Type == apiserver.TypeWebhook && webhook.Webhook != nil {
			authorizerNames = append(authorizerNames, webhook.Name)
			// We do not allow users to set the connection info in the webhook configurations, so let's first validate this:
			if !apiequality.Semantic.DeepEqual(webhook.Webhook.ConnectionInfo, apiserver.WebhookConnectionInfo{}) {
				return http.StatusUnprocessableEntity, fmt.Errorf("provided invalid authorization configuration: %v", field.ErrorList{field.Forbidden(field.NewPath("authorizers").Index(i).Child("connectionInfo"), "connectionInfo is not allowed to be set")})
			}
			// At this point, we have ensured that the connection info is not set. However, below validation function
			// expects it to be set, so let's fake it here to make the function pass (this is not persisted anywhere).
			authConfig.Authorizers[i].Webhook.ConnectionInfo.Type = apiserver.AuthorizationWebhookConnectionInfoTypeInCluster
		}
	}

	if verifyKubeconfigReferences {
		for _, authorizerName := range authorizerNames {
			if !slices.ContainsFunc(kubeconfigReferences, func(ref gardencore.AuthorizerKubeconfigReference) bool {
				return ref.AuthorizerName == authorizerName
			}) {
				return http.StatusUnprocessableEntity, fmt.Errorf("provided invalid authorization configuration: must provide kubeconfig secret name reference for webhook authorizer %q", authorizerName)
			}
		}
	}

	if errList := apiservervalidation.ValidateAuthorizationConfiguration(field.NewPath(""), authConfig, sets.NewString("Webhook"), sets.NewString("Webhook")); len(errList) != 0 {
		return http.StatusUnprocessableEntity, fmt.Errorf("provided invalid authorization configuration: %v", errList)
	}

	return 0, nil
}

func getAuthorizationConfiguration(cm *corev1.ConfigMap) (string, error) {
	configRaw, ok := cm.Data[authorizationConfigurationDataKey]
	if !ok {
		return "", fmt.Errorf("missing '.data[%s]' in authorization configuration ConfigMap", authorizationConfigurationDataKey)
	}
	if len(configRaw) == 0 {
		return "", errors.New("empty authorization configuration. Provide non-empty authorization configuration")
	}
	return configRaw, nil
}
