// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package auditpolicy

import (
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
func NewHandler(apiReader, c client.Reader, decoder admission.Decoder) admission.Handler {
	return &configvalidator.Handler{
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

func admitConfig(auditPolicyRaw string, _ []*gardencore.Shoot) (int32, error) {
	return configvalidator.AdmitAudtPolicy(auditPolicyRaw)
}
