// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package app

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	certificatesv1beta1client "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/keyutil"
	componentbaseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func bootstrapKubeconfig(
	ctx context.Context,
	logger *logrus.Logger,
	gardenClientConnection *config.GardenClientConnection,
	seedClientConnection componentbaseconfig.ClientConnectionConfiguration,
	seedConfig *config.SeedConfig,
) (
	[]byte,
	string,
	string,
	error,
) {
	seedRESTCfg, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&seedClientConnection, nil)
	if err != nil {
		return nil, "", "", err
	}
	k8sSeedClient, err := kubernetes.NewWithConfig(kubernetes.WithRESTConfig(seedRESTCfg))
	if err != nil {
		return nil, "", "", err
	}

	kubeconfigSecret := &corev1.Secret{}
	if err := k8sSeedClient.DirectClient().Get(ctx, kutil.Key(gardenClientConnection.KubeconfigSecret.Namespace, gardenClientConnection.KubeconfigSecret.Name), kubeconfigSecret); client.IgnoreNotFound(err) != nil {
		return nil, "", "", err
	}

	if len(kubeconfigSecret.Data[kubernetes.KubeConfig]) > 0 {
		logger.Info("Found kubeconfig generated from bootstrap process. Using it")
		return kubeconfigSecret.Data[kubernetes.KubeConfig], "", "", nil
	}

	// kubeconfig secret not found or empty - trigger bootstrap process
	if gardenClientConnection.BootstrapKubeconfig == nil {
		return nil, "", "", fmt.Errorf("cannot trigger kubeconfig bootstrap process because `.gardenClientConnection.bootstrapKubeconfig` is not set")
	}

	logger.Info("No kubeconfig for garden cluster found, but bootstrap kubeconfig was given.")

	// check if we got a kubeconfig for bootstrapping
	bootstrapKubeconfigSecret := &corev1.Secret{}
	if err := k8sSeedClient.DirectClient().Get(ctx, kutil.Key(gardenClientConnection.BootstrapKubeconfig.Namespace, gardenClientConnection.BootstrapKubeconfig.Name), bootstrapKubeconfigSecret); err != nil {
		return nil, "", "", err
	}
	if len(bootstrapKubeconfigSecret.Data[kubernetes.KubeConfig]) == 0 {
		return nil, "", "", fmt.Errorf("bootstrap kubeconfig secret does not contain a kubeconfig")
	}

	// create certificate client with bootstrap kubeconfig in order to create CSR
	bootstrapClientConfig, err := clientcmd.NewClientConfigFromBytes(bootstrapKubeconfigSecret.Data[kubernetes.KubeConfig])
	if err != nil {
		return nil, "", "", err
	}
	bootstrapConfig, err := bootstrapClientConfig.ClientConfig()
	if err != nil {
		return nil, "", "", err
	}
	bootstrapClient, err := certificatesv1beta1client.NewForConfig(bootstrapConfig)
	if err != nil {
		return nil, "", "", fmt.Errorf("unable to create certificates signing request client: %v", err)
	}

	logger.Info("Creating certificate signing request...")

	// generate a new private key and create a CSR resource + wait until it got approved/signed
	seedName := "<ambiguous>"
	if seedConfig != nil {
		seedName = seedConfig.Name
	}
	privateKeyData, err := keyutil.MakeEllipticPrivateKeyPEM()
	if err != nil {
		return nil, "", "", fmt.Errorf("error generating key: %v", err)
	}
	certData, csrName, err := bootstrap.RequestSeedCertificate(ctx, bootstrapClient.CertificateSigningRequests(), privateKeyData, seedName)
	if err != nil {
		return nil, "", "", err
	}

	logger.Infof("Certificate signing request got approved! Creating kubeconfig and storing it in secret %s/%s", gardenClientConnection.KubeconfigSecret.Namespace, gardenClientConnection.KubeconfigSecret.Name)

	// marshal kubeconfig with just derived client certificate
	kubeconfig, err := bootstrap.MarshalKubeconfigWithClientCertificate(bootstrapConfig, privateKeyData, certData)
	if err != nil {
		return nil, "", "", err
	}

	// store kubeconfig in kubeconfig secret in seed cluster and delete bootstrap kubeconfig secret
	kubeconfigSecret.ObjectMeta = metav1.ObjectMeta{
		Name:      gardenClientConnection.KubeconfigSecret.Name,
		Namespace: gardenClientConnection.KubeconfigSecret.Namespace,
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, k8sSeedClient.DirectClient(), kubeconfigSecret, func() error {
		kubeconfigSecret.Data = map[string][]byte{kubernetes.KubeConfig: kubeconfig}
		return nil
	}); err != nil {
		return nil, "", "", err
	}

	logger.Infof("Deleting secret %s/%s holding bootstrap kubeconfig", gardenClientConnection.BootstrapKubeconfig.Namespace, gardenClientConnection.BootstrapKubeconfig.Name)

	if err := k8sSeedClient.DirectClient().Delete(ctx, bootstrapKubeconfigSecret); client.IgnoreNotFound(err) != nil {
		return nil, "", "", err
	}

	return kubeconfig, csrName, seedName, nil
}
