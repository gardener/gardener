// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagentauthorizer

import (
	"context"
	"fmt"
	"slices"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	auth "k8s.io/apiserver/pkg/authorization/authorizer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// NewAuthorizer returns a new authorizer for requests from gardener-node-agents. It never has an opinion on the request.
func NewAuthorizer(logger logr.Logger, sourceClient, targetClient client.Client, machineNamespace string, authorizeWithSelectors bool) *authorizer {
	return &authorizer{
		sourceClient:           sourceClient,
		targetClient:           targetClient,
		logger:                 logger,
		machineNamespace:       machineNamespace,
		authorizeWithSelectors: authorizeWithSelectors,
	}
}

const valitailTokenSecretName = "gardener-valitail"

var (
	certificateSigningRequestResource = certificatesv1.Resource("certificatesigningrequests")
	eventCoreResource                 = corev1.Resource("events")
	eventResource                     = eventsv1.Resource("events")
	leaseResource                     = coordinationv1.Resource("leases")
	nodeResource                      = corev1.Resource("nodes")
	podResource                       = corev1.Resource("pods")
	secretsResource                   = corev1.Resource("secrets")
)

type authorizer struct {
	sourceClient           client.Client
	targetClient           client.Client
	logger                 logr.Logger
	machineNamespace       string
	authorizeWithSelectors bool
}

var _ auth.Authorizer = (*authorizer)(nil)

func (a *authorizer) Authorize(ctx context.Context, attrs auth.Attributes) (auth.Decision, string, error) {
	machineName, isNodeAgent := GetNodeAgentIdentity(attrs.GetUser())
	if !isNodeAgent {
		return auth.DecisionNoOpinion, "", nil
	}

	requestLog := a.logger.WithValues(
		"user", attrs.GetUser().GetName(), "verb", attrs.GetVerb(),
		"namespace", attrs.GetNamespace(), "resource", attrs.GetResource(), "subresource", attrs.GetSubresource(),
	)

	if machineName == "" {
		requestLog.Info("No machine for user")
		return auth.DecisionDeny, "", nil
	}

	if attrs.IsResourceRequest() {
		requestResource := schema.GroupResource{Group: attrs.GetAPIGroup(), Resource: attrs.GetResource()}
		switch requestResource {
		case certificateSigningRequestResource:
			return a.authorizeCertificateSigningRequest(ctx, requestLog, attrs)
		case eventCoreResource, eventResource:
			return a.authorizeEvent(requestLog, attrs)
		case leaseResource:
			return a.authorizeLease(ctx, requestLog, machineName, attrs)
		case nodeResource:
			return a.authorizeNode(ctx, requestLog, machineName, attrs)
		case podResource:
			return a.authorizePod(ctx, requestLog, machineName, attrs)
		case secretsResource:
			return a.authorizeSecret(ctx, requestLog, machineName, attrs)
		}
	}

	return auth.DecisionDeny, "", nil
}

func (a *authorizer) authorizeCertificateSigningRequest(ctx context.Context, log logr.Logger, attrs auth.Attributes) (auth.Decision, string, error) {
	if ok, reason := a.checkSubresource(log, attrs); !ok {
		return auth.DecisionDeny, reason, nil
	}

	if allowed, reason := a.checkVerb(log, attrs, "get", "create"); !allowed {
		return auth.DecisionDeny, reason, nil
	}

	if attrs.GetVerb() == "create" {
		return auth.DecisionAllow, "", nil
	}

	csr := &certificatesv1.CertificateSigningRequest{}
	if err := a.targetClient.Get(ctx, types.NamespacedName{Name: attrs.GetName()}, csr); err != nil {
		return auth.DecisionDeny, "", err
	}

	if csr.Spec.Username != attrs.GetUser().GetName() {
		log.Info("Denying authorization because the CSR is for a different user", "csrUsername", csr.Spec.Username)
		return auth.DecisionDeny, fmt.Sprintf("gardener-node-agent is only allowed to read or request CSRs for its own user %q", attrs.GetUser().GetName()), nil
	}

	return auth.DecisionAllow, "", nil
}

func (a *authorizer) authorizeEvent(log logr.Logger, attrs auth.Attributes) (auth.Decision, string, error) {
	if ok, reason := a.checkSubresource(log, attrs); !ok {
		return auth.DecisionDeny, reason, nil
	}

	if allowed, reason := a.checkVerb(log, attrs, "create", "patch"); !allowed {
		return auth.DecisionDeny, reason, nil
	}

	return auth.DecisionAllow, "", nil
}

func (a *authorizer) authorizeLease(ctx context.Context, log logr.Logger, machineName string, attrs auth.Attributes) (auth.Decision, string, error) {
	if ok, reason := a.checkSubresource(log, attrs); !ok {
		return auth.DecisionDeny, reason, nil
	}

	allowedVerbs := []string{"get", "list", "watch", "create", "update"}
	if allowed, reason := a.checkVerb(log, attrs, allowedVerbs...); !allowed {
		return auth.DecisionDeny, reason, nil
	}

	machine := &machinev1alpha1.Machine{}
	if err := a.sourceClient.Get(ctx, client.ObjectKey{Name: machineName, Namespace: a.machineNamespace}, machine); err != nil {
		return auth.DecisionDeny, "", fmt.Errorf("error getting machine %q: %w", machineName, err)
	}

	node := machine.Labels[machinev1alpha1.NodeLabelKey]
	if node == "" {
		log.Info(`Denying request because the machine does not have a "node" label`, "machineName", machineName)
		return auth.DecisionDeny, fmt.Sprintf(`expecting "node" label on machine %q`, machineName), nil
	}

	allowedLease := "gardener-node-agent-" + node
	if (attrs.GetVerb() != "create" && attrs.GetName() != allowedLease) || attrs.GetNamespace() != metav1.NamespaceSystem {
		log.Info("Denying authorization because gardener-node-agent is not allowed to access the lease", "nodeName", node, "machineName", machineName, "leaseName", attrs.GetName())
		return auth.DecisionDeny, fmt.Sprintf("this gardener-node-agent can only access lease %q in %q namespace", allowedLease, metav1.NamespaceSystem), nil
	}

	return auth.DecisionAllow, "", nil
}

func (a *authorizer) authorizeNode(ctx context.Context, log logr.Logger, machineName string, attrs auth.Attributes) (auth.Decision, string, error) {
	if ok, reason := a.checkSubresource(log, attrs, "status"); !ok {
		return auth.DecisionDeny, reason, nil
	}

	allowedVerbs := []string{"get", "list", "watch", "patch", "update"}
	if allowed, reason := a.checkVerb(log, attrs, allowedVerbs...); !allowed {
		return auth.DecisionDeny, reason, nil
	}

	// Listing nodes must be allowed unconditionally, because gardener-node-agent only knows its hostname, but not its node name.
	// Kubelet creates the latter when gardener-node-agent is already running.
	if attrs.GetVerb() == "list" || attrs.GetVerb() == "watch" {
		return auth.DecisionAllow, "", nil
	}

	machine := &machinev1alpha1.Machine{}
	if err := a.sourceClient.Get(ctx, client.ObjectKey{Name: machineName, Namespace: a.machineNamespace}, machine); err != nil {
		return auth.DecisionDeny, "", err
	}

	if machine.Labels[machinev1alpha1.NodeLabelKey] != attrs.GetName() {
		log.Info("Denying request because node belongs to a different machine", "nodeName", attrs.GetName(), "machineName", machineName)
		return auth.DecisionDeny, fmt.Sprintf("node %q does not belong to machine %q", attrs.GetName(), machineName), nil
	}

	return auth.DecisionAllow, "", nil
}

func (a *authorizer) authorizePod(ctx context.Context, log logr.Logger, machineName string, attrs auth.Attributes) (auth.Decision, string, error) {
	if ok, reason := a.checkSubresource(log, attrs); !ok {
		return auth.DecisionDeny, reason, nil
	}

	allowedVerbs := []string{"get", "list", "watch", "delete"}
	if allowed, reason := a.checkVerb(log, attrs, allowedVerbs...); !allowed {
		return auth.DecisionDeny, reason, nil
	}

	machine := &machinev1alpha1.Machine{}
	if err := a.sourceClient.Get(ctx, client.ObjectKey{Name: machineName, Namespace: a.machineNamespace}, machine); err != nil {
		return auth.DecisionDeny, "", fmt.Errorf("error getting machine %q: %w", machineName, err)
	}

	node := machine.Labels[machinev1alpha1.NodeLabelKey]
	if node == "" {
		log.Info(`Denying request because the machine does not have a "node" label`, "machineName", machineName)
		return auth.DecisionDeny, fmt.Sprintf(`expecting "node" label on machine %q`, machineName), nil
	}

	switch attrs.GetVerb() {
	case "list", "watch":
		if !a.authorizeWithSelectors {
			return auth.DecisionAllow, "", nil
		}

		// allow a scoped fieldSelector
		reqs, err := attrs.GetFieldSelector()
		if err != nil {
			log.Info("Denying request because field selector is invalid", "error", err)
			return auth.DecisionDeny, "", fmt.Errorf("error parsing field selector: %w", err)
		}

		for _, req := range reqs {
			if req.Field == "spec.nodeName" && req.Operator == selection.Equals && req.Value == node {
				return auth.DecisionAllow, "", nil
			}
		}

		// allow a read of a single pod known to be related to the node
		if attrs.GetName() != "" {
			return a.authorizeSinglePod(ctx, log, node, attrs)
		}

		log.Info("Denying request because only listing/watching pods with spec.nodeName field selector for the same node is allowed")
		return auth.DecisionDeny, fmt.Sprintf("can only list/watch pods with spec.nodeName=%s field selector", node), nil

	case "get", "delete":
		return a.authorizeSinglePod(ctx, log, node, attrs)
	}

	return auth.DecisionAllow, "", nil
}

func (a *authorizer) authorizeSinglePod(ctx context.Context, log logr.Logger, nodeName string, attrs auth.Attributes) (auth.Decision, string, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      attrs.GetName(),
			Namespace: attrs.GetNamespace(),
		},
	}
	if err := a.targetClient.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil {
		return auth.DecisionDeny, "", fmt.Errorf("error getting pod %q: %w", client.ObjectKeyFromObject(pod), err)
	}

	if pod.Spec.NodeName != nodeName {
		log.Info("Denying request because pod belongs to a different node", "podNodeName", pod.Spec.NodeName)
		return auth.DecisionDeny, fmt.Sprintf("pod %q does not belong to node %q", client.ObjectKeyFromObject(pod), nodeName), nil
	}

	return auth.DecisionAllow, "", nil
}

