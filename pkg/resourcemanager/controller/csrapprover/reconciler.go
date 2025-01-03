// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package csrapprover

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"
	certificatesclientv1 "k8s.io/client-go/kubernetes/typed/certificates/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

type decision string

const (
	csrApproved  decision = "csrApproved"
	csrDenied    decision = "csrDenied"
	csrNoOpinion decision = "csrNoOpinion"
)

// Reconciler reconciles CertificateSigningRequest objects.
type Reconciler struct {
	SourceClient       client.Client
	TargetClient       client.Client
	CertificatesClient certificatesclientv1.CertificateSigningRequestInterface
	Config             resourcemanagerconfigv1alpha1.CSRApproverControllerConfig
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	csr := &certificatesv1.CertificateSigningRequest{}
	if err := r.TargetClient.Get(ctx, request.NamespacedName, csr); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	var (
		isInFinalState bool
		finalState     string
	)

	for _, c := range csr.Status.Conditions {
		if c.Type == certificatesv1.CertificateApproved || c.Type == certificatesv1.CertificateDenied {
			isInFinalState = true
			finalState = string(c.Type)
		}
	}

	if len(csr.Status.Certificate) != 0 || isInFinalState {
		log.Info("Ignoring CSR, as it is in final state", "finalState", finalState)
		return reconcile.Result{}, nil
	}

	x509cr, err := utils.DecodeCertificateRequest(csr.Spec.Request)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to parse csr: %w", err)
	}

	switch csr.Spec.SignerName {
	case certificatesv1.KubeletServingSignerName:
		err = r.handleKubeletServing(ctx, csr, x509cr)
	case certificatesv1.KubeAPIServerClientSignerName:
		err = r.handleKubeAPIServerClient(ctx, csr, x509cr)
	default:
		log.Info("Unknown signerName", "signerName", csr.Spec.SignerName)
	}

	return reconcile.Result{}, err
}

func (r *Reconciler) handleKubeletServing(ctx context.Context, csr *certificatesv1.CertificateSigningRequest, x509cr *x509.CertificateRequest) error {
	log := logf.FromContext(ctx)

	reason, allowed, err := r.mustApproveKubeletServing(ctx, csr, x509cr)
	if err != nil {
		return fmt.Errorf("failed when checking for approval conditions: %w", err)
	}

	if allowed {
		log.Info("Auto-approving CSR", "reason", reason)
		csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
			Type:    certificatesv1.CertificateApproved,
			Status:  corev1.ConditionTrue,
			Reason:  "RequestApproved",
			Message: fmt.Sprintf("Approving kubelet server certificate CSR (%s)", reason),
		})
	} else {
		log.Info("Denying CSR", "reason", reason)
		csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
			Type:    certificatesv1.CertificateDenied,
			Status:  corev1.ConditionTrue,
			Reason:  "RequestDenied",
			Message: fmt.Sprintf("Denying kubelet server certificate CSR (%s)", reason),
		})
	}

	_, err = r.CertificatesClient.UpdateApproval(ctx, csr.Name, csr, kubernetes.DefaultUpdateOptions())
	return err
}

func (r *Reconciler) handleKubeAPIServerClient(ctx context.Context, csr *certificatesv1.CertificateSigningRequest, x509cr *x509.CertificateRequest) error {
	log := logf.FromContext(ctx)

	reason, decision, err := r.mustApproveKubeAPIServerClient(ctx, csr, x509cr)
	if err != nil {
		return fmt.Errorf("failed when checking for approval conditions: %w", err)
	}

	switch decision {
	case csrApproved:
		log.Info("Auto-approving CSR", "reason", reason)
		csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
			Type:    certificatesv1.CertificateApproved,
			Status:  corev1.ConditionTrue,
			Reason:  "RequestApproved",
			Message: fmt.Sprintf("Approving gardener-node-agent certificate CSR (%s)", reason),
		})
	case csrDenied:
		log.Info("Denying CSR", "reason", reason)
		csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
			Type:    certificatesv1.CertificateDenied,
			Status:  corev1.ConditionTrue,
			Reason:  "RequestDenied",
			Message: fmt.Sprintf("Denying gardener-node-agent certificate CSR (%s)", reason),
		})
	default:
		log.V(1).Info("Not a CSR for gardener-node-agent", "reason", reason)
		return nil
	}

	_, err = r.CertificatesClient.UpdateApproval(ctx, csr.Name, csr, kubernetes.DefaultUpdateOptions())
	return err
}

