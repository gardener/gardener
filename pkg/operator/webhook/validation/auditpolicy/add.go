// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package auditpolicy

import (
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/apis/audit"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	auditvalidation "k8s.io/apiserver/pkg/apis/audit/validation"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/webhook/configvalidator"
)

const (
	// HandlerName is the name of this admission webhook handler.
	HandlerName = "auditpolicy_validator"
	// WebhookPath is the HTTP handler path for this admission webhook handler.
	WebhookPath = "/webhooks/audit-policies"
)

// AddToManager adds the webhook to the given manager.
func AddToManager(mgr manager.Manager, gardenNamespace string) error {
	webhook := &admission.Webhook{
		Handler: NewHandler(
			mgr.GetAPIReader(),
			mgr.GetClient(),
			admission.NewDecoder(mgr.GetScheme()),
			gardenNamespace,
		),
		RecoverPanic: ptr.To(true),
	}

	mgr.GetWebhookServer().Register(WebhookPath, webhook)
	return nil
}

var decoder runtime.Decoder

func init() {
	scheme := runtime.NewScheme()
	schemeBuilder := runtime.NewSchemeBuilder(auditv1.AddToScheme, audit.AddToScheme)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
	decoder = serializer.NewCodecFactory(scheme).UniversalDecoder()
}

// NewHandler returns a handler that validates audit policy for Garden and referenced ConfigMaps
func NewHandler(apiReader, c client.Reader, decoderAdmission admission.Decoder, gardenNamespace string) admission.Handler {
	return &configvalidator.Handler{
		APIReader: apiReader,
		Client:    c,
		Decoder:   decoderAdmission,

		ConfigMapPurpose: "audit policy",
		ConfigMapDataKey: "policy",

		GetNamespace: func() string { return gardenNamespace },
		GetConfigMapNameFromGarden: func(garden *operatorv1alpha1.Garden) string {
			if garden.Spec.VirtualCluster.Gardener.APIServer != nil && garden.Spec.VirtualCluster.Gardener.APIServer.AuditConfig != nil &&
				garden.Spec.VirtualCluster.Gardener.APIServer.AuditConfig.AuditPolicy != nil && garden.Spec.VirtualCluster.Gardener.APIServer.AuditConfig.AuditPolicy.ConfigMapRef != nil {
				return garden.Spec.VirtualCluster.Gardener.APIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name
			}
			return ""
		},
		AdmitGardenConfig: func(auditPolicyRaw string) (int32, error) {
			obj, schemaVersion, err := decoder.Decode([]byte(auditPolicyRaw), nil, nil)
			if err != nil {
				return http.StatusUnprocessableEntity, fmt.Errorf("failed to decode the provided audit policy: %w", err)
			}
			policy, ok := obj.(*audit.Policy)
			if !ok {
				return http.StatusInternalServerError, fmt.Errorf("failed to cast to audit policy type: %v", schemaVersion)
			}
			if errList := auditvalidation.ValidatePolicy(policy); len(errList) != 0 {
				return http.StatusUnprocessableEntity, fmt.Errorf("provided invalid audit policy: %v", errList)
			}
			return 0, nil
		},
	}
}
