// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package selfupgrade

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"time"

	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/controllerutils"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/controller/common"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	"github.com/gardener/gardener/pkg/nodeagent/registry"
)

// imageDownloadedPath specifies a file which contains the gardener-node-agent image ref, e.g. which version should be installed
// this is stored on disk to survive gardener-node-agent restarts
var imageDownloadedPath = path.Join(nodeagentv1alpha1.NodeAgentBaseDir, "node-agent-downloaded")

// Reconciler check if the version of the gardener-node-agent changed and updates itself if change was detected
type Reconciler struct {
	Client         client.Client
	Config         *nodeagentv1alpha1.NodeAgentConfiguration
	Recorder       record.EventRecorder
	SyncPeriod     time.Duration
	SelfBinaryPath string
	NodeName       string
	TriggerChannel <-chan event.GenericEvent
	Dbus           dbus.Dbus
	Fs             afero.Fs
	Extractor      registry.Extractor
}

// Reconcile check if the version of the gardener-node-agent changed and updates itself if change was detected
func (r *Reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	config, err := common.ReadNodeAgentConfiguration(r.Fs)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to update node agent config: %w", err)
	}
	r.Config = config

	imageRefDownloaded, err := common.ReadTrimmedFile(r.Fs, imageDownloadedPath)
	if err != nil && !os.IsNotExist(err) {
		return reconcile.Result{}, err
	}

	if r.Config.Image == imageRefDownloaded {
		log.Info("Desired gardener-node-agent binary hasn't changed, checking again later", "requeueAfter", r.SyncPeriod)
		return reconcile.Result{RequeueAfter: r.SyncPeriod}, nil
	}

	log.Info("gardener-node-agent binary has changed, starting self-update", "imageRef", r.Config.Image)

	if err := r.Extractor.ExtractFromLayer(r.Config.Image, "gardener-node-agent", r.SelfBinaryPath); err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to extract binary from image: %w", err)
	}

	log.Info("Successfully downloaded new gardener-node-agent binary", "imageRef", r.Config.Image)

	if err := r.Fs.MkdirAll(nodeagentv1alpha1.NodeAgentBaseDir, fs.ModeDir); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed creating node agent directory: %w", err)
	}

	// Save most recently downloaded image ref
	if err := afero.WriteFile(r.Fs, imageDownloadedPath, []byte(r.Config.Image), 0600); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed writing downloaded image ref: %w", err)
	}

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: r.NodeName}, node); client.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, fmt.Errorf("unable to fetch node: %w", err)
	}

	log.Info("Restarting own gardener-node-agent unit")
	if err := r.Dbus.Restart(ctx, r.Recorder, node, nodeagentv1alpha1.NodeAgentUnitName); err != nil {
		return reconcile.Result{}, fmt.Errorf("unable restart service: %w", err)
	}

	log.V(1).Info("Requeuing", "requeueAfter", r.SyncPeriod)
	return reconcile.Result{RequeueAfter: r.SyncPeriod}, nil
}
