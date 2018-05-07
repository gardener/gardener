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

package garden

import (
	"fmt"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
)

// New creates a new Garden object (based on a Shoot object).
func New(k8sGardenClient kubernetes.Client, namespace string) (*Garden, error) {
	ns, err := k8sGardenClient.GetNamespace(namespace)
	if err != nil {
		return nil, err
	}

	projectName := namespace
	if name, ok := ns.Labels[common.ProjectName]; ok && len(name) > 0 {
		projectName = name
	}

	return &Garden{
		ProjectName: projectName,
	}, nil
}

// ReadGardenSecrets reads the Kubernetes Secrets from the Garden cluster which are independent of Shoot clusters.
// The Secret objects are stored on the Controller in order to pass them to created Garden objects later.
func ReadGardenSecrets(k8sInformers kubeinformers.SharedInformerFactory, runningInCluster bool) (map[string]*corev1.Secret, error) {
	var (
		secretsMap                          = make(map[string]*corev1.Secret)
		numberOfInternalDomainSecrets       = 0
		numberOfOpenVPNDiffieHellmanSecrets = 0
	)

	selector, err := labels.Parse(common.GardenRole)
	if err != nil {
		return nil, err
	}
	secrets, err := k8sInformers.Core().V1().Secrets().Lister().Secrets(common.GardenNamespace).List(selector)
	if err != nil {
		return nil, err
	}

	for _, secret := range secrets {
		metadata := secret.ObjectMeta
		name := metadata.Name
		labels := metadata.Labels
		annotations := metadata.Annotations

		// Retrieving default domain secrets based on all secrets in the Garden namespace which have
		// a label indicating the Garden role default-domain.
		if labels[common.GardenRole] == common.GardenRoleDefaultDomain {
			domain, domainFound := annotations[common.DNSDomain]
			if !metav1.HasAnnotation(metadata, common.DNSHostedZoneID) || !metav1.HasAnnotation(metadata, common.DNSProvider) || !domainFound {
				logger.Logger.Warnf("The default domain secret %s does not contain all the annotations %s and %s and %s; ignoring it.", name, common.DNSHostedZoneID, common.DNSDomain, common.DNSProvider)
				continue
			}
			defaultDomainSecret := secret
			secretsMap[fmt.Sprintf("%s-%s", common.GardenRoleDefaultDomain, domain)] = defaultDomainSecret
			logger.Logger.Infof("Found default domain secret %s for domain %s.", name, domain)
		}

		// Retrieving internal domain secrets based on all secrets in the Garden namespace which have
		// a label indicating the Garden role internal-domain.
		if labels[common.GardenRole] == common.GardenRoleInternalDomain {
			domain, domainFound := annotations[common.DNSDomain]
			if !metav1.HasAnnotation(metadata, common.DNSHostedZoneID) || !metav1.HasAnnotation(metadata, common.DNSProvider) || !domainFound {
				logger.Logger.Warnf("The internal domain secret %s does not contain all the annotations %s and %s and %s; ignoring it.", name, common.DNSHostedZoneID, common.DNSDomain, common.DNSProvider)
				continue
			}
			internalDomainSecret := secret
			secretsMap[common.GardenRoleInternalDomain] = internalDomainSecret
			logger.Logger.Infof("Found internal domain secret %s for domain %s.", name, domain)
			numberOfInternalDomainSecrets++
		}

		// Retrieving alerting SMTP secrets based on all secrets in the Garden namespace which have
		// a label indicating the Garden role alerting-smtp.
		// Only when using the in-cluster config as we do not want to configure alerts in development modus.
		if labels[common.GardenRole] == common.GardenRoleAlertingSMTP && runningInCluster {
			alertingSMTP := secret
			secretsMap[fmt.Sprintf("%s-%s", common.GardenRoleAlertingSMTP, name)] = alertingSMTP
			logger.Logger.Infof("Found alerting SMTP secret %s.", name)
		}

		// Retrieving Diffie-Hellman secret for OpenVPN based on all secrets in the Garden namespace which have
		// a label indicating the Garden role openvpn-diffie-hellman.
		if labels[common.GardenRole] == common.GardenRoleOpenVPNDiffieHellman {
			openvpnDiffieHellman := secret
			key := "dh2048.pem"
			if _, ok := secret.Data[key]; !ok {
				return nil, fmt.Errorf("cannot use OpenVPN Diffie Hellman secret '%s' as it does not contain key '%s' (whose value should be the actual Diffie Hellman key)", secret.Name, key)
			}
			secretsMap[common.GardenRoleOpenVPNDiffieHellman] = openvpnDiffieHellman
			logger.Logger.Infof("Found OpenVPN Diffie Hellman secret %s.", name)
			numberOfOpenVPNDiffieHellmanSecrets++
		}
	}

	// For each Shoot we create a LoadBalancer(LB) pointing to the API server of the Shoot. Because the technical address
	// of the LB (ip or hostname) can change we cannot directly write it into the kubeconfig of the components
	// which talk from outside (kube-proxy, kubelet etc.) (otherwise those kubeconfigs would be broken once ip/hostname
	// of LB changed; and we don't have means to exchange kubeconfigs currently).
	// Therefore, to have a stable endpoint, we create a DNS record pointing to the ip/hostname of the LB. This DNS record
	// is used in all kubeconfigs. With that we have a robust endpoint stable against underlying ip/hostname changes.
	// And there can only be one of this internal domain secret because otherwise the gardener would not know which
	// domain it should use.
	if numberOfInternalDomainSecrets != 1 {
		return nil, fmt.Errorf("require exactly ONE internal domain secret, but found %d", numberOfInternalDomainSecrets)
	}

	// The VPN bridge from a Shoot's control plane running in the Seed cluster to the worker nodes of the Shoots is based
	// on OpenVPN. It requires a Diffie Hellman key. If no such key is explicitly provided as secret in the garden namespace
	// then the Gardener will use a default one (not recommended, but useful for local development). If a secret is specified
	// its key will be used for all Shoots. However, at most only one of such a secret is allowed to be specified (otherwise,
	// the Gardener cannot determine which to choose).
	if numberOfOpenVPNDiffieHellmanSecrets > 1 {
		return nil, fmt.Errorf("can only accept at most one OpenVPN Diffie Hellman secret, but found %d", numberOfOpenVPNDiffieHellmanSecrets)
	}

	return secretsMap, nil
}

