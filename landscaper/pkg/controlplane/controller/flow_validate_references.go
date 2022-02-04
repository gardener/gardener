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
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FetchAndValidateConfigurationFromSecretReferences fetches the configuration that is provided as secret references, validates and adds it to the import configuration.
func (o *operation) FetchAndValidateConfigurationFromSecretReferences(ctx context.Context) error {
	// Gardener API Server
	if o.imports.GardenerAPIServer.ComponentConfiguration.CA.SecretRef != nil {
		ca, key, err := ValidateCAConfigurationFromSecretReferences(ctx, o.runtimeClient.Client(), o.imports.GardenerAPIServer.ComponentConfiguration.CA.SecretRef, field.NewPath("gardenerAPIServer.componentConfiguration.ca"))
		if err != nil {
			return fmt.Errorf("failed to validate Gardener API Server CA certificate: %v", err)
		}

		if ca != nil {
			o.log.Debugf("Using configured Gardener API Server CA certificate configured in the secret %s/%s in the runtime cluster", o.imports.GardenerAPIServer.ComponentConfiguration.CA.SecretRef.Namespace, o.imports.GardenerAPIServer.ComponentConfiguration.CA.SecretRef.Name)
			o.imports.GardenerAPIServer.ComponentConfiguration.CA.Crt = pointer.String(*ca)
		}

		if key != nil {
			o.log.Debugf("Using configured Gardener API Server CA key configured in the secret %s/%s in the runtime cluster", o.imports.GardenerAPIServer.ComponentConfiguration.CA.SecretRef.Namespace, o.imports.GardenerAPIServer.ComponentConfiguration.CA.SecretRef.Name)
			o.imports.GardenerAPIServer.ComponentConfiguration.CA.Key = pointer.String(*key)
		}
	}

	if o.imports.GardenerAPIServer.ComponentConfiguration.TLS.SecretRef != nil {
		cert, key, err := ValidateTLSConfigurationFromSecretReferences(ctx, o.runtimeClient.Client(), o.imports.GardenerAPIServer.ComponentConfiguration.TLS.SecretRef, o.imports.GardenerAPIServer.ComponentConfiguration.CA, field.NewPath("gardenerAPIServer.componentConfiguration.tls"))
		if err != nil {
			return fmt.Errorf("failed to validate Gardener API Server TLS certificates: %v", err)
		}

		if cert != nil {
			o.log.Debugf("Using configured Gardener API Server TLS serving certificate configured in the secret %s/%s in the runtime cluster", o.imports.GardenerAPIServer.ComponentConfiguration.TLS.SecretRef.Namespace, o.imports.GardenerAPIServer.ComponentConfiguration.TLS.SecretRef.Name)
			o.imports.GardenerAPIServer.ComponentConfiguration.TLS.Crt = pointer.String(*cert)
		}

		if key != nil {
			o.log.Debugf("Using configured Gardener API Server TLS serving key configured in the secret %s/%s in the runtime cluster", o.imports.GardenerAPIServer.ComponentConfiguration.TLS.SecretRef.Namespace, o.imports.GardenerAPIServer.ComponentConfiguration.TLS.SecretRef.Name)
			o.imports.GardenerAPIServer.ComponentConfiguration.TLS.Key = pointer.String(*key)
		}
	}

	if o.imports.EtcdSecretRef != nil {
		secret := &corev1.Secret{}
		if err := o.runtimeClient.Client().Get(ctx, kutil.Key(o.imports.EtcdSecretRef.Namespace, o.imports.EtcdSecretRef.Name), secret); err != nil {
			if apierrors.IsNotFound(err) {
				return fmt.Errorf("secret %s/%s configured to contain the etcd certificates, does not exist in the runtime cluster: %v", o.imports.EtcdSecretRef.Namespace, o.imports.EtcdSecretRef.Name, err)
			}
			fmt.Errorf("failed to retrieve secret %s/%s from the runtime cluster: %v", o.imports.EtcdSecretRef.Namespace, o.imports.EtcdSecretRef.Name, err)
		}

		if ca, ok := secret.Data[secretDataKeyCACrt]; ok {
			if errors := validation.ValidateCACertificate(string(ca), field.NewPath("etcdSecretRef")); len(errors) > 0 {
				return fmt.Errorf("the configured etcd CA certificate configured in the secret (%s/%s) is erroneous: %s", o.imports.EtcdSecretRef.Namespace, o.imports.EtcdSecretRef.Name, errors.ToAggregate().Error())
			}
			o.log.Debugf("Using configured etcd CA certificate configured in the secret %s/%s in the runtime cluster", o.imports.EtcdSecretRef.Namespace, o.imports.EtcdSecretRef.Name)
			o.imports.EtcdCABundle = pointer.String(string(ca))
		}

		if cert, ok := secret.Data[secretDataKeyTLSCrt]; ok {
			if errors := validation.ValidateClientCertificate(string(cert), field.NewPath("etcdSecretRef")); len(errors) > 0 {
				return fmt.Errorf("the configured etcd client certificate configured in the secret (%s/%s) is erroneous: %s", o.imports.EtcdSecretRef.Namespace, o.imports.EtcdSecretRef.Name, errors.ToAggregate().Error())
			}
			o.log.Debugf("Using configured etcd client certificate configured in the secret %s/%s in the runtime cluster", o.imports.EtcdSecretRef.Namespace, o.imports.EtcdSecretRef.Name)
			o.imports.EtcdClientCert = pointer.String(string(cert))
		}

		if key, ok := secret.Data[secretDataKeyTLSKey]; ok {
			if errors := validation.ValidatePrivateKey(string(key), field.NewPath("etcdSecretRef")); len(errors) > 0 {
				return fmt.Errorf("the configured etcd client key configured in the secret (%s/%s) is erroneous: %s", o.imports.EtcdSecretRef.Namespace, o.imports.EtcdSecretRef.Name, errors.ToAggregate().Error())
			}
			o.log.Debugf("Using configured etcd client key configured in the secret %s/%s in the runtime cluster", o.imports.EtcdSecretRef.Namespace, o.imports.EtcdSecretRef.Name)
			o.imports.EtcdClientKey = pointer.String(string(key))
		}
	}

	// Gardener Admission Controller
	if o.imports.GardenerAdmissionController.Enabled && o.imports.GardenerAdmissionController.ComponentConfiguration.CA.SecretRef != nil {
		ca, key, err := ValidateCAConfigurationFromSecretReferences(ctx, o.runtimeClient.Client(), o.imports.GardenerAdmissionController.ComponentConfiguration.CA.SecretRef, field.NewPath("gardenerAdmissionController.componentConfiguration.ca"))
		if err != nil {
			return fmt.Errorf("failed to validate GardenerAdmissionController TLS certificates: %v", err)
		}

		if ca != nil {
			o.log.Debugf("Using configured GardenerAdmissionController CA certificate configured in the secret %s/%s in the runtime cluster", o.imports.GardenerAdmissionController.ComponentConfiguration.CA.SecretRef.Namespace, o.imports.GardenerAdmissionController.ComponentConfiguration.CA.SecretRef.Name)
			o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Crt = pointer.String(*ca)
		}

		if key != nil {
			o.log.Debugf("Using configured GardenerAdmissionController CA key configured in the secret %s/%s in the runtime cluster", o.imports.GardenerAdmissionController.ComponentConfiguration.CA.SecretRef.Namespace, o.imports.GardenerAdmissionController.ComponentConfiguration.CA.SecretRef.Name)
			o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Key = pointer.String(*key)
		}
	}

	if o.imports.GardenerAdmissionController.Enabled && o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.SecretRef != nil {
		cert, key, err := ValidateTLSConfigurationFromSecretReferences(ctx, o.runtimeClient.Client(), o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.SecretRef, o.imports.GardenerAdmissionController.ComponentConfiguration.CA, field.NewPath("gardenerAdmissionController.componentConfiguration.tls"))
		if err != nil {
			return fmt.Errorf("failed to validate Gardener API Server TLS certificates: %v", err)
		}

		if cert != nil {
			o.log.Debugf("Using configured GardenerAdmissionController TLS serving certificate configured in the secret %s/%s in the runtime cluster", o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.SecretRef.Namespace, o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.SecretRef.Name)
			o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Crt = pointer.String(*cert)
		}

		if key != nil {
			o.log.Debugf("Using configured GardenerAdmissionController TLS serving key configured in the secret %s/%s in the runtime cluster", o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.SecretRef.Namespace, o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.SecretRef.Name)
			o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Key = pointer.String(*key)
		}
	}

	// Gardener Controller Manager
	if o.imports.GardenerControllerManager.ComponentConfiguration.TLS.SecretRef != nil {
		cert, key, err := ValidateTLSConfigurationFromSecretReferences(ctx, o.runtimeClient.Client(), o.imports.GardenerControllerManager.ComponentConfiguration.TLS.SecretRef, o.imports.GardenerAPIServer.ComponentConfiguration.CA, field.NewPath("gardenerControllerManager.componentConfiguration.tls"))
		if err != nil {
			return fmt.Errorf("failed to validate GardenerControllerManager TLS certificates: %v", err)
		}

		if cert != nil {
			o.log.Debugf("Using configured GardenerControllerManager TLS serving certificate configured in the secret %s/%s in the runtime cluster", o.imports.GardenerControllerManager.ComponentConfiguration.TLS.SecretRef.Namespace, o.imports.GardenerControllerManager.ComponentConfiguration.TLS.SecretRef.Name)
			o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Crt = pointer.String(*cert)
		}

		if key != nil {
			o.log.Debugf("Using configured GardenerControllerManager TLS serving key configured in the secret %s/%s in the runtime cluster", o.imports.GardenerControllerManager.ComponentConfiguration.TLS.SecretRef.Namespace, o.imports.GardenerControllerManager.ComponentConfiguration.TLS.SecretRef.Name)
			o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Key = pointer.String(*key)
		}
	}

	return nil
}

