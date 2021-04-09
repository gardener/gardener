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
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	shootcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/shoot"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/version"

	"github.com/go-logr/logr"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	audit_internal "k8s.io/apiserver/pkg/apis/audit"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	auditv1alpha1 "k8s.io/apiserver/pkg/apis/audit/v1alpha1"
	auditv1beta1 "k8s.io/apiserver/pkg/apis/audit/v1beta1"
	auditvalidation "k8s.io/apiserver/pkg/apis/audit/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	policyDecoder runtime.Decoder

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
		auditv1alpha1.AddToScheme,
		auditv1beta1.AddToScheme,
		auditv1.AddToScheme,
		audit_internal.AddToScheme,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(auditPolicyScheme))
	policyDecoder = serializer.NewCodecFactory(auditPolicyScheme).UniversalDecoder()
}

type handler struct {
	apiReader client.Reader
	decoder   *admission.Decoder
	logger    logr.Logger
}

var _ admission.Handler = &handler{}

func (h *handler) InjectAPIReader(reader client.Reader) error {
	h.apiReader = reader
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

	shoot := &gardencorev1beta1.Shoot{}
	if err := h.decoder.Decode(request, shoot); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if shoot.DeletionTimestamp != nil {
		// don't mutate shoot if it's already marked for deletion, otherwise gardener-apiserver will deny the user's/
		// controller's request, because we changed the spec
		return acadmission.Allowed("shoot is already marked for deletion")
	}

	if !hasAuditPolicy(shoot) {
		return acadmission.Allowed("shoot resource is not specifying any audit policy")
	}
	cmRef := shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef

	auditPolicyCm := &corev1.ConfigMap{}
	if err := h.apiReader.Get(ctx, kutil.Key(shoot.Namespace, cmRef.Name), auditPolicyCm); err != nil {
		if apierrors.IsNotFound(err) {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("referenced audit policy does not exist: namespace: %s, name: %s", shoot.Namespace, cmRef.Name))
		}
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("could not retrieve config map: %s", err))
	}

	auditPolicy, err := getAuditPolicy(auditPolicyCm)
	if err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("error getting auditlog policy from ConfigMap %s/%s: %w", shoot.Namespace, cmRef.Name, err))
	}

	if shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.ResourceVersion == auditPolicyCm.ResourceVersion {
		return acadmission.Allowed("no change detected in referenced configmap holding audit policy")
	}

	schemaVersion, errCode, err := validateAuditPolicySemantics(auditPolicy)
	if err != nil {
		return admission.Errored(errCode, err)
	}

	// Validate audit policy schema version against k8s version of shoot
	if isValidVersion, err := IsValidAuditPolicyVersion(shoot.Spec.Kubernetes.Version, schemaVersion); err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, err)
	} else if !isValidVersion {
		err := fmt.Errorf("your shoot cluster kubernetes version %q is not compatible with audit policy version %q", shoot.Spec.Kubernetes.Version, schemaVersion.GroupVersion().String())
		return admission.Errored(http.StatusUnprocessableEntity, err)
	}

	// If the resource Version in the audit policy cm changed it also needs to be adapted in the configMapRef
	return admission.Patched(
		"referenced audit policy is valid",
		jsonpatch.NewOperation("replace", "/spec/kubernetes/kubeAPIServer/auditConfig/auditPolicy/configMapRef/resourceVersion", auditPolicyCm.ResourceVersion),
	)
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
	if !controllerutil.ContainsFinalizer(cm, shootcontroller.FinalizerName) {
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

	schemaVersion, errCode, err := validateAuditPolicySemantics(auditPolicy)
	if err != nil {
		return admission.Errored(errCode, err)
	}

	// Validate schema version against Shoot Kubernetes version
	shootList := &gardencorev1beta1.ShootList{}
	if err := h.apiReader.List(ctx, shootList, client.InNamespace(request.Namespace)); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if err := validatePolicyAgainstShoots(request, schemaVersion, shootList); err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, err)
	}

	return acadmission.Allowed("configmap change is valid")
}

func validateAuditPolicySemantics(auditPolicy string) (schemaVersion *schema.GroupVersionKind, errCode int32, err error) {
	auditPolicyObj, schemaVersion, err := policyDecoder.Decode([]byte(auditPolicy), nil, nil)
	if err != nil {
		return nil, http.StatusUnprocessableEntity, fmt.Errorf("failed to decode the provided audit policy: %w", err)
	}
	auditPolicyInternal, ok := auditPolicyObj.(*audit_internal.Policy)
	if !ok {
		return nil, http.StatusInternalServerError, fmt.Errorf("failure to cast to audit Policy type: %v", schemaVersion)
	}
	errList := auditvalidation.ValidatePolicy(auditPolicyInternal)
	if len(errList) != 0 {
		return nil, http.StatusUnprocessableEntity, fmt.Errorf("provided invalid audit policy: %v", errList)
	}
	return schemaVersion, 0, nil
}

// IsValidAuditPolicyVersion checks whether the api server support the provided audit policy apiVersion
func IsValidAuditPolicyVersion(shootVersion string, schemaVersion *schema.GroupVersionKind) (bool, error) {
	auditGroupVersion := schemaVersion.GroupVersion().String()

	// TODO Update check once https://github.com/kubernetes/kubernetes/issues/98035 is resolved
	if auditGroupVersion == "audit.k8s.io/v1" {
		return version.CheckVersionMeetsConstraint(shootVersion, ">= v1.12")
	}
	return true, nil
}

func (h *handler) getOldObject(request admission.Request, oldObj runtime.Object) error {
	if len(request.OldObject.Raw) != 0 {
		return h.decoder.DecodeRaw(request.OldObject, oldObj)
	}
	return fmt.Errorf("could not find old object")
}

func getAuditPolicy(cm *corev1.ConfigMap) (auditPolicy string, err error) {
	auditPolicy, ok := cm.Data[auditPolicyConfigMapDataKey]
	if !ok {
		return "", fmt.Errorf("missing '.data.policy' in audit policy configmap")
	}
	if len(auditPolicy) == 0 {
		return "", fmt.Errorf("empty audit policy. Provide non-empty audit policy")
	}
	return auditPolicy, nil
}

func validatePolicyAgainstShoots(request admission.Request, schemaVersion *schema.GroupVersionKind, shoots *gardencorev1beta1.ShootList) error {
	for _, s := range shoots.Items {
		if hasAuditPolicy(&s) {
			if s.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name == request.Name {
				isValidVersion, err := IsValidAuditPolicyVersion(s.Spec.Kubernetes.Version, schemaVersion)
				if err != nil {
					return err
				}
				if !isValidVersion {
					return fmt.Errorf("shoot cluster with name %q and version %q is not compatible with audit policy version %q", s.ObjectMeta.Name, s.Spec.Kubernetes.Version, schemaVersion.GroupVersion().String())
				}
			}
		}
	}
	return nil
}

func hasAuditPolicy(shoot *gardencorev1beta1.Shoot) bool {
	apiServerConfig := shoot.Spec.Kubernetes.KubeAPIServer
	return apiServerConfig != nil &&
		apiServerConfig.AuditConfig != nil &&
		apiServerConfig.AuditConfig.AuditPolicy != nil &&
		apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef != nil &&
		len(apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef.Name) != 0
}
