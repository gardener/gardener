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
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/logger"

	authorizationv1beta1 "k8s.io/api/authorization/v1beta1"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
)

func (c *Controller) csrAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.csrQueue.Add(key)
}

func (c *Controller) csrUpdate(oldObj, newObj interface{}) {
	c.csrAdd(newObj)
}

func (c *Controller) reconcileCertificateSigningRequestKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	csr, err := c.csrLister.Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Infof("[CSR AUTO APPROVER] Stopping operations for CSR %s since it has been deleted", key)
		c.csrQueue.Done(key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[CSR AUTO APPROVER] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	if err := c.control.Reconcile(csr, key); err != nil {
		c.csrQueue.AddAfter(key, 15*time.Second)
	}
	return nil
}

// ControlInterface implements the control logic for managing the lifecycle for certificate signing requests.
// It is implemented as an interface to allow for extensions that provide different semantics. Currently, there
// is only one implementation.
type ControlInterface interface {
	Reconcile(certificatesigningrequests *certificatesv1beta1.CertificateSigningRequest, key string) error
}

// NewDefaultControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for checking the lifecycle for certificate signing requests. You should
// use an instance returned from NewDefaultControl() for any scenario other than testing.
func NewDefaultControl(clientMap clientmap.ClientMap) ControlInterface {
	return &defaultControl{clientMap}
}

type defaultControl struct {
	clientMap clientmap.ClientMap
}

func (c *defaultControl) Reconcile(csrObj *certificatesv1beta1.CertificateSigningRequest, key string) error {
	var (
		ctx       = context.TODO()
		csr       = csrObj.DeepCopy()
		csrLogger = logger.NewFieldLogger(logger.Logger, "csr", csr.Name)
	)

	if len(csr.Status.Certificate) != 0 {
		return nil
	}

	for _, c := range csr.Status.Conditions {
		if c.Type == certificatesv1beta1.CertificateApproved {
			return nil
		}
		if c.Type == certificatesv1beta1.CertificateDenied {
			return nil
		}
	}

	x509cr, err := parseCSR(csr)
	if err != nil {
		return fmt.Errorf("unable to parse csr %q: %v", csr.Name, err)
	}

	if !isSeedClientCert(csr, x509cr) {
		return nil
	}

	csrLogger.Infof("[CSR APPROVER] %s", csr.Name)

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	approved, err := authorize(ctx, gardenClient.Client(), csr, authorizationv1beta1.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "seedclient"})
	if err != nil {
		return err
	}

	if approved {
		csrLogger.Infof("[CSR APPROVER] Auto-approving %s", csr.Name)

		csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1beta1.CertificateSigningRequestCondition{
			Type:    certificatesv1beta1.CertificateApproved,
			Reason:  "AutoApproved",
			Message: "Auto approving gardenlet client certificate after SubjectAccessReview.",
		})
		_, err := gardenClient.Kubernetes().CertificatesV1beta1().CertificateSigningRequests().UpdateApproval(ctx, csr, kubernetes.DefaultUpdateOptions())
		return err
	}

	message := fmt.Sprintf("recognized csr %q but subject access review was not approved", csr.Name)
	csrLogger.Errorf("[CSR APPROVER] %s", message)
	return fmt.Errorf(message)
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
	if !reflect.DeepEqual([]string{"gardener.cloud:system:seeds"}, x509cr.Subject.Organization) {
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
	if !strings.HasPrefix(x509cr.Subject.CommonName, "gardener.cloud:system:seed:") {
		return false
	}
	return true
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
