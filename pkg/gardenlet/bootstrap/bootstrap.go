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

	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap/certificate"
	bootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	certificatesv1beta1client "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateBootstrapClientFromKubeconfig creates a CertificatesV1beta1Client client from a given kubeconfig
func CreateBootstrapClientFromKubeconfig(kubeconfig []byte) (*certificatesv1beta1client.CertificatesV1beta1Client, *rest.Config, error) {
	bootstrapClientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		return nil, nil, err
	}

	bootstrapConfig, err := bootstrapClientConfig.ClientConfig()
	if err != nil {
		return nil, nil, err
	}
	bootstrapClient, err := certificatesv1beta1client.NewForConfig(bootstrapConfig)
	if err != nil {
		return nil, nil, err
	}
	return bootstrapClient, bootstrapConfig, nil
}

// RequestBootstrapKubeconfig creates a kubeconfig with a signed certificate using the given bootstrap client
// returns the kubeconfig []byte representation, the CSR name, the seed name or an error
func RequestBootstrapKubeconfig(ctx context.Context, logger logrus.FieldLogger, seedClient client.Client, bootstrapClient certificatesv1beta1client.CertificatesV1beta1Interface, bootstrapConfig *rest.Config, gardenClientConnection *config.GardenClientConnection, seedName, bootstrapTargetCluster string) ([]byte, string, string, error) {
	certificateSubject := &pkix.Name{
		Organization: []string{"gardener.cloud:system:seeds"},
		CommonName:   "gardener.cloud:system:seed:" + seedName,
	}

	certData, privateKeyData, csrName, err := certificate.RequestCertificate(ctx, logger, bootstrapClient, certificateSubject, []string{}, []net.IP{})
	if err != nil {
		return nil, "", "", fmt.Errorf("unable to bootstrap the kubeconfig for the Garden cluster: %v", err)
	}

	logger.Infof("Storing kubeconfig with bootstrapped certificate into secret (%s/%s) on target cluster '%s'", gardenClientConnection.KubeconfigSecret.Namespace, gardenClientConnection.KubeconfigSecret.Name, bootstrapTargetCluster)

	kubeconfig, err := bootstraputil.UpdateGardenKubeconfigSecret(ctx, bootstrapConfig, certData, privateKeyData, seedClient, gardenClientConnection)
	if err != nil {
		return nil, "", "", fmt.Errorf("unable to update secret (%s/%s) with bootstrapped kubeconfig: %v", gardenClientConnection.KubeconfigSecret.Namespace, gardenClientConnection.KubeconfigSecret.Name, err)
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
func DeleteBootstrapAuth(ctx context.Context, c client.Client, csrName, seedName string) error {
	csr := &certificatesv1beta1.CertificateSigningRequest{}
	if err := c.Get(ctx, kutil.Key(csrName), csr); err != nil {
		return err
	}

	var resourcesToDelete []runtime.Object

	switch {
	case strings.HasPrefix(csr.Spec.Username, bootstraptokenapi.BootstrapUserPrefix):
		resourcesToDelete = append(resourcesToDelete,
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("bootstrap-token-%s", strings.TrimPrefix(csr.Spec.Username, "system:bootstrap:")),
					Namespace: metav1.NamespaceSystem,
				},
			},
		)

	case strings.HasPrefix(csr.Spec.Username, serviceaccount.ServiceAccountUsernamePrefix):
		namespace, name, err := serviceaccount.SplitUsername(csr.Spec.Username)
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
					Name: bootstraputil.BuildBootstrapperName(seedName),
				},
			},
		)
	}

	for _, obj := range resourcesToDelete {
		if err := c.Delete(ctx, obj); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}
