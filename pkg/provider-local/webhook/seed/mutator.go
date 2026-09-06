// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubeproxy "github.com/gardener/gardener/pkg/component/kubernetes/proxy"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const dataKeyConfig = "config.yaml"

type mutator struct {
	logger               logr.Logger
	kubeProxyConfigCodec kubeproxy.ConfigCodec
}

// NewMutator creates a new Mutator that disables the kube-proxy conntrack max configuration in the
// kube-proxy ManagedResource secret in the seed, before gardener-resource-manager applies it to the
// shoot. This is required on the local provider because the nested (Docker-in-Docker) CI environment
// does not allow writing to /proc/sys/net/netfilter/nf_conntrack_max, which kube-proxy >= v1.36
// treats as a fatal error.
func NewMutator() extensionswebhook.Mutator {
	return &mutator{
		logger:               log.Log.WithName("seed-mutator"),
		kubeProxyConfigCodec: kubeproxy.NewConfigCodec(),
	}
}

func (m *mutator) Mutate(_ context.Context, newObj, _ client.Object) error {
	acc, err := meta.Accessor(newObj)
	if err != nil {
		return fmt.Errorf("could not create accessor during webhook: %w", err)
	}

	// If the object does have a deletion timestamp then we don't want to mutate anything.
	if acc.GetDeletionTimestamp() != nil {
		return nil
	}

	secret, ok := newObj.(*corev1.Secret)
	if !ok {
		return fmt.Errorf("unexpected object, got %T wanted *corev1.Secret", newObj)
	}

	decoder := serializer.NewCodecFactory(kubernetes.ShootScheme).UniversalDeserializer()
	objects, err := managedresources.ExtractObjectsFromSecret(decoder, secret)
	if err != nil {
		return fmt.Errorf("could not extract objects from kube-proxy managed resource secret %q: %w", client.ObjectKeyFromObject(secret), err)
	}

	mutated := false
	for _, obj := range objects {
		configMap, ok := obj.(*corev1.ConfigMap)
		if !ok {
			continue
		}
		// The kube-proxy ConfigMap name is made unique (hash-suffixed) via MakeUnique, so match on the prefix.
		if !strings.HasPrefix(configMap.Name, kubeproxy.ConfigNamePrefix) {
			continue
		}
		if _, ok := configMap.Data[dataKeyConfig]; !ok {
			continue
		}

		if err := m.mutateKubeProxyConfigMap(configMap); err != nil {
			return fmt.Errorf("could not mutate kube-proxy ConfigMap %q: %w", configMap.Name, err)
		}
		mutated = true
	}

	if !mutated {
		return nil
	}

	extensionswebhook.LogMutation(m.logger, secret.Kind, secret.Namespace, secret.Name)

	// Prevent the secret from being stored as immutable. MakeUnique sets spec.Immutable=true,
	// which causes Kubernetes to reject CreateOrUpdate's subsequent UPDATE requests even after the
	// webhook correctly mutates the data. Clearing it lets every gardenlet reconcile keep
	// the correct conntrack value via the same webhook path.
	secret.Immutable = nil

	data, err := managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer).AddAllAndSerialize(objects...)
	if err != nil {
		return fmt.Errorf("could not re-serialize objects into kube-proxy managed resource secret %q: %w", client.ObjectKeyFromObject(secret), err)
	}
	secret.Data = data

	return nil
}
