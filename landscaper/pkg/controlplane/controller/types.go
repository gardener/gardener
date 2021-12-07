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

const (
	// commonNameGardenerCA is the common name of the Gardener CA
	commonNameGardenerCA = "ca-gardener"
	// commonNameGardenerAdmissionController is the common name of the Gardener Admission Controller CA
	commonNameGardenerAdmissionController = "ca-gardener-admission-controller"

	// serviceNameGardenerAPIServer is the service name of the Gardener API Server in the runtime cluster
	serviceNameGardenerAPIServer = "gardener-apiserver"
	// serviceNameGardenerControllerManager is the service name of the Gardener Controller Manager in the runtime cluster
	serviceNameGardenerControllerManager = "gardener-controller-manager"
	// serviceNameGardenerAdmissionController is the service name of the Gardener Admission Controller in the runtime cluster
	serviceNameGardenerAdmissionController = "gardener-admission-controller"

	// secretNameGardenerAPIServerCert is the name of the secret mounted by the Gardener API server containing the CA bundle and TLS serving certificates
	secretNameGardenerAPIServerCert = "gardener-apiserver-cert"
	// secretNameGardenerAPIServerEncryptionConfig is the name of the secret mounted by the Gardener API server containing the etcd encryption config
	secretNameGardenerAPIServerEncryptionConfig = "gardener-apiserver-encryption-config"
	// secretNameGardenerAdmissionControllerCert is the name of the secret mounted by the Gardener Admission Controller containing its TLS serving certificates
	secretNameGardenerAdmissionControllerCert = "gardener-admission-controller-cert"
	// secretNameGardenerControllerManagerCert is the name of the secret mounted by the Gardener Controller Manager containing its TLS serving certificates
	secretNameGardenerControllerManagerCert = "gardener-controller-manager-cert"

	// secretNameGardenerEncryptionConfig is the name of the secret mounted by the Gardener API server containing the encryption configuration
	secretNameGardenerEncryptionConfig = "gardener-apiserver-encryption-config"
	// secretNameOpenVPNDiffieHellmann is the name of the secret mounted by the Gardener API server containing the OpenVPN Diffie Hellmann key
	secretNameOpenVPNDiffieHellmann = "openvpn-diffie-hellman-key"

	// secretNameLandscaperGardenerAPIServerKey is the name of the secret in the runtime cluster holding the generated private key of the Gardener API Server
	secretNameLandscaperGardenerAPIServerKey = "landscaper-controlplane-apiserver-ca-key"
	// secretNameLandscaperGardenerAPIServerKey is the name of the secret in the runtime cluster holding the generated private key of the Gardener API Server
	secretNameLandscaperGardenerAdmissionControllerKey = "landscaper-controlplane-admission-controller-ca-key"

	// cmNameClusterIdentity is the name of the config map containing the cluster identity
	cmNameClusterIdentity = "cluster-identity"
	// cmDataKeyClusterIdentity is the data key for the config map containing the cluster identity
	cmDataKeyClusterIdentity = "cluster-identity"

	// deploymentNameGardenerAPIServer is the name of the Gardener API Server deployment
	deploymentNameGardenerAPIServer = "gardener-apiserver"
	// deploymentNameGardenerControllerManager is the name of the Gardener Controller Manager deployment
	deploymentNameGardenerControllerManager = "gardener-controller-manager"
	// deploymentNameGardenerScheduler is the name of the Gardener Scheduler deployment
	deploymentNameGardenerScheduler = "gardener-scheduler"
	// deploymentNameGardenerAdmissionController is the name of the Gardener Admission Controller deployment
	deploymentNameGardenerAdmissionController = "gardener-admission-controller"

	// mutatingWebhookNameGardenerAdmissionController is the name of the mutating webhook configuration for the Gardener Admission Controller
	mutatingWebhookNameGardenerAdmissionController = "gardener-admission-controller"
	// mutatingWebhookNameValidateNamespaceDeletion is the name of the validating webhook configuration to validate resources in the Garden cluster.
	// Served by the Gardener Admission Controller
	validatingWebhookNameValidateNamespaceDeletion = "validate-namespace-deletion"

	// secretDataKeyDiffieHellmann is a constant for a key in the data map that contains the Diffie-Hellmann key
	secretDataKeyDiffieHellmann = "dh2048.pem"
	// secretDataKeyCACrt is a key in a secret containing a CA certificate
	secretDataKeyCACrt = "ca.crt"
	// secretDataKeyCAKey is a key in a secret containing a CA key
	secretDataKeyCAKey = "ca.key"
	// secretDataKeyCAKey is a key in a secret containing a TLS serving certificate
	secretDataKeyTLSCrt = "tls.crt"
	// secretDataKeyCAKey is a key in a secret containing the key of a TLS serving certificate
	secretDataKeyTLSKey = "tls.key"
	// secretDataKeyAPIServerCrt is a key in a secret for the Gardener API Server TLS certificate
	secretDataKeyAPIServerCrt = "gardener-apiserver.crt"
	// secretDataKeyAPIServerKey is a key in a secret for the Gardener API Server TLS key
	secretDataKeyAPIServerKey = "gardener-apiserver.key"
	// secretDataKeyAPIServerEncryptionConfig is a constant for a key in the secret containing the config
	// which is used to encrypt etcd data.
	secretDataKeyAPIServerEncryptionConfig = "encryption-config.yaml"
	// secretDataKeyControllerManagerCrt is a key in a secret for the Gardener Controller Manager TLS certificate
	secretDataKeyControllerManagerCrt = "gardener-controller-manager.crt"
	// secretDataKeyControllerManagerKey is a key in a secret for the Gardener Controller Manager TLS key
	secretDataKeyControllerManagerKey = "gardener-controller-manager.key"
	// secretDataKeyEtcdCACrt is a key in a secret containing the etcd CA
	secretDataKeyEtcdCACrt = "etcd-client-ca.crt"
	// secretDataKeyEtcdCrt is a key in a secret containing the etcd client certificate
	secretDataKeyEtcdCrt = "etcd-client.crt"
	// secretDataKeyEtcdKey is a key in a secret containing the etcd client key
	secretDataKeyEtcdKey = "etcd-client.key"
)
