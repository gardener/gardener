// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package configvalidator

import (
	"context"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	admissionwebhook "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencoreinstall "github.com/gardener/gardener/pkg/apis/core/install"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var (
	gardenCoreScheme  *runtime.Scheme
	gardenCoreDecoder runtime.Decoder
)

func init() {
	gardenCoreScheme = runtime.NewScheme()
	gardencoreinstall.Install(gardenCoreScheme)
	gardenCoreDecoder = versioning.NewCodec(nil, serializer.NewCodecFactory(gardenCoreScheme).UniversalDeserializer(),
		runtime.UnsafeObjectConvertor(gardenCoreScheme), gardenCoreScheme, gardenCoreScheme, nil,
		runtime.DisabledGroupVersioner, runtime.InternalGroupVersioner, gardenCoreScheme.Name())
}

// Handler validates configuration part of ConfigMaps which are referenced in Shoot resources.
type Handler struct {
	Logger    logr.Logger
	APIReader client.Reader
	Client    client.Reader
	Decoder   admission.Decoder

	ConfigMapPurpose            string
	ConfigMapDataKey            string
	GetConfigMapNameFromShoot   func(shoot *gardencore.Shoot) string
	SkipValidationOnShootUpdate func(shoot, oldShoot *gardencore.Shoot) bool
	AdmitConfig                 func(configRaw string, shootsReferencingConfigMap []*gardencore.Shoot) (int32, error)
}

// Handle validates configuration part of ConfigMaps which are referenced in Shoot resources.
func (h *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	requestGK := schema.GroupKind{Group: req.Kind.Group, Kind: req.Kind.Kind}

	switch requestGK {
	case schema.GroupKind{Group: gardencorev1beta1.GroupName, Kind: "Shoot"}:
		return h.admitShoot(ctx, req)
	case schema.GroupKind{Group: corev1.GroupName, Kind: "ConfigMap"}:
		return h.admitConfigMap(ctx, req)
	}

	return admissionwebhook.Allowed("resource is neither of type *core.gardener.cloud/v1beta1.Shoot nor *corev1.ConfigMap")
}

func (h *Handler) admitShoot(ctx context.Context, request admission.Request) admission.Response {
	shoot := &gardencore.Shoot{}
	if err := runtime.DecodeInto(gardenCoreDecoder, request.Object.Raw, shoot); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if shoot.DeletionTimestamp != nil {
		// don't validate shoot if it's already marked for deletion, otherwise gardener-apiserver will deny the user's/
		// controller's request, because we changed the spec
		return admissionwebhook.Allowed("shoot is already marked for deletion")
	}

	configMapName := h.GetConfigMapNameFromShoot(shoot)
	if configMapName == "" {
		return admissionwebhook.Allowed(fmt.Sprintf("Shoot resource does not specify any %s ConfigMap", h.ConfigMapPurpose))
	}

	var oldShoot *gardencore.Shoot
	if request.Operation == admissionv1.Update {
		oldShoot = &gardencore.Shoot{}
		if err := runtime.DecodeInto(gardenCoreDecoder, request.OldObject.Raw, oldShoot); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		// skip verification if spec wasn't changed
		// this way we make sure, that users/gardenlet can always annotate/label the shoot if the spec doesn't change
		if apiequality.Semantic.DeepEqual(oldShoot.Spec, shoot.Spec) {
			return admissionwebhook.Allowed("shoot spec was not changed")
		}
	}

	// `oldConfigMapName` is empty for CREATE shoot requests that specify a ConfigMap reference, hence we can skip the
	// validation.
	// Additionally, we only need to revalidate if the ConfigMap data is compatible with a new Kubernetes version if the
	// version changed.
	if oldShoot != nil &&
		h.GetConfigMapNameFromShoot(oldShoot) == configMapName &&
		oldShoot.Spec.Kubernetes.Version == shoot.Spec.Kubernetes.Version &&
		(h.SkipValidationOnShootUpdate == nil || h.SkipValidationOnShootUpdate(shoot, oldShoot)) {
		return admissionwebhook.Allowed(fmt.Sprintf("Neither %s ConfigMap nor Kubernetes version or other relevant fields were changed", h.ConfigMapPurpose))
	}

	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: shoot.Namespace, Name: configMapName}}
	if err := h.APIReader.Get(ctx, client.ObjectKeyFromObject(configMap), configMap); err != nil {
		if apierrors.IsNotFound(err) {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("referenced %s ConfigMap %s does not exist: %w", h.ConfigMapPurpose, client.ObjectKeyFromObject(configMap), err))
		}
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("could not retrieve %s ConfigMap %s: %w", h.ConfigMapPurpose, client.ObjectKeyFromObject(configMap), err))
	}

	configRaw, err := h.rawConfigurationFromConfigMap(configMap.Data)
	if err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("error getting %s from ConfigMap %s: %w", h.ConfigMapPurpose, client.ObjectKeyFromObject(configMap), err))
	}

	if errCode, err := h.AdmitConfig(configRaw, []*gardencore.Shoot{shoot}); err != nil {
		return admission.Errored(errCode, err)
	}

	return admissionwebhook.Allowed(fmt.Sprintf("referenced %s is valid", h.ConfigMapPurpose))
}

