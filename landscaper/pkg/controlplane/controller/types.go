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
	// secretNameGardenerEncryptionConfig is the name of the secret mounted by the Gardener API server containing the encryption configuration
	secretNameGardenerEncryptionConfig = "gardener-apiserver-encryption-config"
	// secretNameOpenVPNDiffieHellmann is the name of the secret mounted by the Gardener API server containing the OpenVPN Diffie Hellmann key
	secretNameOpenVPNDiffieHellmann = "openvpn-diffie-hellman-key"

	// secretDataKeyEtcdEncryption is a constant for a key in the data map that contains the config
	// which is used to encrypt etcd data.
	secretDataKeyEtcdEncryption = "encryption-config.yaml"
	// secretDataKeyDiffieHellmann is a constant for a key in the data map that contains the Diffie-Hellmann key
	secretDataKeyDiffieHellmann = "dh2048.pem"

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
)
