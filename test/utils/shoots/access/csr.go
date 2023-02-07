// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package access

import (
	"context"
	"crypto/rand"
	"crypto/x509/pkix"
	"time"

	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/cert"
	csrutil "k8s.io/client-go/util/certificate/csr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

// labelsE2ETestCSRAccess is the set of labels added to all CSRs and ClusterRoleBindings for easy cleanup.
var labelsE2ETestCSRAccess = map[string]string{"e2e-test": "csr-access"}

// CreateShootClientFromCSR creates and approves a CSR in the shoot and creates a new shoot client from it.
// You should call CleanupObjectsFromCSRAccess to clean up the objects created by this function.
func CreateShootClientFromCSR(ctx context.Context, shootClient kubernetes.Interface, commonName string) (kubernetes.Interface, error) {
	// use fake key to avoid building complex retry/update logic
	privateKey, err := secretsutils.FakeGenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}

	csrData, err := cert.MakeCSR(privateKey, &pkix.Name{CommonName: commonName}, nil, nil)
	if err != nil {
		return nil, err
	}

	reqName, reqUID, err := csrutil.RequestCertificate(
		shootClient.Kubernetes(),
		csrData,
		commonName,
		certificatesv1.KubeAPIServerClientSignerName,
		pointer.Duration(3600*time.Second),
		[]certificatesv1.KeyUsage{
			certificatesv1.UsageDigitalSignature,
			certificatesv1.UsageKeyEncipherment,
			certificatesv1.UsageClientAuth,
		},
		privateKey,
	)
	if err != nil {
		return nil, err
	}

	csr := &certificatesv1.CertificateSigningRequest{}
	if err = shootClient.Client().Get(ctx, client.ObjectKey{Name: reqName}, csr); err != nil {
		return nil, err
	}

	patch := client.MergeFrom(csr.DeepCopy())
	csr.Labels = utils.MergeStringMaps(csr.Labels, labelsE2ETestCSRAccess)
	if err = shootClient.Client().Patch(ctx, csr, patch); err != nil {
		return nil, err
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: commonName}}
	if _, err = controllerutils.GetAndCreateOrMergePatch(ctx, shootClient.Client(), clusterRoleBinding, func() error {
		clusterRoleBinding.Labels = utils.MergeStringMaps(clusterRoleBinding.Labels, labelsE2ETestCSRAccess)
		clusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		}
		clusterRoleBinding.Subjects = []rbacv1.Subject{{
			Kind: rbacv1.UserKind,
			Name: commonName,
		}}
		return nil
	}); err != nil {
		return nil, err
	}

	hasApprovedCondition := false
	for _, condition := range csr.Status.Conditions {
		if condition.Type == certificatesv1.CertificateApproved {
			hasApprovedCondition = true
			break
		}
	}

	if !hasApprovedCondition {
		csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
			Type:    certificatesv1.CertificateApproved,
			Reason:  "AutoApproved",
			Message: "Auto approving test CertificateSigningRequest",
			Status:  corev1.ConditionTrue,
		})
		if err := shootClient.Client().SubResource("approval").Update(ctx, csr); err != nil {
			return nil, err
		}
	}

	certData, err := csrutil.WaitForCertificate(ctx, shootClient.Kubernetes(), reqName, reqUID)
	if err != nil {
		return nil, err
	}

	r := shootClient.RESTConfig()
	restConfig := &rest.Config{
		Host: r.Host,
		TLSClientConfig: rest.TLSClientConfig{
			CAData:   r.CAData,
			CertData: certData,
			KeyData:  utils.EncodePrivateKey(privateKey),
		},
	}

	return kubernetes.NewWithConfig(kubernetes.WithRESTConfig(restConfig), kubernetes.WithDisabledCachedClient())
}

// CleanupObjectsFromCSRAccess cleans up all objects in the shoot created by all calls to CreateShootClientFromCSR.
func CleanupObjectsFromCSRAccess(ctx context.Context, shootClient kubernetes.Interface) error {
	return flow.Parallel(func(ctx context.Context) error {
		return shootClient.Client().DeleteAllOf(ctx, &certificatesv1.CertificateSigningRequest{}, client.MatchingLabels(labelsE2ETestCSRAccess))
	}, func(ctx context.Context) error {
		return shootClient.Client().DeleteAllOf(ctx, &rbacv1.ClusterRoleBinding{}, client.MatchingLabels(labelsE2ETestCSRAccess))
	})(ctx)
}
