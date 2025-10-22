/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

This file was copied and modified from the kubernetes/kubernetes project
https://github.com/kubernetes/kubernetes/blob/release-1.15/pkg/controller/certificates/approver/sarapprove.go

Modifications Copyright SAP SE or an SAP affiliate company and Gardener contributors
*/

package certificatesigningrequest

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	authorizationv1 "k8s.io/api/authorization/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	certificatesclientv1 "k8s.io/client-go/kubernetes/typed/certificates/v1"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	bootstraptokenutil "k8s.io/cluster-bootstrap/token/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	gardenletutils "github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
)

// Reconciler reconciles CertificateSigningRequest.
type Reconciler struct {
	Client             client.Client
	CertificatesClient certificatesclientv1.CertificateSigningRequestInterface
	Config             controllermanagerconfigv1alpha1.CertificateSigningRequestControllerConfiguration
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	var (
		log = logf.FromContext(ctx)

		isInFinalState bool
		finalState     string
	)

	csr := &certificatesv1.CertificateSigningRequest{}
	if err := r.Client.Get(ctx, request.NamespacedName, csr); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	for _, c := range csr.Status.Conditions {
		if c.Type == certificatesv1.CertificateApproved || c.Type == certificatesv1.CertificateDenied {
			isInFinalState = true
			finalState = string(c.Type)
		}
	}

	extra := make(map[string]authorizationv1.ExtraValue, len(csr.Spec.Extra))
	for k, v := range csr.Spec.Extra {
		extra[k] = authorizationv1.ExtraValue(v)
	}

	if len(csr.Status.Certificate) != 0 || isInFinalState {
		log.Info("Ignoring CSR, as it is in final state", "finalState", finalState)
		return reconcile.Result{}, nil
	}

	x509cr, err := utils.DecodeCertificateRequest(csr.Spec.Request)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to parse csr: %w", err)
	}

	var (
		isSeedClient, reasonSeed           = gardenerutils.IsSeedClientCert(x509cr, csr.Spec.Usages)
		isShootClient, reasonShoot         = gardenerutils.IsShootClientCert(x509cr, csr.Spec.Usages)
		isGardenadmClient, reasonGardenadm = gardenerutils.IsGardenadmClientCert(x509cr, csr.Spec.Usages)

		subResource string
	)

	switch {
	case isSeedClient:
		subResource = "seedclient"

	case isShootClient, isGardenadmClient:
		subResource = "shootclient"
		if ok, reason, err := r.isBootstrapTokenForThisCSR(ctx, csr); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed checking bootstrap token description: %w", err)
		} else if !ok {
			return reconcile.Result{}, r.denyCSR(ctx, log, csr, fmt.Sprintf("Bootstrap token does not fulfill requirements for auto-approval: %s", reason))
		}

	default:
		log.Info("Ignoring CSR, as it does not match the requirements for a seed client or a self-hosted shoot client", "reasonSeedCheck", reasonSeed, "reasonShootCheck", reasonShoot, "reasonGardenadmCheck", reasonGardenadm)
		return reconcile.Result{}, nil
	}

	log.Info("Checking if creating user has authorization for subresource", "username", csr.Spec.Username, "groups", csr.Spec.Groups, "extra", extra, "subresource", subResource)
	sarStatus, err := authorize(ctx, r.Client, csr.Spec.Username, csr.Spec.UID, csr.Spec.Groups, extra, authorizationv1.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: subResource})
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed checking subresource authorization: %w", err)
	}

	if !sarStatus.Allowed {
		return reconcile.Result{}, fmt.Errorf("recognized CSR but SubjectAccessReview was not allowed: %s", sarStatus.Reason)
	}

	log.Info("Auto-approving CSR")
	return reconcile.Result{}, r.approveCSR(ctx, log, csr)
}

