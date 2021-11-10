// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controller

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/validation"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/utils/pointer"
)

const (
	// secretDataKeyAPIServerCrt is a key in a secret for the Gardener API Server TLS certificate
	secretDataKeyAPIServerCrt = "gardener-apiserver.crt"
	// secretDataKeyAPIServerKey is a key in a secret for the Gardener API Server TLS key
	secretDataKeyAPIServerKey = "gardener-apiserver.key"

	// secretDataKeyEtcdCACrt is a key in a secret containing the etcd CA
	secretDataKeyEtcdCACrt = "etcd-client-ca.crt"
	// secretDataKeyEtcdCrt is a key in a secret containing the etcd client certificate
	secretDataKeyEtcdCrt = "etcd-client.crt"
	// secretDataKeyEtcdKey is a key in a secret containing the etcd client key
	secretDataKeyEtcdKey = "etcd-client.key"

	// secretDataKeyCACrt is a key in a secret containing a CA certificate
	secretDataKeyCACrt = "ca.crt"
	// secretDataKeyCAKey is a key in a secret containing a CA key
	secretDataKeyCAKey = "ca.key"
	// secretDataKeyCAKey is a key in a secret containing a TLS serving certificate
	secretDataKeyTLSCrt = "tls.crt"
	// secretDataKeyCAKey is a key in a secret containing the key of a TLS serving certificate
	secretDataKeyTLSKey = "tls.key"
)

