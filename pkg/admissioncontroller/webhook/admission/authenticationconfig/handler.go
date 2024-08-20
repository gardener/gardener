// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package authenticationconfig

import (
	"context"
	"errors"
	"fmt"
	"net/http"

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
	"k8s.io/apiserver/pkg/apis/apiserver"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
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

const authenticationConfigurationDataKey = "config.yaml"

var (
	configDecoder   runtime.Decoder
	internalDecoder runtime.Decoder

	shootGK     = schema.GroupKind{Group: "core.gardener.cloud", Kind: "Shoot"}
	configmapGK = schema.GroupKind{Group: "", Kind: "ConfigMap"}
)

func init() {
	authenticationConfigScheme := runtime.NewScheme()
	schemeBuilder := runtime.NewSchemeBuilder(
		// TODO(AleksandarSavchev): add v1beta1 when kubernetes packages are updated to version >= v1.30
		apiserverv1alpha1.AddToScheme,
		apiserver.AddToScheme,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(authenticationConfigScheme))
	configDecoder = serializer.NewCodecFactory(authenticationConfigScheme).UniversalDecoder()

	// create decoder that decodes Shoots from all known API versions to the internal version, but does not perform defaulting
	gardencoreScheme := runtime.NewScheme()
	gardencoreinstall.Install(gardencoreScheme)
	codecFactory := serializer.NewCodecFactory(gardencoreScheme)
	internalDecoder = versioning.NewCodec(nil, codecFactory.UniversalDeserializer(), runtime.UnsafeObjectConvertor(gardencoreScheme),
		gardencoreScheme, gardencoreScheme, nil, runtime.DisabledGroupVersioner, runtime.InternalGroupVersioner, gardencoreScheme.Name())
}

// Handler validates authentication configurations.
type Handler struct {
	Logger    logr.Logger
	APIReader client.Reader
	Client    client.Reader
	Decoder   *admission.Decoder
}

// Handle validates authentication configurations.
func (h *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	requestGK := schema.GroupKind{Group: req.Kind.Group, Kind: req.Kind.Kind}

	switch requestGK {
	case shootGK:
		return h.admitShoot(ctx, req)
	case configmapGK:
		return h.admitConfigMap(ctx, req)
	}
	return admissionwebhook.Allowed("resource is not *core.gardener.cloud/v1beta1.Shoot or *corev1.ConfigMap")
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

	var (
		oldAuthenticationConfigurationConfigMapName string
		newAuthenticationConfigurationConfigMapName string
		oldShootKubernetesVersion                   string
		newShootKubernetesVersion                   string
	)

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

		oldShootKubernetesVersion = oldShoot.Spec.Kubernetes.Version
		oldAuthenticationConfigurationConfigMapName = gardencorehelper.GetShootAuthenticationConfigurationConfigMapName(oldShoot.Spec.Kubernetes.KubeAPIServer)
	}
	newShootKubernetesVersion = shoot.Spec.Kubernetes.Version
	newAuthenticationConfigurationConfigMapName = gardencorehelper.GetShootAuthenticationConfigurationConfigMapName(shoot.Spec.Kubernetes.KubeAPIServer)

	if newAuthenticationConfigurationConfigMapName == "" {
		return admissionwebhook.Allowed("shoot resource is not specifying any authentication configuration")
	}

	// oldAuthenticationConfigurationConfigMapName is empty for CREATE shoot requests that specify authentication configuration reference
	// if Kubernetes version is changed we need to revalidate if the authentication configuration API version is compatible with
	// new Kubernetes version
	if oldAuthenticationConfigurationConfigMapName == newAuthenticationConfigurationConfigMapName && oldShootKubernetesVersion == newShootKubernetesVersion {
		return admissionwebhook.Allowed("authentication configuration configmap was not changed")
	}

	authenticationConfigurationConfigMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: shoot.Namespace, Name: newAuthenticationConfigurationConfigMapName}}
	if err := h.APIReader.Get(ctx, client.ObjectKeyFromObject(authenticationConfigurationConfigMap), authenticationConfigurationConfigMap); err != nil {
		status := http.StatusInternalServerError
		if apierrors.IsNotFound(err) {
			status = http.StatusUnprocessableEntity
		}
		return admission.Errored(int32(status), fmt.Errorf("could not retrieve configmap: %w", err))
	}
	authenticationConfiguration, err := getAuthenticationConfiguration(authenticationConfigurationConfigMap)
	if err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("error getting authentication configuration from configmap %s: %w", client.ObjectKeyFromObject(authenticationConfigurationConfigMap), err))
	}

	if errCode, err := validateAuthenticationConfigurationSemantics(authenticationConfiguration); err != nil {
		return admission.Errored(errCode, err)
	}

	return admissionwebhook.Allowed("referenced authentication configuration is valid")
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
		if v1beta1helper.GetShootAuthenticationConfigurationConfigMapName(shoot.Spec.Kubernetes.KubeAPIServer) == request.Name {
			configMapIsReferenced = true
			break
		}
	}

	if !configMapIsReferenced {
		return admissionwebhook.Allowed("configmap is not referenced by a Shoot")
	}

	authenticationConfiguration, err := getAuthenticationConfiguration(configMap)
	if err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, err)
	}

	if err = h.getOldObject(request, oldConfigMap); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	oldAuthenticationConfiguration, ok := oldConfigMap.Data[authenticationConfigurationDataKey]
	if ok && oldAuthenticationConfiguration == authenticationConfiguration {
		return admissionwebhook.Allowed("authentication configuration not changed")
	}

	if errCode, err := validateAuthenticationConfigurationSemantics(authenticationConfiguration); err != nil {
		return admission.Errored(errCode, err)
	}

	return admissionwebhook.Allowed("configmap change is valid")
}

func (h *Handler) getOldObject(request admission.Request, oldObj runtime.Object) error {
	if len(request.OldObject.Raw) != 0 {
		return h.Decoder.DecodeRaw(request.OldObject, oldObj)
	}
	return errors.New("could not find old object")
}

func validateAuthenticationConfigurationSemantics(authenticationConfiguration string) (errCode int32, err error) {
	authConfigObj, schemaVersion, err := configDecoder.Decode([]byte(authenticationConfiguration), nil, nil)
	if err != nil {
		return http.StatusUnprocessableEntity, fmt.Errorf("failed to decode the provided authentication configuration: %w", err)
	}
	authConfig, ok := authConfigObj.(*apiserver.AuthenticationConfiguration)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("failure to cast to authentication configuration type: %v", schemaVersion)
	}
	if errList := apiservervalidation.ValidateAuthenticationConfiguration(authConfig); len(errList) != 0 {
		return http.StatusUnprocessableEntity, fmt.Errorf("provided invalid authentication configuration: %v", errList)
	}

	return 0, nil
}

func getAuthenticationConfiguration(cm *corev1.ConfigMap) (string, error) {
	authenticationConfigurationRaw, ok := cm.Data[authenticationConfigurationDataKey]
	if !ok {
		return "", fmt.Errorf("missing '.data[%s]' in authentication configuration configmap", authenticationConfigurationDataKey)
	}
	if len(authenticationConfigurationRaw) == 0 {
		return "", errors.New("empty authentication configuration. Provide non-empty authentication configuration")
	}
	return authenticationConfigurationRaw, nil
}
