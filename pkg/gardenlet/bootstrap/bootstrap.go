// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package bootstrap

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"strings"

	"github.com/go-logr/logr"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap/certificate"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// RequestKubeconfigWithBootstrapClient creates a kubeconfig with a signed certificate using the given bootstrap client
// returns the kubeconfig []byte representation, the CSR name, the seed name or an error
func RequestKubeconfigWithBootstrapClient(
	ctx context.Context,
	log logr.Logger,
	seedClient client.Client,
	bootstrapClientSet kubernetes.Interface,
	kubeconfigKey, bootstrapKubeconfigKey client.ObjectKey,
	seedName string,
	validityDuration *metav1.Duration,
) (
	[]byte,
	string,
	string,
	error,
) {
	certificateSubject := &pkix.Name{
		Organization: []string{v1beta1constants.SeedsGroup},
		CommonName:   v1beta1constants.SeedUserNamePrefix + seedName,
	}

	certData, privateKeyData, csrName, err := certificate.RequestCertificate(ctx, log, bootstrapClientSet.Kubernetes(), certificateSubject, []string{}, []net.IP{}, validityDuration)
	if err != nil {
		return nil, "", "", fmt.Errorf("unable to bootstrap the kubeconfig for the Garden cluster: %w", err)
	}

	log.Info("Storing kubeconfig with bootstrapped certificate in kubeconfig secret on target cluster")
	kubeconfig, err := gardenletbootstraputil.UpdateGardenKubeconfigSecret(ctx, bootstrapClientSet.RESTConfig(), certData, privateKeyData, seedClient, kubeconfigKey)
	if err != nil {
		return nil, "", "", fmt.Errorf("unable to update secret %q with bootstrapped kubeconfig: %w", kubeconfigKey.String(), err)
	}

	log.Info("Deleting boostrap kubeconfig secret from target cluster")
	if err := kubernetesutils.DeleteObject(ctx, seedClient, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bootstrapKubeconfigKey.Name,
			Namespace: bootstrapKubeconfigKey.Namespace,
		},
	}); err != nil {
		return nil, "", "", err
	}
	return kubeconfig, csrName, seedName, nil
}

// DeleteBootstrapAuth checks which authentication mechanism was used to request a certificate
// (either a bootstrap token or a service account token was used). If the latter is true then it
// also deletes the corresponding ClusterRoleBinding.
func DeleteBootstrapAuth(ctx context.Context, reader client.Reader, writer client.Writer, csrName, seedName string) error {
	csr := &certificatesv1.CertificateSigningRequest{}
	if err := reader.Get(ctx, kubernetesutils.Key(csrName), csr); err != nil {
		return err
	}

	var resourcesToDelete []client.Object

	switch {
	case strings.HasPrefix(csr.Spec.Username, bootstraptokenapi.BootstrapUserPrefix):
		resourcesToDelete = append(resourcesToDelete,
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bootstraptokenapi.BootstrapTokenSecretPrefix + strings.TrimPrefix(csr.Spec.Username, "system:bootstrap:"),
					Namespace: metav1.NamespaceSystem,
				},
			},
		)

	case strings.HasPrefix(csr.Spec.Username, serviceaccount.ServiceAccountUsernamePrefix):
		serviceAccountNamespace, serviceAccountName, err := serviceaccount.SplitUsername(csr.Spec.Username)
		if err != nil {
			return err
		}

		resourcesToDelete = append(resourcesToDelete,
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceAccountName,
					Namespace: serviceAccountNamespace,
				},
			},
			&rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: gardenletbootstraputil.ClusterRoleBindingName(serviceAccountNamespace, serviceAccountName),
				},
			},
		)
	}

	return kubernetesutils.DeleteObjects(ctx, writer, resourcesToDelete...)
}
