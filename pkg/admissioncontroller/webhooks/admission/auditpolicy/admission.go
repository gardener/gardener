// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auditpolicy

import (
	"context"
	"fmt"
	"net/http"
	"time"

	acadmission "github.com/gardener/gardener/pkg/admissioncontroller/webhooks/admission"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencoreinstall "github.com/gardener/gardener/pkg/apis/core/install"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

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
	audit_internal "k8s.io/apiserver/pkg/apis/audit"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	auditvalidation "k8s.io/apiserver/pkg/apis/audit/validation"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// HandlerName is the name of this admission webhook handler.
	HandlerName = "auditpolicy_validator"
	// WebhookPath is the HTTP handler path for this admission webhook handler.
	WebhookPath = "/webhooks/audit-policies"

	auditPolicyConfigMapDataKey = "policy"
)

var (
	policyDecoder   runtime.Decoder
	internalDecoder runtime.Decoder

	shootGK     = schema.GroupKind{Group: "core.gardener.cloud", Kind: "Shoot"}
	configmapGK = schema.GroupKind{Group: "", Kind: "ConfigMap"}
)

// New creates a new handler for validating audit policies.
func New(logger logr.Logger) *handler {
	return &handler{
		logger: logger,
	}
}

func init() {
	auditPolicyScheme := runtime.NewScheme()
	schemeBuilder := runtime.NewSchemeBuilder(
		auditv1.AddToScheme,
		audit_internal.AddToScheme,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(auditPolicyScheme))
	policyDecoder = serializer.NewCodecFactory(auditPolicyScheme).UniversalDecoder()

	// create decoder that decodes Shoots from all known API versions to the internal version, but does not perform defaulting
	gardencoreScheme := runtime.NewScheme()
	gardencoreinstall.Install(gardencoreScheme)
	codecFactory := serializer.NewCodecFactory(gardencoreScheme)
	internalDecoder = versioning.NewCodec(nil, codecFactory.UniversalDeserializer(), runtime.UnsafeObjectConvertor(gardencoreScheme),
		gardencoreScheme, gardencoreScheme, nil, runtime.DisabledGroupVersioner, runtime.InternalGroupVersioner, gardencoreScheme.Name())
}

type handler struct {
	apiReader client.Reader
	cache     client.Reader
	decoder   *admission.Decoder
	logger    logr.Logger
}

var _ admission.Handler = &handler{}

func (h *handler) InjectAPIReader(reader client.Reader) error {
	h.apiReader = reader
	return nil
}

func (h *handler) InjectCache(cache cache.Cache) error {
	h.cache = cache
	return nil
}

func (h *handler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

// Handle implements the webhook handler for audit log policy validation
func (h *handler) Handle(ctx context.Context, request admission.Request) admission.Response {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	requestGK := schema.GroupKind{Group: request.Kind.Group, Kind: request.Kind.Kind}
	defer cancel()

	switch requestGK {
	case shootGK:
		return h.admitShoot(ctx, request)
	case configmapGK:
		return h.admitConfigMap(ctx, request)
	}
	return acadmission.Allowed("resource is not core.gardener.cloud/v1beta1.shoot or v1.configmap")
}

func (h *handler) admitShoot(ctx context.Context, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create && request.Operation != admissionv1.Update {
		return acadmission.Allowed("operation is not Create or Update")
	}

	if request.SubResource != "" {
		return acadmission.Allowed("subresources are not handled")
	}

	shoot := &gardencore.Shoot{}
	if err := runtime.DecodeInto(internalDecoder, request.Object.Raw, shoot); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if shoot.DeletionTimestamp != nil {
		// don't mutate shoot if it's already marked for deletion, otherwise gardener-apiserver will deny the user's/
		// controller's request, because we changed the spec
		return acadmission.Allowed("shoot is already marked for deletion")
	}

	var oldAuditPolicyConfigMapName, newAuditPolicyConfigMapName string

	if request.Operation == admissionv1.Update {
		oldShoot := &gardencore.Shoot{}
		if err := runtime.DecodeInto(internalDecoder, request.OldObject.Raw, oldShoot); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		// skip verification if spec wasn't changed
		// this way we make sure, that users/gardenlet can always annotate/label the shoot if the spec doesn't change
		if apiequality.Semantic.DeepEqual(oldShoot.Spec, shoot.Spec) {
			return acadmission.Allowed("shoot spec was not changed")
		}

		oldAuditPolicyConfigMapName = gardencorehelper.GetShootAuditPolicyConfigMapName(oldShoot.Spec.Kubernetes.KubeAPIServer)
	}
	newAuditPolicyConfigMapName = gardencorehelper.GetShootAuditPolicyConfigMapName(shoot.Spec.Kubernetes.KubeAPIServer)

	if newAuditPolicyConfigMapName == "" {
		return acadmission.Allowed("shoot resource is not specifying any audit policy")
	}

	// oldAuditPolicyConfigMapName is empty for CREATE shoot requests that specify audit policy reference
	if oldAuditPolicyConfigMapName == newAuditPolicyConfigMapName {
		return acadmission.Allowed("audit policy configmap was not changed")
	}

	auditPolicyCm := &corev1.ConfigMap{}
	if err := h.apiReader.Get(ctx, kutil.Key(shoot.Namespace, newAuditPolicyConfigMapName), auditPolicyCm); err != nil {
		if apierrors.IsNotFound(err) {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("referenced audit policy does not exist: namespace: %s, name: %s", shoot.Namespace, newAuditPolicyConfigMapName))
		}
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("could not retrieve config map: %s", err))
	}

	auditPolicy, err := getAuditPolicy(auditPolicyCm)
	if err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("error getting auditlog policy from ConfigMap %s/%s: %w", shoot.Namespace, newAuditPolicyConfigMapName, err))
	}

	errCode, err := validateAuditPolicySemantics(auditPolicy)
	if err != nil {
		return admission.Errored(errCode, err)
	}

	return acadmission.Allowed("referenced audit policy is valid")
}

