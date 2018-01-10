// Copyright 2018 The Gardener Authors.
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

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// New creates a new Garden object (based on a Shoot object).
func New(shoot *gardenv1beta1.Shoot) *Garden {
	projectName := shoot.Namespace
	if strings.HasPrefix(shoot.Namespace, common.ProjectPrefix) {
		projectName = strings.SplitAfterN(shoot.Namespace, common.ProjectPrefix, 2)[1]
	}

	return &Garden{
		ProjectName: projectName,
	}
}

// ReadGardenSecrets reads the Kubernetes Secrets from the Garden cluster which are independent of Shoot clusters.
// The Secret objects are stored on the Controller in order to pass them to created Garden objects later.
func ReadGardenSecrets(k8sGardenClient kubernetes.Client, gardenNamespace string, runningInCluster bool) (map[string]*corev1.Secret, error) {
	var (
		secretsMap                    = make(map[string]*corev1.Secret)
		numberOfInternalDomainSecrets = 0
	)

	secrets, err := k8sGardenClient.ListSecrets(gardenNamespace, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, secret := range secrets.Items {
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
			secretsMap[fmt.Sprintf("%s-%s", common.GardenRoleDefaultDomain, domain)] = &defaultDomainSecret
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
			secretsMap[common.GardenRoleInternalDomain] = &internalDomainSecret
			logger.Logger.Infof("Found internal domain secret %s for domain %s.", name, domain)
			numberOfInternalDomainSecrets++
		}

		// Retrieving image pull secrets based on all secrets in the Garden namespace which have
		// a label indicating the Garden role image-pull.
		if labels[common.GardenRole] == common.GardenRoleImagePull {
			imagePull := secret
			secretsMap[fmt.Sprintf("%s-%s", common.GardenRoleImagePull, name)] = &imagePull
			logger.Logger.Infof("Found image pull secret %s.", name)
		}

		// Retrieving alerting SMTP secrets based on all secrets in the Garden namespace which have
		// a label indicating the Garden role alerting-smtp.
		// Only when using the in-cluster config as we do not want to configure alerts in development modus.
		if labels[common.GardenRole] == common.GardenRoleAlertingSMTP && runningInCluster {
			alertingSMTP := secret
			secretsMap[fmt.Sprintf("%s-%s", common.GardenRoleAlertingSMTP, name)] = &alertingSMTP
			logger.Logger.Infof("Found alerting SMTP secret %s.", name)
		}
	}

	if numberOfInternalDomainSecrets != 1 {
		return nil, fmt.Errorf("require exactly ONE internal domain secret, but found %d", numberOfInternalDomainSecrets)
	}

	return secretsMap, nil
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

	return common.EnsureImagePullSecrets(k8sGardenClient, gardenNamespace, secrets, false, nil)
}
