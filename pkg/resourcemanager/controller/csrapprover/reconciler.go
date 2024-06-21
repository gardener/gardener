// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package csrapprover

import (
	"context"
	"crypto/x509"
	"fmt"
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

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/resourcemanager/apis/config"
	"github.com/gardener/gardener/pkg/utils"
)

// Reconciler reconciles CertificateSigningRequest objects.
type Reconciler struct {
	SourceClient       client.Client
	TargetClient       client.Client
	CertificatesClient certificatesclientv1.CertificateSigningRequestInterface
	Config             config.CSRApproverControllerConfig
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

	reason, allowed, err := r.mustApprove(ctx, csr, x509cr)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed when checking for approval conditions: %w", err)
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
	return reconcile.Result{}, err
}

func (r *Reconciler) mustApprove(ctx context.Context, csr *certificatesv1.CertificateSigningRequest, x509cr *x509.CertificateRequest) (string, bool, error) {
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
		return "", false, err
	}

	machineList := &machinev1alpha1.MachineList{}
	if err := r.SourceClient.List(ctx, machineList, client.InNamespace(r.Config.MachineNamespace), client.MatchingLabels{"node": node.Name}); err != nil {
		return "", false, err
	}

	if length := len(machineList.Items); length != 1 {
		return fmt.Sprintf("Expected exactly one machine in namespace %q for node %q but found %d", r.Config.MachineNamespace, node.Name, length), false, nil
	}

	var (
		hostNames   []string
		ipAddresses []string
	)

	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeHostName || address.Type == corev1.NodeInternalDNS || address.Type == corev1.NodeExternalDNS {
			hostNames = append(hostNames, address.Address)
		}
		if address.Type == corev1.NodeInternalIP || address.Type == corev1.NodeExternalIP {
			ipAddresses = append(ipAddresses, address.Address)
		}
	}

	if !sets.New(hostNames...).Equal(sets.New(x509cr.DNSNames...)) {
		return "DNS names in CSR do not match addresses of type 'Hostname' or 'InternalDNS' or 'ExternalDNS' in node object", false, nil
	}

	var ipAddressesInCSR []string
	for _, ip := range x509cr.IPAddresses {
		ipAddressesInCSR = append(ipAddressesInCSR, ip.String())
	}

	if !sets.New(ipAddresses...).Equal(sets.New(ipAddressesInCSR...)) {
		return "IP addresses in CSR do not match addresses of type 'InternalIP' or 'ExternalIP' in node object", false, nil
	}

	return "all checks passed", true, nil
}
