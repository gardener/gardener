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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"reflect"
	"strings"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/sirupsen/logrus"
	authorizationv1beta1 "k8s.io/api/authorization/v1beta1"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (c *Controller) csrAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.csrQueue.Add(key)
}

func (c *Controller) csrUpdate(_, newObj interface{}) {
	c.csrAdd(newObj)
}

// NewCSRReconciler creates a new instance of a reconciler which reconciles CSRs.
func NewCSRReconciler(l logrus.FieldLogger, clientMap clientmap.ClientMap) reconcile.Reconciler {
	return &csrReconciler{
		logger:    l,
		clientMap: clientMap,
	}
}

type csrReconciler struct {
	logger    logrus.FieldLogger
	clientMap clientmap.ClientMap
}

func (r *csrReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	gardenClient, err := r.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get garden client: %w", err)
	}

	csr := &certificatesv1beta1.CertificateSigningRequest{}
	if err := gardenClient.Client().Get(ctx, request.NamespacedName, csr); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Infof("Object %q is gone, stop reconciling: %v", request.Name, err)
			return reconcile.Result{}, nil
		}
		r.logger.Infof("Unable to retrieve object %q from store: %v", request.Name, err)
		return reconcile.Result{}, err
	}

	csrLogger := logger.NewFieldLogger(logger.Logger, "csr", csr.Name)

	if len(csr.Status.Certificate) != 0 {
		return reconcile.Result{}, nil
	}

	for _, c := range csr.Status.Conditions {
		if c.Type == certificatesv1beta1.CertificateApproved {
			return reconcile.Result{}, nil
		}
		if c.Type == certificatesv1beta1.CertificateDenied {
			return reconcile.Result{}, nil
		}
	}

	x509cr, err := parseCSR(csr)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to parse csr %q: %v", csr.Name, err)
	}

	if !isSeedClientCert(csr, x509cr) {
		return reconcile.Result{}, nil
	}

	csrLogger.Infof("[CSR APPROVER] %s", csr.Name)

	approved, err := authorize(ctx, gardenClient.Client(), csr, authorizationv1beta1.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "seedclient"})
	if err != nil {
		return reconcile.Result{}, err
	}

	if approved {
		csrLogger.Infof("[CSR APPROVER] Auto-approving %s", csr.Name)

		csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1beta1.CertificateSigningRequestCondition{
			Type:    certificatesv1beta1.CertificateApproved,
			Reason:  "AutoApproved",
			Message: "Auto approving gardenlet client certificate after SubjectAccessReview.",
		})
		_, err := gardenClient.Kubernetes().CertificatesV1beta1().CertificateSigningRequests().UpdateApproval(ctx, csr, kubernetes.DefaultUpdateOptions())
		return reconcile.Result{}, err
	}

	message := fmt.Sprintf("recognized csr %q but subject access review was not approved", csr.Name)
	csrLogger.Errorf("[CSR APPROVER] %s", message)
	return reconcile.Result{}, fmt.Errorf(message)
}

func authorize(ctx context.Context, c client.Client, csr *certificatesv1beta1.CertificateSigningRequest, resourceAttributes authorizationv1beta1.ResourceAttributes) (bool, error) {
	extra := make(map[string]authorizationv1beta1.ExtraValue)
	for k, v := range csr.Spec.Extra {
		extra[k] = authorizationv1beta1.ExtraValue(v)
	}

	sar := &authorizationv1beta1.SubjectAccessReview{
		Spec: authorizationv1beta1.SubjectAccessReviewSpec{
			User:               csr.Spec.Username,
			UID:                csr.Spec.UID,
			Groups:             csr.Spec.Groups,
			Extra:              extra,
			ResourceAttributes: &resourceAttributes,
		},
	}

	if err := c.Create(ctx, sar); err != nil {
		return false, err
	}
	return sar.Status.Allowed, nil
}

func parseCSR(csr *certificatesv1beta1.CertificateSigningRequest) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode(csr.Spec.Request)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("PEM block type must be CERTIFICATE REQUEST")
	}
	return x509.ParseCertificateRequest(block.Bytes)
}

func isSeedClientCert(csr *certificatesv1beta1.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !reflect.DeepEqual([]string{v1beta1constants.SeedsGroup}, x509cr.Subject.Organization) {
		return false
	}
	if (len(x509cr.DNSNames) > 0) || (len(x509cr.EmailAddresses) > 0) || (len(x509cr.IPAddresses) > 0) {
		return false
	}
	if !hasExactUsages(csr, []certificatesv1beta1.KeyUsage{
		certificatesv1beta1.UsageKeyEncipherment,
		certificatesv1beta1.UsageDigitalSignature,
		certificatesv1beta1.UsageClientAuth,
	}) {
		return false
	}
	return strings.HasPrefix(x509cr.Subject.CommonName, v1beta1constants.SeedUserNamePrefix)
}

func hasExactUsages(csr *certificatesv1beta1.CertificateSigningRequest, usages []certificatesv1beta1.KeyUsage) bool {
	if len(usages) != len(csr.Spec.Usages) {
		return false
	}

	usageMap := map[certificatesv1beta1.KeyUsage]struct{}{}
	for _, u := range csr.Spec.Usages {
		usageMap[u] = struct{}{}
	}

	for _, u := range usages {
		if _, ok := usageMap[u]; !ok {
			return false
		}
	}

	return true
}