func (r *Reconciler) mustApproveKubeletServing(ctx context.Context, csr *certificatesv1.CertificateSigningRequest, x509cr *x509.CertificateRequest) (string, bool, error) {
	if prefix := "system:node:"; !strings.HasPrefix(csr.Spec.Username, prefix) {
		return fmt.Sprintf("username %q is not prefixed with %q", csr.Spec.Username, prefix), false, nil
	}

	if len(x509cr.DNSNames)+len(x509cr.IPAddresses) == 0 {
		return "no DNS names or IP addresses in the SANs found", false, nil
	}

	if x509cr.Subject.CommonName != csr.Spec.Username {
		return "common name in CSR does not match username", false, nil
	}

	if len(x509cr.Subject.Organization) != 1 || !slices.Contains(x509cr.Subject.Organization, user.NodesGroup) {
		return "organization in CSR does not match nodes group", false, nil
	}

	nodeName := strings.TrimPrefix(x509cr.Subject.CommonName, "system:node:")

	node := &corev1.Node{}
	if err := r.TargetClient.Get(ctx, client.ObjectKey{Name: nodeName}, node); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Sprintf("could not find node object with name %q", node.Name), false, nil
		}
		return "", false, fmt.Errorf("error getting node object with name %q: %w", nodeName, err)
	}

	machineList := &machinev1alpha1.MachineList{}
	if err := r.SourceClient.List(ctx, machineList, client.InNamespace(r.Config.MachineNamespace), client.MatchingLabels{"node": node.Name}); err != nil {
		return "", false, fmt.Errorf("failed to list machine objects: %w", err)
	}

	if length := len(machineList.Items); length != 1 {
		return fmt.Sprintf("Expected exactly one machine in namespace %q for node %q but found %d", r.Config.MachineNamespace, node.Name, length), false, nil
	}

	var (
		hostNames   []string
		ipAddresses []netip.Addr
	)

	for _, address := range node.Status.Addresses {
		switch address.Type {
		case corev1.NodeHostName, corev1.NodeInternalDNS, corev1.NodeExternalDNS:
			hostNames = append(hostNames, address.Address)
		case corev1.NodeInternalIP, corev1.NodeExternalIP:
			comparableIP, err := netip.ParseAddr(address.Address)
			if err != nil {
				return fmt.Sprintf("IP address %q in node.Status.Addresses is invalid: %v", address.Address, err), false, nil //nolint:nilerr
			}
			ipAddresses = append(ipAddresses, comparableIP)
		}
	}

	if !sets.New(hostNames...).Equal(sets.New(x509cr.DNSNames...)) {
		return "DNS names in CSR do not match addresses of type 'Hostname' or 'InternalDNS' or 'ExternalDNS' in node object", false, nil
	}

	var ipAddressesInCSR []netip.Addr
	for _, ip := range x509cr.IPAddresses {
		if comparableIP, ok := netip.AddrFromSlice(ip); ok {
			ipAddressesInCSR = append(ipAddressesInCSR, comparableIP)
		}
	}

	if !sets.New(ipAddresses...).Equal(sets.New(ipAddressesInCSR...)) {
		return "IP addresses in CSR do not match addresses of type 'InternalIP' or 'ExternalIP' in node object", false, nil
	}

	return "all checks passed", true, nil
}

func (r *Reconciler) mustApproveKubeAPIServerClient(ctx context.Context, csr *certificatesv1.CertificateSigningRequest, x509cr *x509.CertificateRequest) (string, decision, error) {
	// Handle CSRs for gardener-node-agent only. There are other "kube-apiserver-client" CSRs which must not be touched.
	if !strings.HasPrefix(x509cr.Subject.CommonName, v1beta1constants.NodeAgentUserNamePrefix) {
		return fmt.Sprintf("commonName %q is not prefixed with %q", x509cr.Subject.CommonName, v1beta1constants.NodeAgentUserNamePrefix), csrNoOpinion, nil
	}

	machineName := strings.TrimPrefix(x509cr.Subject.CommonName, v1beta1constants.NodeAgentUserNamePrefix)
	machine := &machinev1alpha1.Machine{}
	if err := r.SourceClient.Get(ctx, client.ObjectKey{Namespace: r.Config.MachineNamespace, Name: machineName}, machine); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Sprintf("machine %q does not exist", machineName), csrDenied, nil
		}
		return "", csrNoOpinion, fmt.Errorf("error getting machine object with name %q: %w", machineName, err)
	}

	switch {
	case strings.HasPrefix(csr.Spec.Username, "system:bootstrap:"):
		if nodeName, found := machine.Labels[machinev1alpha1.NodeLabelKey]; found {
			if err := r.TargetClient.Get(ctx, client.ObjectKey{Name: nodeName}, &corev1.Node{}); err == nil {
				return fmt.Sprintf("Cannot use bootstrap token since gardener-node-agent for machine %q is already bootstrapped", machineName), csrDenied, nil
			} else if !apierrors.IsNotFound(err) {
				return "", csrNoOpinion, fmt.Errorf("error getting node object with name %q: %w", nodeName, err)
			}
		}
	case strings.HasPrefix(csr.Spec.Username, v1beta1constants.NodeAgentUserNamePrefix):
		if csr.Spec.Username != x509cr.Subject.CommonName {
			return fmt.Sprintf("username %q and commonName %q do not match", csr.Spec.Username, x509cr.Subject.CommonName), csrDenied, nil
		}
	// TODO(oliver-goetz): remove this case when NodeAgentAuthorizer feature gate is removed. It is used for migrating existing gardener-node-agents
	case csr.Spec.Username == "system:serviceaccount:kube-system:gardener-node-agent":
		if _, found := machine.Labels[machinev1alpha1.NodeLabelKey]; !found {
			return "gardener-node-agent service account is allowed to create CSRs for machines with existing nodes only", csrDenied, nil
		}
	default:
		return fmt.Sprintf("username %q is not allowed to create CSRs for a gardener-node-agent", csr.Spec.Username), csrDenied, nil
	}

	return "all checks passed", csrApproved, nil
}
