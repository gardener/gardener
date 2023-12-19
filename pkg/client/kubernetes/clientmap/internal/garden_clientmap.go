// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/gardener/tokenrequest"
)

// gardenClientMap is a ClientMap for requesting and storing clients for virtual gardens.
type gardenClientMap struct {
	clientmap.ClientMap
}

// NewGardenClientMap creates a new gardenClientMap with the given factory.
func NewGardenClientMap(log logr.Logger, factory *GardenClientSetFactory) clientmap.ClientMap {
	logger := log.WithValues("clientmap", "GardenClientMap")
	factory.log = logger
	return &gardenClientMap{
		ClientMap: NewGenericClientMap(factory, logger, clock.RealClock{}),
	}
}

// GardenClientSetFactory is a ClientSetFactory that can produce new ClientSets to virtual gardens.
type GardenClientSetFactory struct {
	// RuntimeClient is the runtime cluster client.
	RuntimeClient client.Client
	// ClientConnectionConfiguration is the configuration that will be used by created ClientSets.
	ClientConnectionConfig componentbaseconfig.ClientConnectionConfiguration
	// GardenNamespace is the namespace the virtual gardens run in. Defaults to `garden` if not set.
	GardenNamespace string

	// log is a logger for logging entries related to creating Garden ClientSets.
	log logr.Logger
}

// CalculateClientSetHash calculates a SHA256 hash of the kubeconfig in the 'gardener' secret in the Garden's Garden namespace.
func (f *GardenClientSetFactory) CalculateClientSetHash(ctx context.Context, k clientmap.ClientSetKey) (string, error) {
	_, hash, err := f.getSecretAndComputeHash(ctx, k)
	if err != nil {
		return "", err
	}

	return hash, nil
}

// NewClientSet creates a new ClientSet for a Garden cluster.
func (f *GardenClientSetFactory) NewClientSet(ctx context.Context, k clientmap.ClientSetKey) (kubernetes.Interface, string, error) {
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
		return nil, "", fmt.Errorf("token for virtual garden kubeconfig was not populated yet")
	}

	clientSet, err := NewClientFromSecretObject(kubeconfigSecret,
		kubernetes.WithClientConnectionOptions(f.ClientConnectionConfig),
		kubernetes.WithClientOptions(client.Options{Scheme: operatorclient.VirtualScheme}),
		kubernetes.WithDisabledCachedClient(),
	)
	if err != nil {
		return nil, "", err
	}

	return clientSet, hash, nil
}

func (f *GardenClientSetFactory) getSecretAndComputeHash(ctx context.Context, k clientmap.ClientSetKey) (*corev1.Secret, string, error) {
	_, ok := k.(GardenClientSetKey)
	if !ok {
		return nil, "", fmt.Errorf("unsupported ClientSetKey: expected %T got %T", GardenClientSetKey{}, k)
	}

	gardenNamespace := f.getGardenNamespace()

	kubeconfigSecret := &corev1.Secret{}
	if err := f.RuntimeClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace, Name: f.secretName(gardenNamespace)}, kubeconfigSecret); err != nil {
		return nil, "", err
	}

	return kubeconfigSecret, utils.ComputeSHA256Hex(kubeconfigSecret.Data[kubernetes.KubeConfig]), nil
}

func (f *GardenClientSetFactory) secretName(gardenNamespace string) string {
	secretName := v1beta1constants.SecretNameGardener

	// Prefer the internal host if available
	addr, err := LookupHost(fmt.Sprintf("virtual-garden-%s.%s.svc.cluster.local", v1beta1constants.DeploymentNameKubeAPIServer, gardenNamespace))
	if err != nil {
		f.log.Info("Service DNS name lookup of virtual-garden-kube-apiserver failed, falling back to external kubeconfig", "error", err)
	} else if len(addr) > 0 {
		secretName = v1beta1constants.SecretNameGardenerInternal
	}

	return secretName
}

var _ clientmap.Invalidate = &GardenClientSetFactory{}

// InvalidateClient invalidates information cached for the given ClientSetKey in the factory.
func (f *GardenClientSetFactory) InvalidateClient(k clientmap.ClientSetKey) error {
	_, ok := k.(GardenClientSetKey)
	if !ok {
		return fmt.Errorf("unsupported ClientSetKey: expected %T got %T", GardenClientSetKey{}, k)
	}
	return nil
}

func (f *GardenClientSetFactory) getGardenNamespace() string {
	if f.GardenNamespace == "" {
		return v1beta1constants.GardenNamespace
	}

	return f.GardenNamespace
}

// GardenClientSetKey is a ClientSetKey for a Garden cluster.
type GardenClientSetKey struct {
	Name string
}

// Key returns the string representation of the ClientSetKey.
func (k GardenClientSetKey) Key() string {
	return k.Name
}
