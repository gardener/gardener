// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clientmap

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/gardener/tokenrequest"
)

// shootClientMap is a ClientMap for requesting and storing clients for Shoot clusters.
type shootClientMap struct {
	ClientMap
}

// NewShootClientMap creates a new shootClientMap with the given factory.
func NewShootClientMap(log logr.Logger, factory *ShootClientSetFactory) ClientMap {
	logger := log.WithValues("clientmap", "ShootClientMap")
	factory.clientKeyToSeedNamespace = make(map[ShootClientSetKey]string)
	factory.log = logger
	return &shootClientMap{
		ClientMap: NewGenericClientMap(factory, logger, clock.RealClock{}),
	}
}

// ShootClientSetFactory is a ClientSetFactory that can produce new ClientSets to Shoot clusters.
type ShootClientSetFactory struct {
	// GardenClient is the garden cluster client.
	GardenClient client.Client
	// SeedClient is the seed cluster client.
	SeedClient client.Client
	// ClientConnectionConfiguration is the configuration that will be used by created ClientSets.
	ClientConnectionConfig componentbaseconfigv1alpha1.ClientConnectionConfiguration

	// log is a logger for logging entries related to creating Shoot ClientSets.
	log logr.Logger

	clientKeyToSeedNamespace map[ShootClientSetKey]string
}

// CalculateClientSetHash calculates a SHA256 hash of the kubeconfig in the 'gardener' secret in the Shoot's Seed namespace.
func (f *ShootClientSetFactory) CalculateClientSetHash(ctx context.Context, k ClientSetKey) (string, error) {
	_, hash, err := f.getSecretAndComputeHash(ctx, k)
	if err != nil {
		return "", err
	}

	return hash, nil
}

// NewClientSet creates a new ClientSet for a Shoot cluster.
func (f *ShootClientSetFactory) NewClientSet(ctx context.Context, k ClientSetKey) (kubernetes.Interface, string, error) {
	kubeconfigSecret, hash, err := f.getSecretAndComputeHash(ctx, k)
	if err != nil {
		return nil, "", err
	}

	// Kubeconfig secrets are created with empty authinfo and it's expected that gardener-resource-manager eventually
	// populates a token, so let's check whether the read secret already contains authinfo
	tokenPopulated, err := tokenrequest.IsTokenPopulated(kubeconfigSecret)
	if err != nil {
		return nil, "", err
	}
	if !tokenPopulated {
		return nil, "", fmt.Errorf("token for shoot kubeconfig was not populated yet")
	}

	clientSet, err := NewClientFromSecretObject(kubeconfigSecret,
		kubernetes.WithClientConnectionOptions(f.ClientConnectionConfig),
		kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.ShootScheme}),
		kubernetes.WithDisabledCachedClient(),
	)
	if err != nil {
		return nil, "", err
	}

	return clientSet, hash, nil
}

func (f *ShootClientSetFactory) getSecretAndComputeHash(ctx context.Context, k ClientSetKey) (*corev1.Secret, string, error) {
	key, ok := k.(ShootClientSetKey)
	if !ok {
		return nil, "", fmt.Errorf("unsupported ClientSetKey: expected %T got %T", ShootClientSetKey{}, k)
	}

	seedNamespace, err := f.getSeedNamespace(ctx, key)
	if err != nil {
		return nil, "", err
	}

	kubeconfigSecret := &corev1.Secret{}
	if err := f.SeedClient.Get(ctx, client.ObjectKey{Namespace: seedNamespace, Name: f.secretName(seedNamespace)}, kubeconfigSecret); err != nil {
		return nil, "", err
	}

	return kubeconfigSecret, utils.ComputeSHA256Hex(kubeconfigSecret.Data[kubernetes.KubeConfig]), nil
}

func (f *ShootClientSetFactory) secretName(seedNamespace string) string {
	secretName := v1beta1constants.SecretNameGardener

	// If the gardenlet runs in the same cluster like the API server of the shoot then use the internal kubeconfig
	// and communicate internally. Otherwise, fall back to the "external" kubeconfig and communicate via the
	// load balancer of the shoot API server.
	addr, err := LookupHost(fmt.Sprintf("%s.%s.svc", v1beta1constants.DeploymentNameKubeAPIServer, seedNamespace))
	if err != nil {
		f.log.Info("Service DNS name lookup of kube-apiserver failed, falling back to external kubeconfig", "error", err)
	} else if len(addr) > 0 {
		secretName = v1beta1constants.SecretNameGardenerInternal
	}

	return secretName
}

var _ Invalidate = &ShootClientSetFactory{}

// InvalidateClient invalidates information cached for the given ClientSetKey in the factory.
func (f *ShootClientSetFactory) InvalidateClient(k ClientSetKey) error {
	key, ok := k.(ShootClientSetKey)
	if !ok {
		return fmt.Errorf("unsupported ClientSetKey: expected %T got %T", ShootClientSetKey{}, k)
	}
	delete(f.clientKeyToSeedNamespace, key)
	return nil
}

func (f *ShootClientSetFactory) seedNamespaceFromCache(key ShootClientSetKey) (string, error) {
	namespace, ok := f.clientKeyToSeedNamespace[key]
	if !ok {
		return "", fmt.Errorf("no seed info cached for client %s", key)
	}
	return namespace, nil
}

func (f *ShootClientSetFactory) seedNamespaceToCache(key ShootClientSetKey, namespace string) {
	f.clientKeyToSeedNamespace[key] = namespace
}

func (f *ShootClientSetFactory) getSeedNamespace(ctx context.Context, key ShootClientSetKey) (string, error) {
	if namespace, err := f.seedNamespaceFromCache(key); err == nil {
		return namespace, nil
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := f.GardenClient.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: key.Name}, shoot); err != nil {
		return "", fmt.Errorf("failed to get Shoot object %q: %w", key.Key(), err)
	}

	seedName := shoot.Spec.SeedName
	if seedName == nil {
		return "", fmt.Errorf("shoot %q is not scheduled yet", key.Key())
	}

	var namespace string
	if len(shoot.Status.TechnicalID) > 0 {
		namespace = shoot.Status.TechnicalID
	} else {
		project, err := ProjectForNamespaceFromReader(ctx, f.GardenClient, shoot.Namespace)
		if err != nil {
			return "", fmt.Errorf("failed to get Project for Shoot %q: %w", key.Key(), err)
		}
		namespace = gardenerutils.ComputeTechnicalID(project.Name, shoot)
	}

	f.seedNamespaceToCache(key, namespace)

	return namespace, nil
}

// ShootClientSetKey is a ClientSetKey for a Shoot cluster.
type ShootClientSetKey struct {
	Namespace, Name string
}

// Key returns the string representation of the ClientSetKey.
func (k ShootClientSetKey) Key() string {
	return k.Namespace + "/" + k.Name
}
