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
	"strings"

	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
)

// New creates a new Garden object (based on a Shoot object).
func New(projectLister gardenlisters.ProjectLister, namespace string, secrets map[string]*corev1.Secret) (*Garden, error) {
	project, err := common.ProjectForNamespace(projectLister, namespace)
	if err != nil {
		return nil, err
	}

	internalDomain, err := GetInternalDomain(secrets)
	if err != nil {
		return nil, err
	}

	defaultDomains, err := GetDefaultDomains(secrets)
	if err != nil {
		return nil, err
	}

	return &Garden{
		Project:        project,
		InternalDomain: internalDomain,
		DefaultDomains: defaultDomains,
	}, nil
}

// GetDefaultDomains finds all the default domain secrets within the given map and returns a list of
// objects that contains all relevant information about the default domains.
func GetDefaultDomains(secrets map[string]*corev1.Secret) ([]*DefaultDomain, error) {
	var defaultDomains []*DefaultDomain

	for key, secret := range secrets {
		if strings.HasPrefix(key, common.GardenRoleDefaultDomain) {
			provider, domain, err := common.GetDomainInfoFromAnnotations(secret.Annotations)
			if err != nil {
				return nil, fmt.Errorf("error getting information out of default domain secret: %+v", err)
			}

			defaultDomains = append(defaultDomains, &DefaultDomain{
				Domain:     domain,
				Provider:   provider,
				SecretData: secret.Data,
			})
		}
	}

	return defaultDomains, nil
}

// GetInternalDomain finds the internal domain secret within the given map and returns the object
// that contains all relevant information about the internal domain.
func GetInternalDomain(secrets map[string]*corev1.Secret) (*InternalDomain, error) {
	internalDomainSecret, ok := secrets[common.GardenRoleInternalDomain]
	if !ok {
		return nil, fmt.Errorf("missing secret with key %s", common.GardenRoleInternalDomain)
	}

	provider, domain, err := common.GetDomainInfoFromAnnotations(internalDomainSecret.Annotations)
	if err != nil {
		return nil, fmt.Errorf("error getting information out of internal domain secret: %+v", err)
	}

	return &InternalDomain{
		Domain:     domain,
		Provider:   provider,
		SecretData: internalDomainSecret.Data,
	}, nil
}

// DomainIsDefaultDomain identifies whether a the given domain is a default domain.
func DomainIsDefaultDomain(domain string, defaultDomains []*DefaultDomain) *DefaultDomain {
	for _, defaultDomain := range defaultDomains {
		if strings.HasSuffix(domain, defaultDomain.Domain) {
			return defaultDomain
		}
	}
	return nil
}