func (h *Handler) admitConfigMap(ctx context.Context, request admission.Request) admission.Response {
	var (
		newConfigMap = &corev1.ConfigMap{}
		oldConfigMap = &corev1.ConfigMap{}
	)

	if request.Operation != admissionv1.Update {
		return admissionwebhook.Allowed("operation is not update, nothing to validate")
	}

	if err := h.Decoder.Decode(request, newConfigMap); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// lookup if ConfigMap is referenced by any shoot in the same namespace
	shootList := &gardencorev1beta1.ShootList{}
	if err := h.Client.List(ctx, shootList, client.InNamespace(request.Namespace)); err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed listing shoots in namespace %s: %w", request.Namespace, err))
	}

	var shoots []*gardencore.Shoot
	for _, obj := range shootList.Items {
		shoot := &gardencore.Shoot{}
		if err := gardenCoreScheme.Convert(&obj, shoot, nil); err != nil {
			return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed converting Shoot %s into internal version: %w", client.ObjectKeyFromObject(&obj), err))
		}

		if h.GetConfigMapNameFromShoot(shoot) == request.Name {
			shoots = append(shoots, shoot)
		}
	}

	if len(shoots) == 0 {
		return admissionwebhook.Allowed("ConfigMap is not referenced by a Shoot")
	}

	configRaw, err := h.rawConfigurationFromConfigMap(newConfigMap.Data)
	if err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("error getting %s from ConfigMap %s: %w", h.ConfigMapPurpose, client.ObjectKeyFromObject(newConfigMap), err))
	}

	if err = h.Decoder.DecodeRaw(request.OldObject, oldConfigMap); err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("error decoding old ConfigMap: %w", err))
	}

	if oldConfigRaw, ok := oldConfigMap.Data[h.ConfigMapDataKey]; ok && oldConfigRaw == configRaw {
		return admissionwebhook.Allowed(fmt.Sprintf("%s did not change", h.ConfigMapPurpose))
	}

	if errCode, err := h.AdmitConfig(configRaw, shoots); err != nil {
		return admission.Errored(errCode, err)
	}

	return admissionwebhook.Allowed(fmt.Sprintf("referenced %s is valid", h.ConfigMapPurpose))
}

func (h *Handler) rawConfigurationFromConfigMap(data map[string]string) (string, error) {
	configRaw, ok := data[h.ConfigMapDataKey]
	if !ok {
		return "", fmt.Errorf("missing %s key in %s ConfigMap data", h.ConfigMapPurpose, h.ConfigMapDataKey)
	}

	if len(configRaw) == 0 {
		return "", fmt.Errorf("%s in %s key is empty", h.ConfigMapPurpose, h.ConfigMapDataKey)
	}

	return configRaw, nil
}
