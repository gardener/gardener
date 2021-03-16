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
	"reflect"
	"time"

	acadmission "github.com/gardener/gardener/pkg/admissioncontroller/webhooks/admission"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/version"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
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
	WebhookPath = "/webhooks/validate-audit-policies"

	auditPolicyConfigMapDataKey = "policy"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	decoder       = codecs.UniversalDecoder()
	_             = auditv1alpha1.AddToScheme(runtimeScheme)
	_             = auditv1beta1.AddToScheme(runtimeScheme)
	_             = auditv1.AddToScheme(runtimeScheme)
	_             = audit_internal.AddToScheme(runtimeScheme)

	shootGVK     = metav1.GroupVersionKind{Group: "core.gardener.cloud", Kind: "Shoot", Version: "v1beta1"}
	configmapGVK = metav1.GroupVersionKind{Group: "", Kind: "ConfigMap", Version: "v1"}
)

// New creates a new handler for validating audit policies.
func New(logger logr.Logger) *handler {
	return &handler{
		logger: logger,
	}
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
	defer cancel()

	switch request.Kind {
	case shootGVK:
		return h.admitShoot(ctx, request)
	case configmapGVK:
		return h.admitConfigMap(ctx, request)
	}
	return acadmission.Allowed("resource is not core.gardener.cloud.v1beta1.shoot or corev1.configmap")
}

func (h *handler) admitShoot(ctx context.Context, request admission.Request) admission.Response {
	var (
		shoot    = &gardencorev1beta1.Shoot{}
		oldShoot = &gardencorev1beta1.Shoot{}
	)
	if request.Operation != admissionv1.Create && request.Operation != admissionv1.Update {
		return acadmission.Allowed("operation is not Create or Update")
	}

	if err := h.decoder.Decode(request, shoot); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if !hasAuditPolicy(shoot) {
		return acadmission.Allowed("shoot resource is not specifying any audit policy")
	}
	cmRef := shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef

	if request.Operation == admissionv1.Update {
		if err := h.getOldObject(request, oldShoot); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		if hasAuditPolicy(oldShoot) && reflect.DeepEqual(cmRef, oldShoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef) {
			return acadmission.Allowed("audit policy reference did not change in Shoot resource")
		}
	}

	auditPolicyCm := &corev1.ConfigMap{}
	if err := h.apiReader.Get(ctx, kutil.Key(shoot.Namespace, cmRef.Name), auditPolicyCm); err != nil {
		if apierrors.IsNotFound(err) {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("referenced audit policy does not exist: namespace: %s, name: %s", cmRef.Namespace, cmRef.Name))
		}
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("could not retrieve config map: %s", err))
	}
	auditPolicy, err := getAuditPolicy(auditPolicyCm)
	if err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("Error: %s in %v/%v", err, cmRef.Namespace, cmRef.Name))
	}

	// Validate audit policy semantically
	auditPolicyObj, schemaVersion, err := decoder.Decode([]byte(auditPolicy), nil, nil)
	if err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("failed to decode the provided audit policy err=%v", err))
	}
	auditPolicyInternal, ok := auditPolicyObj.(*audit_internal.Policy)
	if !ok {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failure to cast to audit Policy type: %v", schemaVersion))
	}
	errList := auditvalidation.ValidatePolicy(auditPolicyInternal)
	if len(errList) != 0 {
		return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("provided invalid audit policy err=%v", errList))
	}

	// Validate audit policy schema version against k8s version of shoot
	if isValidVersion, err := IsValidAuditPolicyVersion(shoot.Spec.Kubernetes.Version, schemaVersion); err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, err)
	} else if !isValidVersion {
		err := fmt.Errorf("your shoot cluster version %q is not compatible with audit policy version %q", shoot.Spec.Kubernetes.Version, schemaVersion.GroupVersion().String())
		return admission.Errored(http.StatusUnprocessableEntity, err)
	}

	return acadmission.Allowed("referenced audit policy is valid")
}

func (h *handler) admitConfigMap(ctx context.Context, request admission.Request) admission.Response {
	var (
		oldCm = &corev1.ConfigMap{}
		cm    = &corev1.ConfigMap{}
	)
	if err := h.getOldObject(request, oldCm); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if request.Operation == admissionv1.Delete {
		if controllerutil.ContainsFinalizer(oldCm, shoot.FinalizerName) {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("deletion of audit-policy is forbidden as it is still referenced by a shoot (cm has finalizer)"))
		}
		return acadmission.Allowed("deletion of audit-policy accepted")
	}

	if request.Operation == admissionv1.Update {
		if err := h.decoder.Decode(request, cm); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if !controllerutil.ContainsFinalizer(cm, shoot.FinalizerName) {
			return acadmission.Allowed("configmap is not referenced by a Shoot")
		}

		auditPolicy, err := getAuditPolicy(cm)
		if err != nil {
			return admission.Errored(http.StatusUnprocessableEntity, err)
		}

		oldAuditPolicy, ok := oldCm.Data[auditPolicyConfigMapDataKey]
		if ok {
			if oldAuditPolicy == auditPolicy {
				return acadmission.Allowed("audit policy did not change")
			}
		}

		// Validate audit policy semantically
		auditPolicyObj, schemaVersion, err := decoder.Decode([]byte(auditPolicy), nil, nil)
		if err != nil {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("failed to decode the provided audit policy err=%v", err))
		}
		auditPolicyInternal, ok := auditPolicyObj.(*audit_internal.Policy)
		if !ok {
			return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failure to cast to audit Policy type: %v", schemaVersion))
		}
		errList := auditvalidation.ValidatePolicy(auditPolicyInternal)
		if len(errList) != 0 {
			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("provided invalid audit policy err=%v", errList))
		}

		// Validate schema version against Shoot Kubernetes version
		existingShoots := &gardencorev1beta1.ShootList{}
		if err := h.apiReader.List(ctx, existingShoots, client.InNamespace(request.Namespace)); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if err := validatePolicyAgainstShoots(request, schemaVersion, existingShoots); err != nil {
			return admission.Errored(http.StatusUnprocessableEntity, err)
		}

		return acadmission.Allowed("configmap change is valid")
	}
	return acadmission.Allowed("operation is not update or delete")
}

// IsValidAuditPolicyVersion checks whether the api server support the provided audit policy apiVersion
func IsValidAuditPolicyVersion(shootVersion string, schemaVersion *schema.GroupVersionKind) (bool, error) {
	auditGroupVersion := schemaVersion.GroupVersion().String()

	if auditGroupVersion == "audit.k8s.io/v1" {
		return version.CheckVersionMeetsConstraint(shootVersion, ">= v1.12")
	}
	return true, nil
}

func (h *handler) getOldObject(request admission.Request, oldObj runtime.Object) error {
	if len(request.OldObject.Raw) != 0 {
		if err := h.decoder.DecodeRaw(request.OldObject, oldObj); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("could not find old object")
	}
	return nil
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
			cmRef := s.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef
			if cmRef.Name == request.Name {
				if isValidVersion, err := IsValidAuditPolicyVersion(s.Spec.Kubernetes.Version, schemaVersion); err != nil {
					return err
				} else if !isValidVersion {
					err := fmt.Errorf("your shoot cluster version %q is not compatible with audit policy version %q", s.Spec.Kubernetes.Version, schemaVersion.GroupVersion().String())
					return err
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