// isBootstrapTokenForThisCSR checks if the CSR was requested via a bootstrap token. If yes, it extracts the
// shoot metadata from the bootstrap token secret's description (namespace and name of the shoot). The namespace and
// name must be used in the CSR's subject as organization and common name, respectively, to ensure that the bootstrap
// token was created for exactly this shoot.
func (r *Reconciler) isBootstrapTokenForThisCSR(ctx context.Context, csr *certificatesv1.CertificateSigningRequest) (bool, string, error) {
	if !strings.HasPrefix(csr.Spec.Username, bootstraptokenapi.BootstrapUserPrefix) || !slices.Contains(csr.Spec.Groups, bootstraptokenapi.BootstrapDefaultGroup) {
		return false, "CSR does not seem to be requested via a bootstrap token", nil
	}

	shootMeta, err := gardenletutils.ShootMetaFromBootstrapToken(ctx, r.Client, bootstraptokenutil.BootstrapTokenSecretName(strings.TrimPrefix(csr.Spec.Username, bootstraptokenapi.BootstrapUserPrefix)))
	if err != nil {
		// Intentionally, we don't return the err as error here, but rather as reason. This will lead to denial of the
		// CSR if we cannot extract the shoot metadata from the bootstrap token secret.
		//nolint:nilerr
		return false, err.Error(), nil
	}

	return ensureCSRSubjectMatchesBootstrapTokenDescription(shootMeta, csr.Spec.Request)
}

func ensureCSRSubjectMatchesBootstrapTokenDescription(shootMeta types.NamespacedName, rawCSR []byte) (bool, string, error) {
	x509cr, err := utils.DecodeCertificateRequest(rawCSR)
	if err != nil {
		return false, "", fmt.Errorf("failed decoding certificate signing request: %w", err)
	}

	var usernameMatchesFormat bool

	for _, prefix := range []string{v1beta1constants.ShootUserNamePrefix, v1beta1constants.GardenadmUserNamePrefix} {
		if username := prefix + shootMeta.Namespace + ":" + shootMeta.Name; x509cr.Subject.CommonName == username {
			usernameMatchesFormat = true
			break
		}
	}

	if !usernameMatchesFormat {
		return false, fmt.Sprintf("the common name in the certificate request (%s) does not match the expected format 'gardener.cloud:{system,gardenadm}:shoot:<namespace>:<name>'", x509cr.Subject.CommonName), nil
	}

	return true, "all requirements met", nil
}

func (r *Reconciler) approveCSR(ctx context.Context, log logr.Logger, csr *certificatesv1.CertificateSigningRequest) error {
	csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
		Type:    certificatesv1.CertificateApproved,
		Reason:  "AutoApproved",
		Message: "Auto approving gardenlet client certificate after SubjectAccessReview.",
		Status:  corev1.ConditionTrue,
	})

	if _, err := r.CertificatesClient.UpdateApproval(ctx, csr.Name, csr, kubernetes.DefaultUpdateOptions()); err != nil {
		return fmt.Errorf("failed approving CertificateSigningRequest: %w", err)
	}

	log.Info("Approval successful")
	return nil
}

func (r *Reconciler) denyCSR(ctx context.Context, log logr.Logger, csr *certificatesv1.CertificateSigningRequest, message string) error {
	csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
		Type:    certificatesv1.CertificateDenied,
		Reason:  "RequestDenied",
		Message: fmt.Sprintf("Denying gardenlet client certificate (%s)", message),
		Status:  corev1.ConditionTrue,
	})

	if _, err := r.CertificatesClient.UpdateApproval(ctx, csr.Name, csr, kubernetes.DefaultUpdateOptions()); err != nil {
		return fmt.Errorf("failed denying CertificateSigningRequest: %w", err)
	}

	log.Info("Denial successful")
	return nil
}

func authorize(ctx context.Context, c client.Client, username, uid string, groups []string, extra map[string]authorizationv1.ExtraValue, resourceAttributes authorizationv1.ResourceAttributes) (authorizationv1.SubjectAccessReviewStatus, error) {
	sar := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User:               username,
			UID:                uid,
			Groups:             groups,
			Extra:              extra,
			ResourceAttributes: &resourceAttributes,
		},
	}

	return sar.Status, c.Create(ctx, sar)
}
