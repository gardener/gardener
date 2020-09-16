// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"context"
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/secrets"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// CAChecksumAnnotation is a resource annotation used to store the checksum of a certificate authority.
const CAChecksumAnnotation = "checksum/ca"

// GetOrCreateShootKubeconfig gets or creates a Kubeconfig for a Shoot cluster which has a running control plane in the given `namespace`.
// If the CA of an existing Kubeconfig has changed, it creates a new Kubeconfig.
// Newly generated Kubeconfigs are applied with the given `client` to the given `namespace`.
func GetOrCreateShootKubeconfig(ctx context.Context, c client.Client, certificateConfig secrets.CertificateSecretConfig, namespace string) (*corev1.Secret, error) {
	caSecret, ca, err := secrets.LoadCAFromSecret(c, namespace, v1beta1constants.SecretNameCACluster)
	if err != nil {
		return nil, fmt.Errorf("error fetching CA secret %s/%s: %v", namespace, v1beta1constants.SecretNameCACluster, err)
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
		return nil, fmt.Errorf("error preparing kubeconfig: %v", err)
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

		KubeConfigRequest: &secrets.KubeConfigRequest{
			ClusterName:  namespace,
			APIServerURL: kubeAPIServerServiceDNS(namespace),
		},
	}

	controlPlane, err := config.GenerateControlPlane()
	if err != nil {
		return nil, fmt.Errorf("error creating kubeconfig: %v", err)
	}

	_, err = controllerutil.CreateOrUpdate(ctx, c, &secret, func() error {
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
		return "", errors.Wrapf(err, "Invalid version string '%s'", version)
	}
	return fmt.Sprintf("%d.%d", v.Major(), v.Minor()), nil
}

// VersionInfo converts the given version string to version.Info (input must be a semantic version).
func VersionInfo(vs string) (*version.Info, error) {
	v, err := semver.NewVersion(vs)
	if err != nil {
		return nil, errors.Wrapf(err, "Invalid version string '%s'", vs)
	}
	return &version.Info{
		Major:      fmt.Sprintf("%d", v.Major()),
		Minor:      fmt.Sprintf("%d", v.Minor()),
		GitVersion: fmt.Sprintf("v%d.%d.%d", v.Major(), v.Minor(), v.Patch()),
	}, nil
}
