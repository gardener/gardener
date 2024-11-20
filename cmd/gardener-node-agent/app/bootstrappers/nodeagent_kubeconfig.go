// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	"k8s.io/client-go/rest"

	"github.com/gardener/gardener/pkg/nodeagent"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
)

// NodeAgentKubeconfig is a runnable to create the kubeconfig for the node-agent.
type NodeAgentKubeconfig struct {
	Log         logr.Logger
	FS          afero.Afero
	Cancel      context.CancelFunc
	Config      *rest.Config
	MachineName string
}

// Start performs creation of the gardener-node-agent kubeconfig.
func (n *NodeAgentKubeconfig) Start(ctx context.Context) error {
	ok, err := n.FS.Exists(nodeagentv1alpha1.KubeconfigFilePath)
	if err != nil {
		return fmt.Errorf("failed to check if kubeconfig file exists: %w", err)
	}
	if ok {
		n.Log.Info("Kubeconfig file exists, skipping bootstrap")
		return nil
	}

	n.Log.Info("Requesting kubeconfig for gardener-node agent")
	if err := nodeagent.RequestAndStoreKubeconfig(ctx, n.Log, n.FS, n.Config, n.MachineName); err != nil {
		return fmt.Errorf("failed to request and store kubeconfig: %w", err)
	}

	n.Log.Info("New kubeconfig written to disk successfully. Terminating gardener-node-agent")
	n.Cancel()

	return nil
}