// ValidateCAConfigurationFromSecretReferences validates the CA certificate and key obtained via the given secret reference
// returns the certificate as first, and the key as second argument, or an error
func ValidateCAConfigurationFromSecretReferences(ctx context.Context, client client.Client, ref *corev1.SecretReference, path *field.Path) (*string, *string, error) {
	secret := &corev1.Secret{}
	if err := client.Get(ctx, kutil.Key(ref.Namespace, ref.Name), secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil, fmt.Errorf("secret %s/%s configured to contain the Gardener API Server certificates, does not exist in the runtime cluster: %v", ref.Namespace, ref.Name, err)
		}
		return nil, nil, fmt.Errorf("failed to retrieve secret %s/%s from the runtime cluster: %v", ref.Namespace, ref.Name, err)
	}

	ca, ok := secret.Data[secretDataKeyCACrt]
	if ok {
		if errors := validation.ValidateCACertificate(string(ca), path.Child("secretRef")); len(errors) > 0 {
			return nil, nil, fmt.Errorf("the configured CA certificate configured in the secret (%s/%s) is erroneous: %s", ref.Namespace, ref.Name, errors.ToAggregate().Error())
		}
	}

	key, ok := secret.Data[secretDataKeyCAKey]
	if ok {
		if errors := validation.ValidatePrivateKey(string(key), path.Child("secretRef")); len(errors) > 0 {
			return nil, nil, fmt.Errorf("the configured CA key configured in the secret (%s/%s) is erroneous: %s", ref.Namespace, ref.Name, errors.ToAggregate().Error())
		}
	}

	return pointer.String(string(ca)), pointer.String(string(key)), nil
}

