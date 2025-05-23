// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package auditpolicy

import (
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
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

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	"github.com/gardener/gardener/pkg/webhook/configvalidator"
)

const (
	// HandlerName is the name of this admission webhook handler.
	HandlerName = "auditpolicy_validator"
	// WebhookPath is the HTTP handler path for this admission webhook handler.
	WebhookPath = "/webhooks/audit-policies"
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

// NewHandler returns a new handler for validating audit policies.
func NewHandler(log logr.Logger, apiReader, c client.Reader, decoder admission.Decoder) admission.Handler {
	return &configvalidator.Handler{
		Logger:    log,
		APIReader: apiReader,
		Client:    c,
		Decoder:   decoder,

		ConfigMapPurpose: "audit policy",
		ConfigMapDataKey: "policy",
		GetConfigMapNameFromShoot: func(shoot *gardencore.Shoot) string {
			return gardencorehelper.GetShootAuditPolicyConfigMapName(shoot.Spec.Kubernetes.KubeAPIServer)
		},
		AdmitConfig: admitConfig,
	}
}

var decoder runtime.Decoder

func init() {
	scheme := runtime.NewScheme()
	schemeBuilder := runtime.NewSchemeBuilder(auditv1.AddToScheme, audit.AddToScheme)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
	decoder = serializer.NewCodecFactory(scheme).UniversalDecoder()
}

func admitConfig(auditPolicyRaw string, _ []*gardencore.Shoot) (int32, error) {
	obj, schemaVersion, err := decoder.Decode([]byte(auditPolicyRaw), nil, nil)
	if err != nil {
		return http.StatusUnprocessableEntity, fmt.Errorf("failed to decode the provided audit policy: %w", err)
	}

	auditPolicy, ok := obj.(*audit.Policy)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("failed to cast to audit policy type: %v", schemaVersion)
	}

	if errList := auditvalidation.ValidatePolicy(auditPolicy); len(errList) != 0 {
		return http.StatusUnprocessableEntity, fmt.Errorf("provided invalid audit policy: %v", errList)
	}

	return 0, nil
}
