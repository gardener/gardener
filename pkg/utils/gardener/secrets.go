// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

var (
	// NoControlPlaneSecretsReq is a label selector requirement to select non-control plane secrets.
	NoControlPlaneSecretsReq = utils.MustNewRequirement(v1beta1constants.GardenRole, selection.NotIn, v1beta1constants.ControlPlaneSecretRoles...)
	// UncontrolledSecretSelector is a selector for objects which are managed by operators/users and not created by
	// Gardener controllers.
	UncontrolledSecretSelector = client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(NoControlPlaneSecretsReq)}
)

// FetchKubeconfigFromSecret tries to retrieve the kubeconfig bytes in given secret.
func FetchKubeconfigFromSecret(ctx context.Context, c client.Client, key client.ObjectKey) ([]byte, error) {
	secret := &corev1.Secret{}
	if err := c.Get(ctx, key, secret); err != nil {
		return nil, err
	}

	kubeconfig, ok := secret.Data[kubernetes.KubeConfig]
	if !ok || len(kubeconfig) == 0 {
		return nil, errors.New("the secret's field 'kubeconfig' is either not present or empty")
	}

	return kubeconfig, nil
}

// LabelPurposeGlobalMonitoringSecret is a constant for the value of the purpose label for replicated global monitoring
// secrets.
const LabelPurposeGlobalMonitoringSecret = "global-monitoring-secret-replica"

// ReplicateGlobalMonitoringSecret replicates the global monitoring secret into the given namespace and prefixes it with
// the given prefix.
func ReplicateGlobalMonitoringSecret(ctx context.Context, c client.Client, prefix, namespace string, globalMonitoringSecret *corev1.Secret) (*corev1.Secret, error) {
	globalMonitoringSecretReplica := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: prefix + globalMonitoringSecret.Name, Namespace: namespace}}
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, c, globalMonitoringSecretReplica, func() error {
		metav1.SetMetaDataLabel(&globalMonitoringSecretReplica.ObjectMeta, v1beta1constants.GardenerPurpose, LabelPurposeGlobalMonitoringSecret)

		globalMonitoringSecretReplica.Type = globalMonitoringSecret.Type
		globalMonitoringSecretReplica.Data = globalMonitoringSecret.Data
		globalMonitoringSecretReplica.Immutable = globalMonitoringSecret.Immutable

		if _, ok := globalMonitoringSecretReplica.Data[secretsutils.DataKeySHA1Auth]; !ok {
			globalMonitoringSecretReplica.Data[secretsutils.DataKeySHA1Auth] = utils.CreateSHA1Secret(globalMonitoringSecret.Data[secretsutils.DataKeyUserName], globalMonitoringSecret.Data[secretsutils.DataKeyPassword])
		}

		return nil
	})
	return globalMonitoringSecretReplica, err
}
