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

const authenticationConfigurationCmDataKey = "config.yaml"

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

	var oldAuthenticationConfigurationCmName, newAuthenticationConfigurationCmName, oldShootKubernetesVersion, newShootKubernetesVersion string

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
		oldAuthenticationConfigurationCmName = gardencorehelper.GetShootAuthenticationConfigurationConfigMapName(oldShoot.Spec.Kubernetes.KubeAPIServer)
	}
	newShootKubernetesVersion = shoot.Spec.Kubernetes.Version
	newAuthenticationConfigurationCmName = gardencorehelper.GetShootAuthenticationConfigurationConfigMapName(shoot.Spec.Kubernetes.KubeAPIServer)

	if newAuthenticationConfigurationCmName == "" {
		return admissionwebhook.Allowed("shoot resource is not specifying any authentication configuration")
	}

	// oldAuthenticationConfigurationCmName is empty for CREATE shoot requests that specify authentication configuration reference
	// if Kubernetes version is changed we need to revalidate if the authentication configuration API version is compatible with
	// new Kubernetes version
	if oldAuthenticationConfigurationCmName == newAuthenticationConfigurationCmName && oldShootKubernetesVersion == newShootKubernetesVersion {
		return admissionwebhook.Allowed("authentication configuration configmap was not changed")
	}

	authenticationConfigurationCm := &corev1.ConfigMap{}
	if err := h.APIReader.Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: newAuthenticationConfigurationCmName}, authenticationConfigurationCm); err != nil {
		if apierrors.IsNotFound(err) {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("referenced authentication configuration does not exist: namespace: %s, name: %s", shoot.Namespace, newAuthenticationConfigurationCmName))
		}
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("could not retrieve config map: %s", err))
	}

	authenticationConfiguration, err := getAuthenticationConfiguration(authenticationConfigurationCm)
	if err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("error getting authentication configuration from ConfigMap %s/%s: %w", shoot.Namespace, newAuthenticationConfigurationCmName, err))
	}

	if errCode, err := validateAuthenticaionConfigurationSemantics(authenticationConfiguration); err != nil {
		return admission.Errored(errCode, err)
	}

	return admissionwebhook.Allowed("referenced authentication configuration is valid")
}

func (h *Handler) admitConfigMap(ctx context.Context, request admission.Request) admission.Response {
	var (
		oldCm = &corev1.ConfigMap{}
		cm    = &corev1.ConfigMap{}
	)

	if request.Operation != admissionv1.Update {
		return admissionwebhook.Allowed("operation is not update")
	}

	if err := h.Decoder.Decode(request, cm); err != nil {
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

	authenticaionConfiguration, err := getAuthenticationConfiguration(cm)
	if err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, err)
	}

	if err = h.getOldObject(request, oldCm); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	oldAuthenticationConfiguration, ok := oldCm.Data[authenticationConfigurationCmDataKey]
	if ok && oldAuthenticationConfiguration == authenticaionConfiguration {
		return admissionwebhook.Allowed("authentication configuration not changed")
	}

	errCode, err := validateAuthenticaionConfigurationSemantics(authenticaionConfiguration)
	if err != nil {
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

func validateAuthenticaionConfigurationSemantics(authenticaionConfiguration string) (errCode int32, err error) {
	authConfigObj, schemaVersion, err := configDecoder.Decode([]byte(authenticaionConfiguration), nil, nil)
	if err != nil {
		return http.StatusUnprocessableEntity, fmt.Errorf("failed to decode the provided authentication configuration: %w", err)
	}
	authConfig, ok := authConfigObj.(*apiserver.AuthenticationConfiguration)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("failure to cast to authentication configuration type: %v", schemaVersion)
	}
	errList := apiservervalidation.ValidateAuthenticationConfiguration(authConfig)
	if len(errList) != 0 {
		return http.StatusUnprocessableEntity, fmt.Errorf("provided invalid authentication configuration: %v", errList)
	}

	return 0, nil
}

func getAuthenticationConfiguration(cm *corev1.ConfigMap) (string, error) {
	authenticationConfigurationRaw, ok := cm.Data[authenticationConfigurationCmDataKey]
	if !ok {
		return "", fmt.Errorf("missing '.data[%s]' in authentication configuration configmap", authenticationConfigurationCmDataKey)
	}
	if len(authenticationConfigurationRaw) == 0 {
		return "", errors.New("empty authentication configuration. Provide non-empty authentication configuration")
	}
	return authenticationConfigurationRaw, nil
}
