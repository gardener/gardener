// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package util

import (
	"context"
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/secrets"

	"github.com/Masterminds/semver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CAChecksumAnnotation is a resource annotation used to store the checksum of a certificate authority.
const CAChecksumAnnotation = "checksum/ca"

// GetOrCreateShootKubeconfig gets or creates a Kubeconfig for a Shoot cluster which has a running control plane in the given `namespace`.
// If the CA of an existing Kubeconfig has changed, it creates a new Kubeconfig.
// Newly generated Kubeconfigs are applied with the given `client` to the given `namespace`.
// Deprecated: Use gutil.NewShootAccessSecret instead.
func GetOrCreateShootKubeconfig(ctx context.Context, c client.Client, certificateConfig secrets.CertificateSecretConfig, namespace string) (*corev1.Secret, error) {
	caSecret, ca, err := secrets.LoadCAFromSecret(ctx, c, namespace, v1beta1constants.SecretNameCACluster)
	if err != nil {
		return nil, fmt.Errorf("error fetching CA secret %s/%s: %w", namespace, v1beta1constants.SecretNameCACluster, err)
	}

	var (
		secret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: make(map[string]string),
				Name:        certificateConfig.Name,
				Namespace:   namespace,
			},
		}
		key = types.NamespacedName{
			Name:      certificateConfig.Name,
			Namespace: namespace,
		}
	)
	if err := c.Get(ctx, key, &secret); client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("error preparing kubeconfig: %w", err)
	}

	var (
		computedChecksum   = utils.ComputeChecksum(caSecret.Data)
		storedChecksum, ok = secret.Annotations[CAChecksumAnnotation]
	)
	if ok && computedChecksum == storedChecksum {
		return &secret, nil
	}

	certificateConfig.SigningCA = ca
	certificateConfig.CertType = secrets.ClientCert

	config := secrets.ControlPlaneSecretConfig{
		CertificateSecretConfig: &certificateConfig,

		KubeConfigRequests: []secrets.KubeConfigRequest{{
			ClusterName:   namespace,
			APIServerHost: kubeAPIServerServiceDNS(namespace),
		}},
	}

	controlPlane, err := config.GenerateControlPlane()
	if err != nil {
		return nil, fmt.Errorf("error creating kubeconfig: %w", err)
	}

	_, err = controllerutils.GetAndCreateOrMergePatch(ctx, c, &secret, func() error {
		secret.Data = controlPlane.SecretData()
		if secret.Annotations == nil {
			secret.Annotations = make(map[string]string)
		}
		secret.Annotations[CAChecksumAnnotation] = computedChecksum
		return nil
	})

	return &secret, err
}

// kubeAPIServerServiceDNS returns a domain name which can be used to contact
// the Kube-Apiserver deployment of a Shoot within the Seed cluster.
// e.g. kube-apiserver.shoot--project--prod.svc.cluster.local.
func kubeAPIServerServiceDNS(namespace string) string {
	return fmt.Sprintf("%s.%s", v1beta1constants.DeploymentNameKubeAPIServer, namespace)
}

// VersionMajorMinor extracts and returns the major and the minor part of the given version (input must be a semantic version).
func VersionMajorMinor(version string) (string, error) {
	v, err := semver.NewVersion(version)
	if err != nil {
		return "", fmt.Errorf("Invalid version string '%s': %w", version, err)
	}
	return fmt.Sprintf("%d.%d", v.Major(), v.Minor()), nil
}

// VersionInfo converts the given version string to version.Info (input must be a semantic version).
func VersionInfo(vs string) (*version.Info, error) {
	v, err := semver.NewVersion(vs)
	if err != nil {
		return nil, fmt.Errorf("Invalid version string '%s': %w", vs, err)
	}
	return &version.Info{
		Major:      fmt.Sprintf("%d", v.Major()),
		Minor:      fmt.Sprintf("%d", v.Minor()),
		GitVersion: fmt.Sprintf("v%d.%d.%d", v.Major(), v.Minor(), v.Patch()),
	}, nil
}
