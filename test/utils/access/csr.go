// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

// labelsE2ETestCSRAccess is the set of labels added to all CSRs and ClusterRoleBindings for easy cleanup.
var labelsE2ETestCSRAccess = map[string]string{"e2e-test": "csr-access"}

// CreateTargetClientFromCSR creates and approves a CSR in the shoot and creates a new target client from it.
// You should call CleanupObjectsFromCSRAccess to clean up the objects created by this function.
func CreateTargetClientFromCSR(ctx context.Context, targetClient kubernetes.Interface, commonName string) (kubernetes.Interface, error) {
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
		targetClient.Kubernetes(),
		csrData,
		commonName,
		certificatesv1.KubeAPIServerClientSignerName,
		ptr.To(3600*time.Second),
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
	if err = targetClient.Client().Get(ctx, client.ObjectKey{Name: reqName}, csr); err != nil {
		return nil, err
	}

	patch := client.MergeFrom(csr.DeepCopy())
	csr.Labels = utils.MergeStringMaps(csr.Labels, labelsE2ETestCSRAccess)
	if err = targetClient.Client().Patch(ctx, csr, patch); err != nil {
		return nil, err
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: commonName}}
	if _, err = controllerutils.GetAndCreateOrMergePatch(ctx, targetClient.Client(), clusterRoleBinding, func() error {
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
		if err := targetClient.Client().SubResource("approval").Update(ctx, csr); err != nil {
			return nil, err
		}
	}

	certData, err := csrutil.WaitForCertificate(ctx, targetClient.Kubernetes(), reqName, reqUID)
	if err != nil {
		return nil, err
	}

	r := targetClient.RESTConfig()
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

// CleanupObjectsFromCSRAccess cleans up all objects in the target created by all calls to CreateTargetClientFromCSR.
func CleanupObjectsFromCSRAccess(ctx context.Context, targetClient kubernetes.Interface) error {
	return flow.Parallel(func(ctx context.Context) error {
		return targetClient.Client().DeleteAllOf(ctx, &certificatesv1.CertificateSigningRequest{}, client.MatchingLabels(labelsE2ETestCSRAccess))
	}, func(ctx context.Context) error {
		return targetClient.Client().DeleteAllOf(ctx, &rbacv1.ClusterRoleBinding{}, client.MatchingLabels(labelsE2ETestCSRAccess))
	})(ctx)
}
