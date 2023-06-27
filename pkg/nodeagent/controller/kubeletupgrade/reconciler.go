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

package kubeletupgrade

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

	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/controllerutils"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/controller/common"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	"github.com/gardener/gardener/pkg/nodeagent/registry"
)

// hyperkubeImageDownloadedPath specifies a file which contains the hyperkube image ref, e.g. which version should be installed
// this is stored on disk to survive gardener-node-agent restarts
var hyperkubeImageDownloadedPath = path.Join(nodeagentv1alpha1.NodeAgentBaseDir, "hyperkube-downloaded")

// Reconciler checks which kubelet must be downloaded and restarts the kubelet if a change was detected
type Reconciler struct {
	Client           client.Client
	Config           *nodeagentv1alpha1.NodeAgentConfiguration
	Recorder         record.EventRecorder
	SyncPeriod       time.Duration
	TargetBinaryPath string
	NodeName         string
	TriggerChannel   <-chan event.GenericEvent
	Dbus             dbus.Dbus
	Fs               afero.Fs
	Extractor        registry.Extractor
}

// Reconcile checks which kubelet must be downloaded and restarts the kubelet if a change was detected
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	config, err := common.ReadNodeAgentConfiguration(r.Fs)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to update node agent config: %w", err)
	}
	r.Config = config

	imageRefDownloaded, err := common.ReadTrimmedFile(r.Fs, hyperkubeImageDownloadedPath)
	if err != nil && !os.IsNotExist(err) {
		return reconcile.Result{}, err
	}

	if r.Config.HyperkubeImage == imageRefDownloaded {
		log.Info("Desired kubelet binary hasn't changed, checking again later", "requeueAfter", r.SyncPeriod)
		return reconcile.Result{RequeueAfter: r.SyncPeriod}, nil
	}

	log.Info("kubelet binary has changed, starting kubelet update", "imageRef", r.Config.HyperkubeImage)

	if err := r.Extractor.ExtractFromLayer(r.Config.HyperkubeImage, "kubelet", r.TargetBinaryPath); err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to extract binary from image: %w", err)
	}

	log.Info("Successfully downloaded new kubelet binary", "imageRef", r.Config.HyperkubeImage)

	if err := r.Fs.MkdirAll(nodeagentv1alpha1.NodeAgentBaseDir, fs.ModeDir); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed creating node agent directory: %w", err)
	}

	// Save most recently downloaded image ref
	if err := afero.WriteFile(r.Fs, hyperkubeImageDownloadedPath, []byte(r.Config.HyperkubeImage), 0600); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed writing downloaded image ref: %w", err)
	}

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: r.NodeName}, node); client.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, fmt.Errorf("unable to fetch node: %w", err)
	}

	log.Info("Restarting kubelet unit")
	if err := r.Dbus.Restart(ctx, r.Recorder, node, kubelet.UnitName); err != nil {
		return reconcile.Result{}, fmt.Errorf("unable restart service: %w", err)
	}

	log.V(1).Info("Requeuing", "requeueAfter", r.SyncPeriod)
	return reconcile.Result{RequeueAfter: r.SyncPeriod}, nil
}
