// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package finalizerrestriction

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// HandlerName is the name of this admission webhook handler.
	HandlerName = "finalizer_restriction"
	// WebhookPath is the HTTP handler path for this admission webhook handler.
	WebhookPath = "/webhooks/finalizer-restriction"
)

// AddToManager adds Handler to the given manager.
func AddToManager(mgr manager.Manager) error {
	kubeClient, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	webhook := &admission.Webhook{
		Handler: &Handler{
			Logger:                mgr.GetLogger().WithName("webhook").WithName(HandlerName),
			Decoder:               admission.NewDecoder(mgr.GetScheme()),
			ProtectedFinalizers:   sets.New("gardener", "gardener.cloud/reference-protection"),
			SubjectAccessReviewer: kubeClient.AuthorizationV1().SubjectAccessReviews(),
		},
		RecoverPanic: new(true),
	}

	mgr.GetWebhookServer().Register(WebhookPath, webhook)
	return nil
}
