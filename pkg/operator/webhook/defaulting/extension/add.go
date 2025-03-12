// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

const (
	// HandlerName is the name of this admission webhook handler.
	HandlerName = "extension-defaulter"
	// WebhookPath is the HTTP handler path for this admission webhook handler.
	WebhookPath = "/webhooks/default-operator-gardener-cloud-v1alpha1-extension"
)

// AddToManager adds Handler to the given manager.
func (h *Handler) AddToManager(mgr manager.Manager) error {
	webhook := admission.
		WithCustomDefaulter(mgr.GetScheme(), &operatorv1alpha1.Extension{}, h).
		WithRecoverPanic(true)

	mgr.GetWebhookServer().Register(WebhookPath, webhook)
	return nil
}
