// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"strings"

	"github.com/go-logr/logr"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/certificatesigningrequest"
)

const (
	// SeedCSRPrefix defines the prefix of seed CSR created by gardenlet.
	SeedCSRPrefix = "seed-csr-"
	// ShootCSRPrefix defines the prefix of self-hosted shoot CSR created by gardenlet.
	ShootCSRPrefix = "shoot-csr-"
)

// RequestKubeconfigWithBootstrapClient creates a kubeconfig with a signed certificate using the given bootstrap client
// returns the kubeconfig []byte representation, the CSR name, the seed name or an error.
func RequestKubeconfigWithBootstrapClient(
	ctx context.Context,
	log logr.Logger,
	runtimeClient client.Client,
	bootstrapClientSet kubernetes.Interface,
	kubeconfigKey, bootstrapKubeconfigKey client.ObjectKey,
	seedConfig *gardenletconfigv1alpha1.SeedConfig,
	selfHostedShootMeta *types.NamespacedName,
	validityDuration *metav1.Duration,
) (
	[]byte,
	string,
	error,
) {
	var (
		csrPrefix          string
		certificateSubject *pkix.Name
	)

	switch {
	case seedConfig != nil && seedConfig.Name != "":
		seedName := gardenletbootstraputil.GetSeedName(seedConfig)
		log = log.WithValues("seedName", seedName)

		csrPrefix = SeedCSRPrefix
		certificateSubject = &pkix.Name{
			Organization: []string{v1beta1constants.SeedsGroup},
			CommonName:   v1beta1constants.SeedUserNamePrefix + seedName,
		}

	case gardenlet.IsResponsibleForSelfHostedShoot():
		log = log.WithValues("shoot", selfHostedShootMeta)

		csrPrefix = ShootCSRPrefix
		certificateSubject = &pkix.Name{
			Organization: []string{v1beta1constants.ShootsGroup},
			CommonName:   v1beta1constants.ShootUserNamePrefix + selfHostedShootMeta.Namespace + ":" + selfHostedShootMeta.Name,
		}

	default:
		return nil, "", fmt.Errorf("failed determining gardenlet bootstrap scenario (seed or self-hosted shoot)")
	}

	certData, privateKeyData, csrName, err := certificatesigningrequest.RequestCertificate(ctx, log, bootstrapClientSet.Kubernetes(), certificateSubject, []string{}, []net.IP{}, validityDuration, csrPrefix)
	if err != nil {
		return nil, "", fmt.Errorf("unable to bootstrap the kubeconfig for the Garden cluster: %w", err)
	}

	log.Info("Storing kubeconfig with bootstrapped certificate in kubeconfig secret on target cluster")
	kubeconfig, err := gardenletbootstraputil.UpdateGardenKubeconfigSecret(ctx, bootstrapClientSet.RESTConfig(), certData, privateKeyData, runtimeClient, kubeconfigKey)
	if err != nil {
		return nil, "", fmt.Errorf("unable to update secret %q with bootstrapped kubeconfig: %w", kubeconfigKey.String(), err)
	}

	log.Info("Deleting bootstrap kubeconfig secret from target cluster")
	if err := kubernetesutils.DeleteObject(ctx, runtimeClient, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bootstrapKubeconfigKey.Name,
			Namespace: bootstrapKubeconfigKey.Namespace,
		},
	}); err != nil {
		return nil, "", err
	}
	return kubeconfig, csrName, nil
}

// DeleteBootstrapAuth checks which authentication mechanism was used to request a certificate
// (either a bootstrap token or a service account token was used). If the latter is true then it
// also deletes the corresponding ClusterRoleBinding.
func DeleteBootstrapAuth(ctx context.Context, reader client.Reader, writer client.Writer, csrName string) error {
	csr := &certificatesv1.CertificateSigningRequest{}
	if err := reader.Get(ctx, client.ObjectKey{Name: csrName}, csr); err != nil {
		return err
	}

	var resourcesToDelete []client.Object

	switch {
	case strings.HasPrefix(csr.Spec.Username, bootstraptokenapi.BootstrapUserPrefix):
		resourcesToDelete = append(resourcesToDelete,
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bootstraptokenapi.BootstrapTokenSecretPrefix + strings.TrimPrefix(csr.Spec.Username, bootstraptokenapi.BootstrapUserPrefix),
					Namespace: metav1.NamespaceSystem,
				},
			},
		)

	case strings.HasPrefix(csr.Spec.Username, serviceaccount.ServiceAccountUsernamePrefix):
		serviceAccountNamespace, serviceAccountName, err := serviceaccount.SplitUsername(csr.Spec.Username)
		if err != nil {
			return err
		}

		resourcesToDelete = append(resourcesToDelete,
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceAccountName,
					Namespace: serviceAccountNamespace,
				},
			},
			&rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: gardenletbootstraputil.ClusterRoleBindingName(serviceAccountNamespace, serviceAccountName),
				},
			},
		)
	}

	return kubernetesutils.DeleteObjects(ctx, writer, resourcesToDelete...)
}
