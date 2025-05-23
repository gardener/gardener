// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
)

// IdentityDeterminer determines the identity of the source cluster.
type IdentityDeterminer struct {
	Logger       logr.Logger
	SourceClient client.Client
	Config       *resourcemanagerconfigv1alpha1.ResourceManagerConfiguration
}

// Start determines the identity.
func (i *IdentityDeterminer) Start(ctx context.Context) error {
	if clusterID := ptr.Deref(i.Config.Controllers.ClusterID, ""); clusterID == "<cluster>" || clusterID == "<default>" {
		i.Logger.Info("Trying to get cluster id from cluster")

		id, err := i.determineClusterIdentity(ctx, clusterID == "<cluster>")
		if err != nil {
			return fmt.Errorf("unable to determine cluster id: %+v", err)
		}

		i.Config.Controllers.ClusterID = &id
	}

	i.Logger.Info("Using cluster id", "clusterID", *i.Config.Controllers.ClusterID)
	return nil
}

// determineClusterIdentity is used to extract the cluster identity from the cluster-identity
// config map. This is intended as fallback if no explicit cluster identity is given.
// in  seed-shoot scenario, the cluster id for the managed resources must be explicitly given
// to support the migration of a shoot from one seed to another. Here the identity `seed` should
// be set.
func (i *IdentityDeterminer) determineClusterIdentity(ctx context.Context, force bool) (string, error) {
	configMap := &corev1.ConfigMap{}
	if err := i.SourceClient.Get(ctx, client.ObjectKey{Name: v1beta1constants.ClusterIdentity, Namespace: metav1.NamespaceSystem}, configMap); err == nil {
		if id, ok := configMap.Data[v1beta1constants.ClusterIdentity]; ok {
			return id, nil
		}

		if force {
			return "", errors.New("cannot determine cluster identity from configmap: no cluster-identity entry")
		}
	} else if force || !apierrors.IsNotFound(err) {
		return "", fmt.Errorf("cannot determine cluster identity from configmap: %s", err)
	}

	return "", nil
}
