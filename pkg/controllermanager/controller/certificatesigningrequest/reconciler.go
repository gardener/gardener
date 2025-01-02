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

Modifications Copyright 2024 SAP SE or an SAP affiliate company and Gardener contributors
*/

package certificatesigningrequest

import (
	"context"
	"fmt"

	authorizationv1 "k8s.io/api/authorization/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	certificatesclientv1 "k8s.io/client-go/kubernetes/typed/certificates/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
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

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

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

	if ok, reason := gardenerutils.IsSeedClientCert(x509cr, csr.Spec.Usages); !ok {
		log.Info("Ignoring CSR, as it does not match the requirements for a seed client", "reason", reason)
		return reconcile.Result{}, nil
	}

	log.Info("Checking if creating user has authorization for seedclient subresource", "username", csr.Spec.Username, "groups", csr.Spec.Groups, "extra", extra)
	sarStatus, err := authorize(ctx, r.Client, csr.Spec.Username, csr.Spec.UID, csr.Spec.Groups, extra, authorizationv1.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "seedclient"})
	if err != nil {
		return reconcile.Result{}, err
	}

	if sarStatus.Allowed {
		log.Info("Auto-approving CSR")
		csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
			Type:    certificatesv1.CertificateApproved,
			Reason:  "AutoApproved",
			Message: "Auto approving gardenlet client certificate after SubjectAccessReview.",
			Status:  corev1.ConditionTrue,
		})
		_, err := r.CertificatesClient.UpdateApproval(ctx, csr.Name, csr, kubernetes.DefaultUpdateOptions())
		if err == nil {
			log.Info("Update successful", "csr", csr)
		}
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, fmt.Errorf("recognized CSR but SubjectAccessReview was not allowed: %s", sarStatus.Reason)
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