// ReadGardenSecrets reads the Kubernetes Secrets from the Garden cluster which are independent of Shoot clusters.
// The Secret objects are stored on the Controller in order to pass them to created Garden objects later.
func ReadGardenSecrets(k8sInformers kubeinformers.SharedInformerFactory) (map[string]*corev1.Secret, error) {
	var (
		secretsMap                                  = make(map[string]*corev1.Secret)
		numberOfInternalDomainSecrets               = 0
		numberOfOpenVPNDiffieHellmanSecrets         = 0
		numberOfCertificateManagementConfigurations = 0
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
		// Retrieving default domain secrets based on all secrets in the Garden namespace which have
		// a label indicating the Garden role default-domain.
		if secret.Labels[common.GardenRole] == common.GardenRoleDefaultDomain {
			_, domain, err := common.GetDomainInfoFromAnnotations(secret.Annotations)
			if err != nil {
				logger.Logger.Warnf("error getting information out of default domain secret %s: %+v", secret.Name, err)
				continue
			}
			defaultDomainSecret := secret
			secretsMap[fmt.Sprintf("%s-%s", common.GardenRoleDefaultDomain, domain)] = defaultDomainSecret
			logger.Logger.Infof("Found default domain secret %s for domain %s.", secret.Name, domain)
		}

		// Retrieving internal domain secrets based on all secrets in the Garden namespace which have
		// a label indicating the Garden role internal-domain.
		if secret.Labels[common.GardenRole] == common.GardenRoleInternalDomain {
			_, domain, err := common.GetDomainInfoFromAnnotations(secret.Annotations)
			if err != nil {
				logger.Logger.Warnf("error getting information out of internal domain secret %s: %+v", secret.Name, err)
				continue
			}
			internalDomainSecret := secret
			secretsMap[common.GardenRoleInternalDomain] = internalDomainSecret
			logger.Logger.Infof("Found internal domain secret %s for domain %s.", secret.Name, domain)
			numberOfInternalDomainSecrets++
		}

		// Retrieving alerting SMTP secrets based on all secrets in the Garden namespace which have
		// a label indicating the Garden role alerting-smtp.
		// Only when using the in-cluster config as we do not want to configure alerts in development modus.
		if secret.Labels[common.GardenRole] == common.GardenRoleAlertingSMTP {
			alertingSMTP := secret
			secretsMap[fmt.Sprintf("%s-%s", common.GardenRoleAlertingSMTP, secret.Name)] = alertingSMTP
			logger.Logger.Infof("Found alerting SMTP secret %s.", secret.Name)
		}

		// Retrieving Diffie-Hellman secret for OpenVPN based on all secrets in the Garden namespace which have
		// a label indicating the Garden role openvpn-diffie-hellman.
		if secret.Labels[common.GardenRole] == common.GardenRoleOpenVPNDiffieHellman {
			openvpnDiffieHellman := secret
			key := "dh2048.pem"
			if _, ok := secret.Data[key]; !ok {
				return nil, fmt.Errorf("cannot use OpenVPN Diffie Hellman secret '%s' as it does not contain key '%s' (whose value should be the actual Diffie Hellman key)", secret.Name, key)
			}
			secretsMap[common.GardenRoleOpenVPNDiffieHellman] = openvpnDiffieHellman
			logger.Logger.Infof("Found OpenVPN Diffie Hellman secret %s.", secret.Name)
			numberOfOpenVPNDiffieHellmanSecrets++
		}

		if secret.Labels[common.GardenRole] == common.GardenRoleCertificateManagement {
			secretsMap[common.GardenRoleCertificateManagement] = secret
			logger.Logger.Infof("Found certificate management configuration %s.", secret.Name)
			numberOfCertificateManagementConfigurations++
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

	// For certificate management an instance of Cert-Manager will be deployed to the Seed cluster which requires a certain configuration.
	// This configuration is placed in the Garden cluster and must not be exist more than one time.
	if numberOfCertificateManagementConfigurations > 1 {
		return nil, fmt.Errorf("can only accept at most one certificate management configuration secret, but found %d", numberOfCertificateManagementConfigurations)
	}

	return secretsMap, nil
}

// VerifyInternalDomainSecret verifies that the internal domain secret matches to the internal domain secret used for
// existing Shoot clusters. It is not allowed to change the internal domain secret if there are existing Shoot clusters.
func VerifyInternalDomainSecret(k8sGardenClient kubernetes.Interface, numberOfShoots int, internalDomainSecret *corev1.Secret) error {
	_, currentDomain, err := common.GetDomainInfoFromAnnotations(internalDomainSecret.Annotations)
	if err != nil {
		return fmt.Errorf("error getting information out of current internal domain secret: %+v", err)
	}

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
func BootstrapCluster(k8sGardenClient kubernetes.Interface, gardenNamespace string, secrets map[string]*corev1.Secret) error {
	// Check whether the Kubernetes version of the Garden cluster is at least 1.10 (least supported K8s version of Gardener).
	minGardenVersion := "1.10"
	gardenVersionOK, err := utils.CompareVersions(k8sGardenClient.Version(), ">=", minGardenVersion)
	if err != nil {
		return err
	}
	if !gardenVersionOK {
		return fmt.Errorf("the Kubernetes version of the Garden cluster must be at least %s", minGardenVersion)
	}
	return nil
}