// VerifyInternalDomainSecret verifies that the internal domain secret matches to the internal domain secret used for
// existing Shoot clusters. It is not allowed to change the internal domain secret if there are existing Shoot clusters.
func VerifyInternalDomainSecret(k8sGardenClient kubernetes.Client, numberOfShoots int, internalDomainSecret *corev1.Secret) error {
	currentDomain := internalDomainSecret.Annotations[common.DNSDomain]

	internalConfigMap, err := k8sGardenClient.GetConfigMap(common.GardenNamespace, common.ControllerManagerInternalConfigMapName)
	if apierrors.IsNotFound(err) || numberOfShoots == 0 {
		if _, err := k8sGardenClient.CreateConfigMap(common.GardenNamespace, common.ControllerManagerInternalConfigMapName, map[string]string{
			common.GardenRoleInternalDomain: currentDomain,
		}, true); err != nil {
			return err
		}
		return nil
	}
	if err != nil {
		return err
	}

	oldDomain := internalConfigMap.Data[common.GardenRoleInternalDomain]
	if oldDomain != currentDomain {
		return fmt.Errorf("cannot change internal domain from '%s' to '%s' unless there are no more Shoots", oldDomain, currentDomain)
	}

	return nil
}

// BootstrapCluster bootstraps the Garden cluster and deploys various required manifests.
func BootstrapCluster(k8sGardenClient kubernetes.Client, gardenNamespace string, secrets map[string]*corev1.Secret) error {
	// Check whether the Kubernetes version of the Garden cluster is at least 1.8.
	minGardenVersion := "1.8"
	gardenVersionOK, err := utils.CompareVersions(k8sGardenClient.Version(), ">=", minGardenVersion)
	if err != nil {
		return err
	}
	if !gardenVersionOK {
		return fmt.Errorf("the Kubernetes version of the Garden cluster must be at least %s", minGardenVersion)
	}
	return nil
}
