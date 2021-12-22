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

Modifications Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved.
*/

package certificatesigningrequest

import (
	"context"
	"fmt"

	authorizationv1 "k8s.io/api/authorization/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func (c *Controller) csrAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.log.Error(err, "Couldn't get key for object", "object", obj)
		return
	}
	c.csrQueue.Add(key)
}

func (c *Controller) csrUpdate(_, newObj interface{}) {
	c.csrAdd(newObj)
}

// NewCSRReconciler creates a new instance of a reconciler which reconciles CSRs.
func NewCSRReconciler(gardenClient kubernetes.Interface, certificatesAPIVersion string) reconcile.Reconciler {
	return &csrReconciler{
		gardenClient:           gardenClient,
		certificatesAPIVersion: certificatesAPIVersion,
	}
}

type csrReconciler struct {
	gardenClient           kubernetes.Interface
	certificatesAPIVersion string
}

func (r *csrReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	var (
		log = logf.FromContext(ctx)

		cert               []byte
		isInFinalState     bool
		finalState         string
		req                []byte
		usages             []certificatesv1.KeyUsage
		extra              = make(map[string]authorizationv1.ExtraValue)
		username           string
		uid                string
		groups             []string
		updateConditionsFn func() error
	)

	switch r.certificatesAPIVersion {
	case "v1":
		csrV1 := &certificatesv1.CertificateSigningRequest{}
		if err := r.gardenClient.Client().Get(ctx, request.NamespacedName, csrV1); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Object is gone, stop reconciling")
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
		}

		for _, c := range csrV1.Status.Conditions {
			if c.Type == certificatesv1.CertificateApproved || c.Type == certificatesv1.CertificateDenied {
				isInFinalState = true
				finalState = string(c.Type)
			}
		}
		for k, v := range csrV1.Spec.Extra {
			extra[k] = authorizationv1.ExtraValue(v)
		}
		cert = csrV1.Status.Certificate
		req = csrV1.Spec.Request
		usages = csrV1.Spec.Usages
		username = csrV1.Spec.Username
		uid = csrV1.Spec.UID
		groups = csrV1.Spec.Groups
		updateConditionsFn = func() error {
			csrV1.Status.Conditions = append(csrV1.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
				Type:    certificatesv1.CertificateApproved,
				Reason:  "AutoApproved",
				Message: "Auto approving gardenlet client certificate after SubjectAccessReview.",
				Status:  v1.ConditionTrue,
			})
			_, err := r.gardenClient.Kubernetes().CertificatesV1().CertificateSigningRequests().UpdateApproval(ctx, csrV1.Name, csrV1, kubernetes.DefaultUpdateOptions())
			return err
		}

	case "v1beta1":
		csrV1beta1 := &certificatesv1beta1.CertificateSigningRequest{}
		if err := r.gardenClient.Client().Get(ctx, request.NamespacedName, csrV1beta1); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Object is gone, stop reconciling")
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
		}

		for _, c := range csrV1beta1.Status.Conditions {
			if c.Type == certificatesv1beta1.CertificateApproved || c.Type == certificatesv1beta1.CertificateDenied {
				isInFinalState = true
				finalState = string(c.Type)
			}
		}
		for k, v := range csrV1beta1.Spec.Extra {
			extra[k] = authorizationv1.ExtraValue(v)
		}
		cert = csrV1beta1.Status.Certificate
		req = csrV1beta1.Spec.Request
		usages = kutil.CertificatesV1beta1UsagesToCertificatesV1Usages(csrV1beta1.Spec.Usages)
		username = csrV1beta1.Spec.Username
		uid = csrV1beta1.Spec.UID
		groups = csrV1beta1.Spec.Groups
		updateConditionsFn = func() error {
			csrV1beta1.Status.Conditions = append(csrV1beta1.Status.Conditions, certificatesv1beta1.CertificateSigningRequestCondition{
				Type:    certificatesv1beta1.CertificateApproved,
				Reason:  "AutoApproved",
				Message: "Auto approving gardenlet client certificate after SubjectAccessReview.",
			})
			_, err := r.gardenClient.Kubernetes().CertificatesV1beta1().CertificateSigningRequests().UpdateApproval(ctx, csrV1beta1, kubernetes.DefaultUpdateOptions())
			return err
		}
	}

	if len(cert) != 0 || isInFinalState {
		log.Info("Ignoring CSR, as it is in final state", "finalState", finalState)
		return reconcile.Result{}, nil
	}

	x509cr, err := utils.DecodeCertificateRequest(req)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to parse csr: %w", err)
	}

	if ok, reason := gutil.IsSeedClientCert(x509cr, usages); !ok {
		log.Info("Ignoring CSR, as it does not match the requirements for a seed client", "reason", reason)
		return reconcile.Result{}, nil
	}

	log.Info("Checking if creating user has authorization for seedclient subresource", "username", username, "groups", groups, "extra", extra)
	sarStatus, err := authorize(ctx, r.gardenClient.Client(), username, uid, groups, extra, authorizationv1.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "seedclient"})
	if err != nil {
		return reconcile.Result{}, err
	}

	if sarStatus.Allowed {
		log.Info("Auto-approving CSR")
		return reconcile.Result{}, updateConditionsFn()
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
