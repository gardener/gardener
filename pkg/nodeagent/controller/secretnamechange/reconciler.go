// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretnamechange

import (
	"context"
	"fmt"

	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	nodeagenthelper "github.com/gardener/gardener/pkg/api/config/nodeagent/v1alpha1/helper"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/nodeagent"
)

// Reconciler checks if the label containing the relevant OSC secret for this node is changed and restarts if so.
// For hosted shoots, this never happens: During the lifecycle of a node, the OSC secret name is fixed. For self-hosted
// shoots, this may happen after the initial connect of the Shoot to the garden cluster: `gardenadm init` may not have
// run with the final spec of the Shoot (after registering it with gardener-apiserver, the spec might be augmented via
// admission plugins or webhooks). Hence, when gardenlet reconciles for the first time, it might compute a different
// OSC secret name. Without this controller, gardener-node-agent would have no means to switch to this new secret since
// its component config dictates which OSC secret to watch.
// This controller reacts on changes of said label on the node. If a change is detected, it overwrites the secret name
// in the component config and cancels the context (which triggers a restart to re-read the new config).
type Reconciler struct {
	Client        client.Client
	ConfigDir     string
	FS            afero.Afero
	CancelContext context.CancelFunc
}

// Reconcile checks if the label containing the relevant OSC secret for this node is changed and restarts if so.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, request.NamespacedName, node); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	var (
		configPath        = nodeagenthelper.GetConfigFilePath(r.ConfigDir)
		desiredSecretName = node.Labels[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName]
	)

	configRaw, err := r.FS.ReadFile(configPath)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("error reading gardener-node-agent config file %q: %w", configPath, err)
	}

	config := &nodeagentconfigv1alpha1.NodeAgentConfiguration{}
	if err := runtime.DecodeInto(nodeagent.Codec, configRaw, config); err != nil {
		return reconcile.Result{}, fmt.Errorf("error decoding gardener-node-agent config: %w", err)
	}

	if config.Controllers.OperatingSystemConfig.SecretName == desiredSecretName {
		return reconcile.Result{}, nil
	}

	log.Info("The gardener-node-agent secret name in component config does not match node label - overwriting component config",
		"secretNameInConfig", config.Controllers.OperatingSystemConfig.SecretName,
		"configPath", configPath,
		"secretNameOnNode", desiredSecretName,
	)

	config.Controllers.OperatingSystemConfig.SecretName = desiredSecretName

	configRaw, err = runtime.Encode(nodeagent.Codec, config)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to encode component config: %w", err)
	}

	if err := r.FS.WriteFile(configPath, configRaw, 0600); err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to write current OSC to file path %q: %w", configPath, err)
	}

	log.Info("Component config has been overwritten, calling the cancel func to trigger a restart of gardener-node-agent")
	r.CancelContext()
	return reconcile.Result{}, nil
}
