// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/sirupsen/logrus"
	certificatesv1 "k8s.io/api/certificates/v1"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap/certificate"
	bootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// RequestBootstrapKubeconfig creates a kubeconfig with a signed certificate using the given bootstrap client
// returns the kubeconfig []byte representation, the CSR name, the seed name or an error
func RequestBootstrapKubeconfig(ctx context.Context, logger logrus.FieldLogger, seedClient client.Client, boostrapClientSet kubernetes.Interface, gardenClientConnection *config.GardenClientConnection, seedName, bootstrapTargetCluster string) ([]byte, string, string, error) {
	certificateSubject := &pkix.Name{
		Organization: []string{v1beta1constants.SeedsGroup},
		CommonName:   v1beta1constants.SeedUserNamePrefix + seedName,
	}

	certData, privateKeyData, csrName, err := certificate.RequestCertificate(ctx, logger, boostrapClientSet.Kubernetes(), certificateSubject, []string{}, []net.IP{})
	if err != nil {
		return nil, "", "", fmt.Errorf("unable to bootstrap the kubeconfig for the Garden cluster: %w", err)
	}

	logger.Infof("Storing kubeconfig with bootstrapped certificate into secret (%s/%s) on target cluster '%s'", gardenClientConnection.KubeconfigSecret.Namespace, gardenClientConnection.KubeconfigSecret.Name, bootstrapTargetCluster)

	kubeconfig, err := bootstraputil.UpdateGardenKubeconfigSecret(ctx, boostrapClientSet.RESTConfig(), certData, privateKeyData, seedClient, gardenClientConnection)
	if err != nil {
		return nil, "", "", fmt.Errorf("unable to update secret (%s/%s) with bootstrapped kubeconfig: %w", gardenClientConnection.KubeconfigSecret.Namespace, gardenClientConnection.KubeconfigSecret.Name, err)
	}

	logger.Infof("Deleting secret (%s/%s) containing the bootstrap kubeconfig from target cluster '%s')", gardenClientConnection.BootstrapKubeconfig.Namespace, gardenClientConnection.BootstrapKubeconfig.Name, bootstrapTargetCluster)

	bootstrapSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gardenClientConnection.BootstrapKubeconfig.Name,
			Namespace: gardenClientConnection.BootstrapKubeconfig.Namespace,
		},
	}

	if err := seedClient.Delete(ctx, bootstrapSecret); client.IgnoreNotFound(err) != nil {
		return nil, "", "", err
	}
	return kubeconfig, csrName, seedName, nil
}

// DeleteBootstrapAuth checks which authentication mechanism was used to request a certificate
// (either a bootstrap token or a service account token was used). If the latter is true then it
// also deletes the corresponding ClusterRoleBinding.
func DeleteBootstrapAuth(ctx context.Context, reader client.Reader, writer client.Writer, csrName, seedName string) error {
	var username string

	// try certificates v1 API first
	csr := &certificatesv1.CertificateSigningRequest{}
	if err := reader.Get(ctx, kutil.Key(csrName), csr); err == nil {
		username = csr.Spec.Username
	} else {
		if !meta.IsNoMatchError(err) {
			return err
		}

		// certificates v1 API not found, fall back to v1beta1
		csrv1beta1 := &certificatesv1beta1.CertificateSigningRequest{}
		if err := reader.Get(ctx, kutil.Key(csrName), csrv1beta1); err != nil {
			return err
		}
		username = csrv1beta1.Spec.Username
	}

	var resourcesToDelete []client.Object

	switch {
	case strings.HasPrefix(username, bootstraptokenapi.BootstrapUserPrefix):
		resourcesToDelete = append(resourcesToDelete,
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bootstraptokenapi.BootstrapTokenSecretPrefix + strings.TrimPrefix(username, "system:bootstrap:"),
					Namespace: metav1.NamespaceSystem,
				},
			},
		)

	case strings.HasPrefix(username, serviceaccount.ServiceAccountUsernamePrefix):
		namespace, name, err := serviceaccount.SplitUsername(username)
		if err != nil {
			return err
		}

		resourcesToDelete = append(resourcesToDelete,
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			},
			&rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: bootstraputil.ClusterRoleBindingName(v1beta1constants.GardenNamespace, seedName),
				},
			},
		)
	}

	return kutil.DeleteObjects(ctx, writer, resourcesToDelete...)
}
