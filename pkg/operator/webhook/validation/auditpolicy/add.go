// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package auditpolicy

import (
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

// NewHandler returns a handler that validates audit policy for Garden and referenced ConfigMaps
func NewHandler(apiReader, c client.Reader, decoderAdmission admission.Decoder, gardenNamespace string) admission.Handler {
	return &configvalidator.Handler{
		APIReader: apiReader,
		Client:    c,
		Decoder:   decoderAdmission,

		ConfigMapPurpose: "audit policy",
		ConfigMapDataKey: "policy",

		GetNamespace: func() string { return gardenNamespace },
		GetConfigMapNameFromGarden: func(garden *operatorv1alpha1.Garden) map[string]string {
			configMapNames := map[string]string{}
			if garden.Spec.VirtualCluster.Gardener.APIServer != nil && garden.Spec.VirtualCluster.Gardener.APIServer.AuditConfig != nil &&
				garden.Spec.VirtualCluster.Gardener.APIServer.AuditConfig.AuditPolicy != nil && garden.Spec.VirtualCluster.Gardener.APIServer.AuditConfig.AuditPolicy.ConfigMapRef != nil {
				configMapNames["gardener-apiserver-audit-policy"] = garden.Spec.VirtualCluster.Gardener.APIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name
			}

			if garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer != nil && garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.AuditConfig != nil &&
				garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy != nil && garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef != nil {
				configMapNames["virtual-garden-kube-apiserver-audit-policy"] = garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name
			}
			return configMapNames
		},
		AdmitGardenConfig: configvalidator.AdmitAudtPolicy,
	}
}
