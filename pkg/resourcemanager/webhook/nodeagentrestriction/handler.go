// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package nodeagentrestriction

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/resourcemanager/webhook/nodeagentauthorizer"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var (
	// protectedLabels are the labels on a Node which gardener-node-agent is not allowed to modify because the
	// node-agent-authorizer derives authorization decisions from them.
	protectedLabels = []string{
		v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName,
		corev1.LabelHostname,
	}

	nodeResource                      = corev1.Resource("nodes")
	certificateSigningRequestResource = certificatesv1.Resource("certificatesigningrequests")
)

// Handler prevents gardener-node-agents from widening their own authorization scope: it rejects changes to the Node
// labels which the node-agent-authorizer uses to derive authorization decisions, and CertificateSigningRequests for an
// identity other than the requesting gardener-node-agent's own.
type Handler struct {
	Logger  logr.Logger
	Decoder admission.Decoder
}

// Handle allows the request unless it is a gardener-node-agent trying to escalate its own authorization scope.
func (h *Handler) Handle(_ context.Context, request admission.Request) admission.Response {
	if _, isNodeAgent := nodeagentauthorizer.GetNodeAgentIdentity(&user.DefaultInfo{
		Name:   request.UserInfo.Username,
		Groups: request.UserInfo.Groups,
	}); !isNodeAgent {
		return admission.Allowed("")
	}

	switch (schema.GroupResource{Group: request.Resource.Group, Resource: request.Resource.Resource}) {
	case nodeResource:
		return h.admitNode(request)
	case certificateSigningRequestResource:
		return h.admitCertificateSigningRequest(request)
	default:
		return admission.Allowed("")
	}
}

func (h *Handler) admitNode(request admission.Request) admission.Response {
	if request.Operation != admissionv1.Update {
		return admission.Allowed("")
	}

	oldNode := &corev1.Node{}
	if err := h.Decoder.DecodeRaw(request.OldObject, oldNode); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	newNode := &corev1.Node{}
	if err := h.Decoder.Decode(request, newNode); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	for _, label := range protectedLabels {
		if oldNode.Labels[label] != newNode.Labels[label] {
			h.Logger.Info("Denying request because gardener-node-agent is not allowed to modify the label", "user", request.UserInfo.Username, "node", newNode.Name, "label", label)
			return admission.Denied(fmt.Sprintf("gardener-node-agent is not allowed to modify the %q label on its Node", label))
		}
	}

	return admission.Allowed("")
}

func (h *Handler) admitCertificateSigningRequest(request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Allowed("")
	}

	certificateSigningRequest := &certificatesv1.CertificateSigningRequest{}
	if err := h.Decoder.Decode(request, certificateSigningRequest); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if certificateSigningRequest.Spec.SignerName != certificatesv1.KubeAPIServerClientSignerName {
		return admission.Allowed("")
	}

	x509CertificateRequest, err := utils.DecodeCertificateRequest(certificateSigningRequest.Spec.Request)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if ok, reason := gardenerutils.IsNodeAgentClientCert(x509CertificateRequest, certificateSigningRequest.Spec.Usages); !ok {
		h.Logger.Info("Denying request because gardener-node-agent may only create client certificates for itself", "user", request.UserInfo.Username, "reason", reason)
		return admission.Denied(fmt.Sprintf("gardener-node-agent may only create client certificates for itself: %s", reason))
	}

	if x509CertificateRequest.Subject.CommonName != request.UserInfo.Username {
		h.Logger.Info("Denying request because gardener-node-agent requested a client certificate for a different identity", "user", request.UserInfo.Username, "commonName", x509CertificateRequest.Subject.CommonName)
		return admission.Denied(fmt.Sprintf("gardener-node-agent may only create client certificates for itself, but the common name %q does not match the requesting user %q", x509CertificateRequest.Subject.CommonName, request.UserInfo.Username))
	}

	return admission.Allowed("")
}
