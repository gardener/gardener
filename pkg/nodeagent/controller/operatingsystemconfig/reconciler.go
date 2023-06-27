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

package operatingsystemconfig

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"time"

	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/executor"
	"github.com/gardener/gardener/pkg/controllerutils"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/controller/common"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// Reconciler fetches the shoot access token for gardener-node-agent and writes the token to disk.
type Reconciler struct {
	Client          client.Client
	Config          *nodeagentv1alpha1.NodeAgentConfiguration
	Recorder        record.EventRecorder
	SyncPeriod      time.Duration
	NodeName        string
	TriggerChannels []chan event.GenericEvent
	Dbus            dbus.Dbus
	Fs              afero.Fs
}

// Reconcile the operatingsystemconfig.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	log.Info("Reconciling")

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, request.NamespacedName, secret); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	osc, oscRaw, err := r.extractOSCFromSecret(secret)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to extract osc from secret: %w", err)
	}

	oscCheckSum := utils.ComputeSHA256Hex(oscRaw)

	if err := yaml.Unmarshal(oscRaw, osc); err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to unmarshal osc from secret data: %w", err)
	}

	oscChanges, err := common.CalculateChangedUnitsAndRemovedFiles(r.Fs, osc)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to calculate osc changes from previous run: %w", err)
	}

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: r.NodeName}, node); client.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, fmt.Errorf("unable to fetch node %w", err)
	}

	if node != nil && node.Annotations[executor.AnnotationKeyChecksum] == oscCheckSum {
		log.Info("node is up to date, osc did not change, returning")
		return reconcile.Result{}, nil
	}

	tmpDir, err := afero.TempDir(r.Fs, "/tmp", "gardener-node-agent-*")
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to create temp dir: %w", err)
	}

	for _, file := range osc.Spec.Files {
		if file.Content.Inline == nil {
			continue
		}

		if err := r.Fs.MkdirAll(filepath.Dir(file.Path), fs.ModeDir); err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to create directory %q: %w", file.Path, err)
		}

		perm := fs.FileMode(0600)
		if file.Permissions != nil {
			perm = fs.FileMode(*file.Permissions)
		}

		data, err := helper.Decode(file.Content.Inline.Encoding, []byte(file.Content.Inline.Data))
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to decode data of file %q: %w", file.Path, err)
		}

		tmpFilePath := filepath.Join(tmpDir, filepath.Base(file.Path))
		if err := afero.WriteFile(r.Fs, tmpFilePath, data, perm); err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to create file %q: %w", file.Path, err)
		}

		if err := r.Fs.Rename(tmpFilePath, file.Path); err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to move temporary file to %q: %w", file.Path, err)
		}
	}

	for _, changedUnit := range oscChanges.ChangedUnits {
		if changedUnit.Content == nil {
			continue
		}

		systemdUnitFilePath := path.Join("/etc/systemd/system", changedUnit.Name)

		existingUnitContent, err := afero.ReadFile(r.Fs, systemdUnitFilePath)
		if err != nil && !errors.Is(err, afero.ErrFileNotFound) {
			return reconcile.Result{}, fmt.Errorf("unable to read systemd unit %q: %w", changedUnit.Name, err)
		}

		newUnitContent := []byte(*changedUnit.Content)
		if bytes.Equal(newUnitContent, existingUnitContent) {
			continue
		}

		if err := afero.WriteFile(r.Fs, systemdUnitFilePath, newUnitContent, 0600); err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to write unit %q: %w", changedUnit.Name, err)
		}

		if changedUnit.Enable != nil {
			if *changedUnit.Enable {
				if err := r.Dbus.Enable(ctx, changedUnit.Name); err != nil {
					return reconcile.Result{}, fmt.Errorf("unable to enable unit %q: %w", changedUnit.Name, err)
				}
			} else {
				if err := r.Dbus.Disable(ctx, changedUnit.Name); err != nil {
					return reconcile.Result{}, fmt.Errorf("unable to disable unit %q: %w", changedUnit.Name, err)
				}
			}
		}
		log.Info("processed writing unit", "name", changedUnit.Name, "command", pointer.StringDeref(changedUnit.Command, ""))
	}

	for _, deletedUnit := range oscChanges.DeletedUnits {
		if err := r.Dbus.Stop(ctx, r.Recorder, node, deletedUnit.Name); err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to stop deleted unit %q: %w", deletedUnit.Name, err)
		}

		if err := r.Dbus.Disable(ctx, deletedUnit.Name); err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to disable deleted unit %q: %w", deletedUnit.Name, err)
		}

		if err := r.Fs.Remove(path.Join("/etc/systemd/system", deletedUnit.Name)); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
			return reconcile.Result{}, fmt.Errorf("unable to delete systemd unit of deleted %q: %w", deletedUnit.Name, err)
		}
	}

	if err := r.Dbus.DaemonReload(ctx); err != nil {
		return reconcile.Result{}, err
	}

	var fns []flow.TaskFn
	for _, u := range oscChanges.ChangedUnits {
		changedUnit := u
		if changedUnit.Content == nil {
			continue
		}

		// There are some units without a command specified,
		// the old bash based implementation didn't care about command and always restarted all units
		// mimic this behavior at least for these units.
		if changedUnit.Command == nil {
			changedUnit.Command = pointer.String("start")
		}

		fns = append(fns, func(ctx context.Context) error {
			switch *changedUnit.Command {
			// TODO(@majst01): make this accessible constants
			case "start", "restart":
				if err := r.Dbus.Restart(ctx, r.Recorder, node, changedUnit.Name); err != nil {
					return fmt.Errorf("unable to restart %q: %w", changedUnit.Name, err)
				}

			case "stop":
				if err := r.Dbus.Stop(ctx, r.Recorder, node, changedUnit.Name); err != nil {
					return fmt.Errorf("unable to stop %q: %w", changedUnit.Name, err)
				}
			}

			return nil
		})
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := flow.Sequential(fns...)(timeoutCtx); err != nil {
		return reconcile.Result{}, fmt.Errorf("error ensuring states of systemd units: %w", err)
	}

	var deletionErrors []error
	for _, f := range oscChanges.DeletedFiles {
		if err := r.Fs.Remove(f.Path); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
			deletionErrors = append(deletionErrors, err)
		}
	}
	if len(deletionErrors) > 0 {
		return reconcile.Result{}, fmt.Errorf("unable to delete all files which must not exist anymore: %w", errors.Join(deletionErrors...))
	}

	// Persist current OSC for comparison with next one
	if err := afero.WriteFile(r.Fs, nodeagentv1alpha1.NodeAgentOSCOldConfigPath, oscRaw, 0644); err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to write previous osc to file %w", err)
	}

	log.Info("Successfully processed operating system configs", "files", len(osc.Spec.Files), "units", len(osc.Spec.Units))

	// notifying other controllers about possible change in applied files (e.g. configuration.yaml)
	for _, c := range r.TriggerChannels {
		c <- event.GenericEvent{}
	}

	r.Recorder.Event(node, corev1.EventTypeNormal, "OSCApplied", "all osc files and units have been applied successfully")

	if node == nil || node.Name == "" || node.Annotations == nil {
		return reconcile.Result{RequeueAfter: 10 * time.Second}, fmt.Errorf("still waiting for node to get registered")
	}

	node.Annotations[v1beta1constants.LabelWorkerKubernetesVersion] = r.Config.KubernetesVersion
	node.Annotations[executor.AnnotationKeyChecksum] = oscCheckSum

	if err := r.Client.Update(ctx, node); err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to set node annotation %w", err)
	}

	log.V(1).Info("Requeuing", "requeueAfter", r.SyncPeriod)
	return reconcile.Result{RequeueAfter: r.SyncPeriod}, nil
}

func (r *Reconciler) extractOSCFromSecret(secret *corev1.Secret) (*extensionsv1alpha1.OperatingSystemConfig, []byte, error) {
	oscRaw, ok := secret.Data[nodeagentv1alpha1.NodeAgentOSCSecretKey]
	if !ok {
		return nil, nil, fmt.Errorf("no token found in secret")
	}

	osc := &extensionsv1alpha1.OperatingSystemConfig{}
	if err := yaml.Unmarshal(oscRaw, osc); err != nil {
		return nil, nil, fmt.Errorf("unable to unmarshal osc from secret data %w", err)
	}

	return osc, oscRaw, nil
}