func (a *authorizer) authorizeSecret(ctx context.Context, log logr.Logger, machineName string, attrs auth.Attributes) (auth.Decision, string, error) {
	if ok, reason := a.checkSubresource(log, attrs); !ok {
		return auth.DecisionDeny, reason, nil
	}

	allowedVerbs := []string{"get", "list", "watch"}
	if allowed, reason := a.checkVerb(log, attrs, allowedVerbs...); !allowed {
		return auth.DecisionDeny, reason, nil
	}

	machine := &machinev1alpha1.Machine{}
	if err := a.sourceClient.Get(ctx, client.ObjectKey{Name: machineName, Namespace: a.machineNamespace}, machine); err != nil {
		return auth.DecisionDeny, "", fmt.Errorf("error getting machine %q: %w", machineName, err)
	}

	validSecrets := []string{machine.Spec.NodeTemplateSpec.Labels[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName], valitailTokenSecretName}

	if !slices.Contains(validSecrets, attrs.GetName()) || attrs.GetNamespace() != metav1.NamespaceSystem {
		log.Info("Denying authorization because gardener-node-agent is not allowed to access secret", "secret", attrs.GetName(), "machine", machineName)
		return auth.DecisionDeny, fmt.Sprintf("gardener-node-agent can only access secrets %v in %q namespace", validSecrets, metav1.NamespaceSystem), nil
	}

	return auth.DecisionAllow, "", nil
}

func (a *authorizer) checkVerb(log logr.Logger, attrs auth.Attributes, allowedVerbs ...string) (bool, string) {
	if !slices.Contains(allowedVerbs, attrs.GetVerb()) {
		log.Info("Denying authorization because verb is not allowed for this resource type", "allowedVerbs", allowedVerbs)
		return false, fmt.Sprintf("only the following verbs are allowed for this resource type: %+v", allowedVerbs)
	}

	return true, ""
}

func (a *authorizer) checkSubresource(log logr.Logger, attrs auth.Attributes, allowedSubresources ...string) (bool, string) {
	if subresource := attrs.GetSubresource(); len(subresource) > 0 && !slices.Contains(allowedSubresources, subresource) {
		log.Info("Denying authorization because subresource is not allowed for this resource", "allowedSubresources", allowedSubresources)
		return false, fmt.Sprintf("only the following subresources are allowed for this resource type: %+v", allowedSubresources)
	}

	return true, ""
}
