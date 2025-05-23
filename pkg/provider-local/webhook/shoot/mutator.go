// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	kubeproxy "github.com/gardener/gardener/pkg/component/kubernetes/proxy"
)

type mutator struct {
	logger               logr.Logger
	kubeProxyConfigCodec kubeproxy.ConfigCodec
}

// NewMutator creates a new Mutator that mutates resources in the shoot cluster.
func NewMutator() extensionswebhook.Mutator {
	return &mutator{
		logger:               log.Log.WithName("shoot-mutator"),
		kubeProxyConfigCodec: kubeproxy.NewConfigCodec(),
	}
}

func (m *mutator) Mutate(ctx context.Context, new, _ client.Object) error {
	acc, err := meta.Accessor(new)
	if err != nil {
		return fmt.Errorf("could not create accessor during webhook: %w", err)
	}

	// If the object does have a deletion timestamp then we don't want to mutate anything.
	if acc.GetDeletionTimestamp() != nil {
		return nil
	}

	switch x := new.(type) {
	case *corev1.ConfigMap:
		switch {
		case strings.HasPrefix(x.Name, kubeproxy.ConfigNamePrefix):
			extensionswebhook.LogMutation(logger, x.Kind, x.Namespace, x.Name)
			return m.mutateKubeProxyConfigMap(ctx, x)
		}
	}

	return nil
}