// SyncWithExistingGardenerInstallation synchronizes the configuration of an existing Gardener installation to augment the locally provided configuration
// This is mostly for convenience purposes
// However, this means that in order to regenerate certificates they first need to be deleted from the cluster (in the future we might offer a dedicated flag for this)
// Exception: we do not store the Gardener CA's private Key in the cluster -> if generated CA, it is only exported during the first installation.
func (o *operation) SyncWithExistingGardenerInstallation(ctx context.Context) error {
	gardenClient := o.getGardenClient().Client()

	// get Gardener API server CA's public X509 certificate for the virtual-garden cluster
	if o.imports.GardenerAPIServer.ComponentConfiguration.CA == nil {
		apiService := &apiregistrationv1.APIService{}
		err := gardenClient.Get(ctx, kutil.Key(fmt.Sprintf("%s.%s", gardencorev1beta1.SchemeGroupVersion.Version, gardencorev1beta1.SchemeGroupVersion.Group)), apiService)
		if err == nil && len(apiService.Spec.CABundle) > 0 {
			if errors := validation.ValidateCACertificate(string(apiService.Spec.CABundle), field.NewPath("gardenerAPIServer.componentConfiguration.caBundle")); len(errors) > 0 {
				return fmt.Errorf("the existing CA certificate for the Gardener API server (configured in the API Service %q) is erroneous: %q", gardencorev1beta1.SchemeGroupVersion.String(), errors.ToAggregate().Error())
			}
			o.log.Infof("Using existing public Gardener x509 CA certificate found in APIService %q in the Garden cluster", gardencorev1beta1.SchemeGroupVersion.String())
			o.imports.GardenerAPIServer.ComponentConfiguration.CA = &imports.CA{Crt: pointer.String(string(apiService.Spec.CABundle))}
		} else if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to check if the CA Bundle for the Gardener API server already exists: %w", err)
		}
	}

	// get TLS Serving certificates of the Gardener API Server from the runtime cluster
	// secret: gardener-apiserver-cert
	secret := &corev1.Secret{}
	err := o.runtimeClient.Client().Get(ctx, kutil.Key(gardencorev1beta1constants.GardenNamespace, secretNameGardenerAPIServerCert), secret)
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to check if the TLS certificates for the Gardener API server already exists: %w", err)
	}

	if err == nil {
		if o.imports.GardenerAPIServer.ComponentConfiguration.Etcd.CABundle == nil {
			if etcdCACrt, ok := secret.Data[secretDataKeyEtcdCACrt]; ok {
				if errors := validation.ValidateCACertificate(string(etcdCACrt), field.NewPath("gardenerAPIServer.componentConfiguration.etcd.caBundle")); len(errors) > 0 {
					return fmt.Errorf("the existing etcd CA certificate configured in the secret (%s/%s) is erroneous: %s", gardencorev1beta1constants.GardenNamespace, secretNameGardenerAPIServerCert, errors.ToAggregate().Error())
				}
				o.log.Infof("Using existing etcd CA certificate found in secret %s/%s in the runtime cluster", gardencorev1beta1constants.GardenNamespace, secretNameGardenerAPIServerCert)
				o.imports.GardenerAPIServer.ComponentConfiguration.Etcd.CABundle = pointer.String(string(etcdCACrt))
			}
		}

		if o.imports.GardenerAPIServer.ComponentConfiguration.Etcd.ClientCert == nil {
			if etcdClientCert, ok := secret.Data[secretDataKeyEtcdCrt]; ok {
				if errors := validation.ValidateClientCertificate(string(etcdClientCert), field.NewPath("gardenerAPIServer.componentConfiguration.etcd.clientCert")); len(errors) > 0 {
					return fmt.Errorf("the existing etcd client certificate configured in the secret (%s/%s) is erroneous: %s", gardencorev1beta1constants.GardenNamespace, secretNameGardenerAPIServerCert, errors.ToAggregate().Error())
				}
				o.log.Infof("Using existing etcd client certificate found in secret %s/%s in the runtime cluster", gardencorev1beta1constants.GardenNamespace, secretNameGardenerAPIServerCert)
				o.imports.GardenerAPIServer.ComponentConfiguration.Etcd.ClientCert = pointer.String(string(etcdClientCert))
			}
		}

		if o.imports.GardenerAPIServer.ComponentConfiguration.Etcd.ClientKey == nil {
			if etcdClientCert, ok := secret.Data[secretDataKeyEtcdKey]; ok {
				if errors := validation.ValidatePrivateKey(string(etcdClientCert), field.NewPath("gardenerAPIServer.componentConfiguration.etcd.clientKey")); len(errors) > 0 {
					return fmt.Errorf("the existing etcd client key configured in the secret (%s/%s) is erroneous: %s", gardencorev1beta1constants.GardenNamespace, secretNameGardenerAPIServerCert, errors.ToAggregate().Error())
				}
				o.log.Infof("Using existing etcd client key found in secret %s/%s in the runtime cluster", gardencorev1beta1constants.GardenNamespace, secretNameGardenerAPIServerCert)
				o.imports.GardenerAPIServer.ComponentConfiguration.Etcd.ClientCert = pointer.String(string(etcdClientCert))
			}
		}

		apiServerCrt, foundServerCrt := secret.Data[secretDataKeyAPIServerCrt]
		if foundServerCrt {
			if errors := validation.ValidateTLSServingCertificate(string(apiServerCrt), field.NewPath("gardenerAPIServer.componentConfiguration.tls.crt")); len(errors) > 0 {
				return fmt.Errorf("the existing Gardener APIServer's TLS serving certificate configured in the secret (%s/%s) is erroneous: %s", gardencorev1beta1constants.GardenNamespace, secretNameGardenerAPIServerCert, errors.ToAggregate().Error())
			}
		}

		apiServerKey, foundServerKey := secret.Data[secretDataKeyAPIServerKey]
		if foundServerKey {
			if errors := validation.ValidateTLSServingCertificate(string(apiServerCrt), field.NewPath("gardenerAPIServer.componentConfiguration.tls.key")); len(errors) > 0 {
				return fmt.Errorf("the existing Gardener APIServer's TLS serving key configured in the secret (%s/%s) is erroneous: %s", gardencorev1beta1constants.GardenNamespace, secretNameGardenerAPIServerCert, errors.ToAggregate().Error())
			}
		}

		// only use if both the certificate and key are found
		if o.imports.GardenerAPIServer.ComponentConfiguration.TLS == nil && foundServerCrt && foundServerKey {
			o.log.Infof("Using existing Gardener API Server TLS serving certificate and key found in secret %s/%s in the runtime cluster", gardencorev1beta1constants.GardenNamespace, secretNameGardenerAPIServerCert)
			o.imports.GardenerAPIServer.ComponentConfiguration.TLS = &imports.TLSServer{
				Crt: pointer.String(string(apiServerCrt)),
				Key: pointer.String(string(apiServerKey)),
			}
		}
	}

	return nil
}