// ValidateTLSConfigurationFromSecretReferences validates the TLS serving certificate and key obtained via the given secret reference
// returns the TLS serving certificate as first, and the key as second argument, or an error
func ValidateTLSConfigurationFromSecretReferences(ctx context.Context, client client.Client, ref *corev1.SecretReference, ca *imports.CA, path *field.Path) (*string, *string, error) {
	secret := &corev1.Secret{}
	if err := client.Get(ctx, kutil.Key(ref.Namespace, ref.Name), secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil, fmt.Errorf("secret %s/%s configured to contain the TLS serving certificates, does not exist in the runtime cluster: %v", ref.Namespace, ref.Name, err)
		}
		return nil, nil, fmt.Errorf("failed to retrieve secret %s/%s from the runtime cluster: %v", ref.Namespace, ref.Name, err)
	}

	cert, found := secret.Data[secretDataKeyTLSCrt]
	if found {
		if errors := validation.ValidateTLSServingCertificate(string(cert), path.Child("secretRef")); len(errors) > 0 {
			return nil, nil, fmt.Errorf("the TLS serving certificate configured in the secret (%s/%s) is erroneous: %s", ref.Namespace, ref.Name, errors.ToAggregate().Error())
		}

		// we know there must be a CA configured as this is enforced by validation
		// let us validate against it
		if ca != nil && ca.Crt != nil {
			if errors := validation.ValidateTLSServingCertificateAgainstCA(string(cert), *ca.Crt, path.Child("secretRef")); len(errors) > 0 {
				return nil, nil, fmt.Errorf("the TLS serving certificate configured in the secret (%s/%s) failed the validation against the CA: %s", ref.Namespace, ref.Name, errors.ToAggregate().Error())
			}
		}
	}

	key, found := secret.Data[secretDataKeyTLSKey]
	if found {
		if errors := validation.ValidatePrivateKey(string(key), path.Child("secretRef")); len(errors) > 0 {
			return nil, nil, fmt.Errorf("the existing TLS serving key configured in the secret (%s/%s) is erroneous: %s", ref.Namespace, ref.Name, errors.ToAggregate().Error())
		}
	}

	return pointer.String(string(cert)), pointer.String(string(key)), nil
}
