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

	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/validation"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/etcdencryption"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/utils/pointer"
)

// SyncWithExistingGardenerInstallation synchronizes the configuration of an existing Gardener installation to augment the locally provided configuration
// This is mostly for convenience purposes & to reduce risk of regenerating a certificate because it is not supplied in the import config
// Please note that the CA's private key is not stored in-cluster for an existing installation
//   - therefore, if the public key of the Gardener API server can be obtained via the APIService, then the existing installation must contain the TLS serving certificates for the GAPI and GCM (GAPI: secret "garden/gardener-apiserver-cert" in the runtime cluster).
func (o *operation) SyncWithExistingGardenerInstallation(ctx context.Context) error {
	gardenClient := o.getGardenClient().Client()

	// get Gardener API server CA's public X509 certificate for the virtual-garden cluster
	if o.imports.GardenerAPIServer.ComponentConfiguration.CA.Crt == nil {
		apiService := &apiregistrationv1.APIService{}
		err := gardenClient.Get(ctx, kutil.Key(fmt.Sprintf("%s.%s", gardencorev1beta1.SchemeGroupVersion.Version, gardencorev1beta1.SchemeGroupVersion.Group)), apiService)
		if err == nil && len(apiService.Spec.CABundle) > 0 {
			if errors := validation.ValidateCACertificate(string(apiService.Spec.CABundle), field.NewPath("gardenerAPIServer.componentConfiguration.ca")); len(errors) > 0 {
				return fmt.Errorf("the existing CA certificate for the Gardener API server (configured in the API Service %q) is erroneous: %q", gardencorev1beta1.SchemeGroupVersion.String(), errors.ToAggregate().Error())
			}
			o.log.Infof("Using existing public Gardener x509 CA certificate found in APIService %q in the Garden cluster", gardencorev1beta1.SchemeGroupVersion.String())
			o.imports.GardenerAPIServer.ComponentConfiguration.CA.Crt = pointer.String(string(apiService.Spec.CABundle))
		} else if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to check if the CA Bundle for the Gardener API server already exists: %w", err)
		}
	}

	// get the Gardener API server CA private key if stored as a secret in cluster.
	// this secret is not available when running against an existing Gardener installation (control plane chart does not store the private key)
	// that was not set up using this landscaper component.
	// in such cases, please create the secret manually or provide the CA's private key via the import configuration
	if o.imports.GardenerAPIServer.ComponentConfiguration.CA.Key == nil {
		secret := &corev1.Secret{}
		err := o.runtimeClient.Client().Get(ctx, kutil.Key(gardencorev1beta1constants.GardenNamespace, secretNameLandscaperGardenerAPIServerKey), secret)

		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to check if the CA private key for the Gardener API server already exists: %w", err)
		}

		if err == nil {
			if key, ok := secret.Data[secretDataKeyCAKey]; ok {
				o.imports.GardenerAPIServer.ComponentConfiguration.CA.Key = pointer.String(string(key))
				o.log.Infof("Using existing Gardener API Server CA private key found in secret %s/%s in the runtime cluster", gardencorev1beta1constants.GardenNamespace, secretNameLandscaperGardenerAPIServerKey)
			}
		}
	}

	// get TLS Serving certificates of the Gardener API Server from the runtime cluster
	// secret: gardener-apiserver-cert
	secret := &corev1.Secret{}
	err := o.runtimeClient.Client().Get(ctx, kutil.Key(gardencorev1beta1constants.GardenNamespace, secretNameGardenerAPIServerCert), secret)

	if err != nil && !apierrors.IsNotFound(err) {
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

		if o.imports.GardenerAPIServer.ComponentConfiguration.TLS.Crt == nil {
			crt, foundServerCrt := secret.Data[secretDataKeyAPIServerCrt]
			key, foundServerKey := secret.Data[secretDataKeyAPIServerKey]

			// only use if both the certificate and key are found
			if foundServerCrt && foundServerKey {
				if errors := validation.ValidateTLS(string(crt), string(key), o.imports.GardenerAPIServer.ComponentConfiguration.CA.Crt, field.NewPath("gardenerAPIServer.componentConfiguration.tls")); len(errors) > 0 {
					return fmt.Errorf("the existing Gardener APIServer's TLS serving certificate configured in the secret (%s/%s) is erroneous: %s", gardencorev1beta1constants.GardenNamespace, secretNameGardenerAPIServerCert, errors.ToAggregate().Error())
				}

				o.imports.GardenerAPIServer.ComponentConfiguration.TLS.Crt = pointer.String(string(crt))
				o.imports.GardenerAPIServer.ComponentConfiguration.TLS.Key = pointer.String(string(key))
				o.log.Infof("Using existing Gardener API Server TLS serving certificate and key found in secret %s/%s in the runtime cluster", gardencorev1beta1constants.GardenNamespace, secretNameGardenerAPIServerCert)
			}
		}
	}

	// CA for Admission Controller
	if o.imports.GardenerAdmissionController.Enabled && o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Crt == nil {
		configuration := &admissionregistrationv1.MutatingWebhookConfiguration{}
		err := o.getGardenClient().Client().Get(ctx, kutil.Key(mutatingWebhookNameGardenerAdmissionController), configuration)

		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to check if the public CA certificate for the Gardener Admission Controller already exists: %w", err)
		}

		if err == nil && len(configuration.Webhooks) > 0 && len(configuration.Webhooks[0].ClientConfig.CABundle) > 0 {
			// expects the first webhook to contain the public CA Bundle
			if errors := validation.ValidateCACertificate(string(configuration.Webhooks[0].ClientConfig.CABundle), field.NewPath("gardenerAdmissionController.componentConfiguration.ca")); len(errors) > 0 {
				return fmt.Errorf("the existing etcd CA certificate configured in the MutatingWebhookConfiguration %q is erroneous: %s", mutatingWebhookNameGardenerAdmissionController, errors.ToAggregate().Error())
			}

			o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Crt = pointer.String(string(configuration.Webhooks[0].ClientConfig.CABundle))
			o.log.Infof("Using existing Gardener Admission Controller CA certificate found in MutatingWebhookConfiguration %q in the garden cluster", mutatingWebhookNameGardenerAdmissionController)
		}
	}

	// get the Gardener Admission Controller CA private key if stored as a secret in cluster.
	// this secret is not available when running against an existing Gardener installation (control plane chart does not store this private key)
	// that was not set up using this landscaper component.
	// in such cases, please create the secret manually or provide the CA's private key via the import configuration
	if o.imports.GardenerAdmissionController.Enabled && o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Key == nil {
		secret := &corev1.Secret{}
		err := o.runtimeClient.Client().Get(ctx, kutil.Key(gardencorev1beta1constants.GardenNamespace, secretNameLandscaperGardenerAdmissionControllerKey), secret)

		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to check if the CA private key for the Gardener Admission Controller already exists: %w", err)
		}

		if err == nil {
			if key, ok := secret.Data[secretDataKeyCAKey]; ok {
				o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Key = pointer.String(string(key))
				o.log.Infof("Using existing Gardener Admission Controller CA private key found in secret %s/%s in the runtime cluster", gardencorev1beta1constants.GardenNamespace, secretNameLandscaperGardenerAdmissionControllerKey)
			}
		}
	}

	// TLS certificates for the Gardener Admission Controller
	if o.imports.GardenerAdmissionController.Enabled && o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Crt == nil {
		secret := &corev1.Secret{}
		err := o.runtimeClient.Client().Get(ctx, kutil.Key(gardencorev1beta1constants.GardenNamespace, secretNameGardenerAdmissionControllerCert), secret)

		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to check if the TLS certificates for the Gardener Admission Controller already exists: %w", err)
		}

		crt, foundCrt := secret.Data[secretDataKeyTLSCrt]
		key, foundKey := secret.Data[secretDataKeyTLSKey]

		// only use if both the certificate and key are found
		if foundCrt && foundKey {
			if errors := validation.ValidateTLS(string(crt), string(key), o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Crt, field.NewPath("gardenerAdmissionController.componentConfiguration.tls")); len(errors) > 0 {
				return fmt.Errorf("the existing Gardener Admission Controller TLS serving certificate configured in the secret (%s/%s) is erroneous: %s", gardencorev1beta1constants.GardenNamespace, secretNameGardenerAdmissionControllerCert, errors.ToAggregate().Error())
			}
			o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Crt = pointer.String(string(crt))
			o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Key = pointer.String(string(key))
			o.log.Infof("Using existing Gardener Admission Controller TLS serving certificate and key found in secret %s/%s in the runtime cluster", gardencorev1beta1constants.GardenNamespace, secretNameGardenerAdmissionControllerCert)
		}
	}

	// TLS certificates for the Gardener Controller Manager
	if o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Crt == nil {
		secret := &corev1.Secret{}
		err := o.runtimeClient.Client().Get(ctx, kutil.Key(gardencorev1beta1constants.GardenNamespace, secretNameGardenerControllerManagerCert), secret)

		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to check if the TLS certificates for the Gardener Controller Manager already exists: %w", err)
		}

		crt, foundCrt := secret.Data[secretDataKeyControllerManagerCrt]
		key, foundKey := secret.Data[secretDataKeyControllerManagerKey]

		// only use if both the certificate and key are found
		if foundCrt && foundKey {
			if errors := validation.ValidateTLS(string(crt), string(key), o.imports.GardenerAPIServer.ComponentConfiguration.CA.Crt, field.NewPath("gardenerControllerManager.componentConfiguration.tls")); len(errors) > 0 {
				return fmt.Errorf("the existing Gardener Controller Manager TLS serving certificate configured in the secret (%s/%s) is erroneous: %s", gardencorev1beta1constants.GardenNamespace, secretNameGardenerAdmissionControllerCert, errors.ToAggregate().Error())
			}
			o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Crt = pointer.String(string(crt))
			o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Key = pointer.String(string(key))
			o.log.Infof("Using existing Gardener Controller Managers TLS serving certificate and key found in secret %s/%s in the runtime cluster", gardencorev1beta1constants.GardenNamespace, secretNameGardenerControllerManagerCert)
		}
	}

	// encryption configuration for the Gardener API Server
	if o.imports.GardenerAPIServer.ComponentConfiguration.Encryption == nil {
		secret := &corev1.Secret{}
		err := o.runtimeClient.Client().Get(ctx, kutil.Key(gardencorev1beta1constants.GardenNamespace, secretNameGardenerAPIServerEncryptionConfig), secret)

		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to check if the etcd encryption configuration for the Gardener API Server already exists: %w", err)
		}

		config, found := secret.Data[secretDataKeyAPIServerEncryptionConfig]
		if found {
			encryptionConfig, err := etcdencryption.Load(config)
			if err != nil {
				return fmt.Errorf("failed to reuse existing etcd encryption configuration from the secret %s/%s in the runtime cluster: %w", gardencorev1beta1constants.GardenNamespace, secretNameGardenerAPIServerEncryptionConfig, err)
			}

			o.imports.GardenerAPIServer.ComponentConfiguration.Encryption = encryptionConfig
			o.log.Infof("Using existing Gardener API Server encryption configuration found in secret %s/%s in the runtime cluster", gardencorev1beta1constants.GardenNamespace, secretNameGardenerAPIServerEncryptionConfig)
		}
	}

	return nil
}