func (h *handler) admitConfigMap(ctx context.Context, request admission.Request) admission.Response {
	var (
		oldCm = &corev1.ConfigMap{}
		cm    = &corev1.ConfigMap{}
	)

	if request.Operation != admissionv1.Update {
		return acadmission.Allowed("operation is not update")
	}

	if err := h.decoder.Decode(request, cm); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// lookup if configmap is referenced by any shoot in the same namespace
	shootList := &gardencorev1beta1.ShootList{}
	if err := h.cache.List(ctx, shootList, client.InNamespace(request.Namespace)); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	configMapIsReferenced := false
	for _, shoot := range shootList.Items {
		if gardencorev1beta1helper.GetShootAuditPolicyConfigMapName(shoot.Spec.Kubernetes.KubeAPIServer) == request.Name {
			configMapIsReferenced = true
			break
		}
	}

	if !configMapIsReferenced {
		return acadmission.Allowed("configmap is not referenced by a Shoot")
	}

	auditPolicy, err := getAuditPolicy(cm)
	if err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, err)
	}

	if err = h.getOldObject(request, oldCm); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	oldAuditPolicy, ok := oldCm.Data[auditPolicyConfigMapDataKey]
	if ok && oldAuditPolicy == auditPolicy {
		return acadmission.Allowed("audit policy not changed")
	}

	errCode, err := validateAuditPolicySemantics(auditPolicy)
	if err != nil {
		return admission.Errored(errCode, err)
	}

	return acadmission.Allowed("configmap change is valid")
}

func validateAuditPolicySemantics(auditPolicy string) (errCode int32, err error) {
	auditPolicyObj, schemaVersion, err := policyDecoder.Decode([]byte(auditPolicy), nil, nil)
	if err != nil {
		return http.StatusUnprocessableEntity, fmt.Errorf("failed to decode the provided audit policy: %w", err)
	}
	auditPolicyInternal, ok := auditPolicyObj.(*audit_internal.Policy)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("failure to cast to audit Policy type: %v", schemaVersion)
	}
	errList := auditvalidation.ValidatePolicy(auditPolicyInternal)
	if len(errList) != 0 {
		return http.StatusUnprocessableEntity, fmt.Errorf("provided invalid audit policy: %v", errList)
	}
	return 0, nil
}

func (h *handler) getOldObject(request admission.Request, oldObj runtime.Object) error {
	if len(request.OldObject.Raw) != 0 {
		return h.decoder.DecodeRaw(request.OldObject, oldObj)
	}
	return fmt.Errorf("could not find old object")
}

func getAuditPolicy(cm *corev1.ConfigMap) (string, error) {
	auditPolicy, ok := cm.Data[auditPolicyConfigMapDataKey]
	if !ok {
		return "", fmt.Errorf("missing '.data.policy' in audit policy configmap")
	}
	if len(auditPolicy) == 0 {
		return "", fmt.Errorf("empty audit policy. Provide non-empty audit policy")
	}
	return auditPolicy, nil
}
