// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package providersecretlabels

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

const (
	// HandlerName is the name of this admission webhook handler.
	HandlerName = "sync-provider-secret-labels"
	// WebhookPath is the HTTP handler path for this admission webhook handler.
	WebhookPath = "/webhooks/sync-provider-secret-labels"
)

// AddToManager adds Handler to the given manager.
func (h *Handler) AddToManager(mgr manager.Manager) error {
	h.secretHandler = admission.
		WithCustomDefaulter(h.Client.Scheme(), &corev1.Secret{}, &SecretHandler{Handler: h}).
		WithRecoverPanic(true)
	h.internalSecretHandler = admission.
		WithCustomDefaulter(h.Client.Scheme(), &gardencorev1beta1.InternalSecret{}, &InternalSecretHandler{Handler: h}).
		WithRecoverPanic(true)

	webhook := &admission.Webhook{
		Handler:      h,
		RecoverPanic: ptr.To(true),
	}

	mgr.GetWebhookServer().Register(WebhookPath, webhook)
	return nil
}
