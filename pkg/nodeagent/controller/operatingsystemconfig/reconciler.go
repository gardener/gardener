// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/nodeagent"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	filespkg "github.com/gardener/gardener/pkg/nodeagent/files"
	"github.com/gardener/gardener/pkg/nodeagent/registry"
	"github.com/gardener/gardener/pkg/utils/flow"
)

const (
	lastAppliedOperatingSystemConfigFilePath         = nodeagentconfigv1alpha1.BaseDir + "/last-applied-osc.yaml"
	lastComputedOperatingSystemConfigChangesFilePath = nodeagentconfigv1alpha1.BaseDir + "/last-computed-osc-changes.yaml"
)

var codec runtime.Codec

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(extensionsv1alpha1.AddToScheme(scheme))
	ser := jsonserializer.NewSerializerWithOptions(jsonserializer.DefaultMetaFactory, scheme, scheme, jsonserializer.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
	versions := schema.GroupVersions([]schema.GroupVersion{nodeagentconfigv1alpha1.SchemeGroupVersion, extensionsv1alpha1.SchemeGroupVersion})
	codec = serializer.NewCodecFactory(scheme).CodecForVersions(ser, ser, versions, versions)
}

// Reconciler decodes the OperatingSystemConfig resources from secrets and applies the systemd units and files to the
// node.
type Reconciler struct {
	Client                 client.Client
	Config                 nodeagentconfigv1alpha1.OperatingSystemConfigControllerConfig
	TokenSecretSyncConfigs []nodeagentconfigv1alpha1.TokenSecretSyncConfig
	Channel                chan event.TypedGenericEvent[*corev1.Secret]
	Recorder               record.EventRecorder
	DBus                   dbus.DBus
	FS                     afero.Afero
	Extractor              registry.Extractor
	CancelContext          context.CancelFunc
	HostName               string
	NodeName               string
	MachineName            string
}

// Reconcile decodes the OperatingSystemConfig resources from secrets and applies the systemd units and files to the
// node.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

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

	node, nodeCreated, err := r.getNode(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed getting node: %w", err)
	}

	if nodeCreated {
		log.Info("Node registered by kubelet. Restarting myself (gardener-node-agent unit) to start lease controller and watch my own node only. Canceling the context to initiate graceful shutdown")
		r.CancelContext()
		return reconcile.Result{}, nil
	}

	osc, oscChecksum, err := extractOSCFromSecret(secret)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed extracting OSC from secret: %w", err)
	}

	log.Info("Applying containerd configuration")
	if err := r.ReconcileContainerdConfig(ctx, log, osc); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed reconciling containerd configuration: %w", err)
	}

	oscChanges, err := computeOperatingSystemConfigChanges(log, r.FS, osc, oscChecksum)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed calculating the OSC changes: %w", err)
	}

	if node != nil && node.Annotations[nodeagentconfigv1alpha1.AnnotationKeyChecksumAppliedOperatingSystemConfig] == oscChecksum {
		log.Info("Configuration on this node is up to date, nothing to be done")
		return reconcile.Result{}, nil
	}

	log.Info("Applying new or changed inline files")
	if err := r.applyChangedInlineFiles(log, oscChanges); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed applying changed inline files: %w", err)
	}

	log.Info("Applying containerd registries")
	waitForRegistries, err := r.ReconcileContainerdRegistries(ctx, log, oscChanges)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed reconciling containerd registries: %w", err)
	}

	log.Info("Applying new or changed imageRef files")
	if err := r.applyChangedImageRefFiles(ctx, log, oscChanges); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed applying changed imageRef files: %w", err)
	}

	log.Info("Applying new or changed units", "changedUnits", len(oscChanges.Units.Changed))
	if err := r.applyChangedUnits(ctx, log, oscChanges); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed applying changed units: %w", err)
	}

	log.Info("Removing no longer needed units", "deletedUnits", len(oscChanges.Units.Deleted))
	if err := r.removeDeletedUnits(ctx, log, node, oscChanges); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed removing deleted units: %w", err)
	}

	log.Info("Reloading systemd daemon")
	if err := r.DBus.DaemonReload(ctx); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed reloading systemd daemon: %w", err)
	}

	log.Info("Executing unit commands (start/stop)", "unitCommands", len(oscChanges.Units.Commands))
	if err := r.executeUnitCommands(ctx, log, node, oscChanges); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed executing unit commands: %w", err)
	}

	// After the node is prepared, we can wait for the registries to be configured.
	// The ones with readiness probes should also succeed here since their cache/mirror pods
	// can now start as workload in the cluster.
	log.Info("Waiting for containerd registries to be configured")
	if err := waitForRegistries(); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed configuring containerd registries: %w", err)
	}

	log.Info("Removing no longer needed files")
	if err := r.removeDeletedFiles(log, oscChanges); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed removing deleted files: %w", err)
	}

	log.Info("Successfully applied operating system config")

	log.Info("Persisting current operating system config as 'last-applied' file to the disk", "path", lastAppliedOperatingSystemConfigFilePath)
	oscRaw, err := runtime.Encode(codec, osc)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to encode OSC: %w", err)
	}

	if err := r.FS.WriteFile(lastAppliedOperatingSystemConfigFilePath, oscRaw, 0600); err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to write current OSC to file path %q: %w", lastAppliedOperatingSystemConfigFilePath, err)
	}

	if oscChanges.MustRestartNodeAgent {
		log.Info("Must restart myself (gardener-node-agent unit), canceling the context to initiate graceful shutdown")
		if err := oscChanges.setMustRestartNodeAgent(false); err != nil {
			return reconcile.Result{}, err
		}
		r.CancelContext()
		return reconcile.Result{}, nil
	}

	if node == nil {
		log.Info("Waiting for Node to get registered by kubelet, requeuing")
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}

	log.Info("Deleting kubelet bootstrap kubeconfig file (in case it still exists)")
	if err := r.FS.Remove(kubelet.PathKubeconfigBootstrap); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
		return reconcile.Result{}, fmt.Errorf("failed removing kubelet bootstrap kubeconfig file %q: %w", kubelet.PathKubeconfigBootstrap, err)
	}
	if err := r.FS.Remove(nodeagentconfigv1alpha1.BootstrapTokenFilePath); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
		return reconcile.Result{}, fmt.Errorf("failed removing bootstrap token file %q: %w", nodeagentconfigv1alpha1.BootstrapTokenFilePath, err)
	}

	r.Recorder.Event(node, corev1.EventTypeNormal, "OSCApplied", "Operating system config has been applied successfully")
	patch := client.MergeFrom(node.DeepCopy())
	metav1.SetMetaDataLabel(&node.ObjectMeta, v1beta1constants.LabelWorkerKubernetesVersion, r.Config.KubernetesVersion.String())
	metav1.SetMetaDataAnnotation(&node.ObjectMeta, nodeagentconfigv1alpha1.AnnotationKeyChecksumAppliedOperatingSystemConfig, oscChecksum)

	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, r.Client.Patch(ctx, node, patch)
}

func (r *Reconciler) getNode(ctx context.Context) (*corev1.Node, bool, error) {
	if r.NodeName != "" {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: r.NodeName}}
		if err := r.Client.Get(ctx, client.ObjectKeyFromObject(node), node); err != nil {
			return nil, false, fmt.Errorf("unable to fetch node %q: %w", r.NodeName, err)
		}
		return node, false, nil
	}

	node, err := nodeagent.FetchNodeByHostName(ctx, r.Client, r.HostName)
	if err != nil {
		return nil, false, err
	}

	var nodeCreated bool
	if node != nil {
		r.NodeName = node.Name
		nodeCreated = true
	}

	return node, nodeCreated, nil
}

var (
	etcSystemdSystem                   = path.Join("/", "etc", "systemd", "system")
	defaultFilePermissions os.FileMode = 0600
	defaultDirPermissions  os.FileMode = 0755
)

func getFilePermissions(file extensionsv1alpha1.File) os.FileMode {
	permissions := defaultFilePermissions
	if file.Permissions != nil {
		permissions = fs.FileMode(*file.Permissions)
	}
	return permissions
}

func (r *Reconciler) applyChangedImageRefFiles(ctx context.Context, log logr.Logger, changes *operatingSystemConfigChanges) error {
	for _, file := range slices.Clone(changes.Files.Changed) {
		if file.Content.ImageRef == nil {
			continue
		}

		if err := r.Extractor.CopyFromImage(ctx, file.Content.ImageRef.Image, file.Content.ImageRef.FilePathInImage, file.Path, getFilePermissions(file)); err != nil {
			return fmt.Errorf("unable to copy file %q from image %q to %q: %w", file.Content.ImageRef.FilePathInImage, file.Content.ImageRef.Image, file.Path, err)
		}

		log.Info("Successfully applied new or changed file from image", "path", file.Path, "image", file.Content.ImageRef.Image)
		if err := changes.completedFileChanged(file.Path); err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) applyChangedInlineFiles(log logr.Logger, changes *operatingSystemConfigChanges) error {
	tmpDir, err := r.FS.TempDir(nodeagentconfigv1alpha1.TempDir, "osc-reconciliation-file-")
	if err != nil {
		return fmt.Errorf("unable to create temporary directory: %w", err)
	}

	defer func() { utilruntime.HandleError(r.FS.RemoveAll(tmpDir)) }()

	for _, file := range slices.Clone(changes.Files.Changed) {
		if file.Content.Inline == nil {
			continue
		}

		if err := r.FS.MkdirAll(filepath.Dir(file.Path), defaultDirPermissions); err != nil {
			return fmt.Errorf("unable to create directory %q: %w", file.Path, err)
		}

		data, err := extensionsv1alpha1helper.Decode(file.Content.Inline.Encoding, []byte(file.Content.Inline.Data))
		if err != nil {
			return fmt.Errorf("unable to decode data of file %q: %w", file.Path, err)
		}

		tmpFilePath := filepath.Join(tmpDir, filepath.Base(file.Path))
		if err := r.FS.WriteFile(tmpFilePath, data, getFilePermissions(file)); err != nil {
			return fmt.Errorf("unable to create temporary file %q: %w", tmpFilePath, err)
		}

		if err := filespkg.Move(r.FS, tmpFilePath, file.Path); err != nil {
			return fmt.Errorf("unable to rename temporary file %q to %q: %w", tmpFilePath, file.Path, err)
		}

		log.Info("Successfully applied new or changed file", "path", file.Path)
		if err := changes.completedFileChanged(file.Path); err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) removeDeletedFiles(log logr.Logger, changes *operatingSystemConfigChanges) error {
	for _, file := range slices.Clone(changes.Files.Deleted) {
		if err := r.FS.Remove(file.Path); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
			return fmt.Errorf("unable to delete no longer needed file %q: %w", file.Path, err)
		}

		log.Info("Successfully removed no longer needed file", "path", file.Path)
		if err := changes.completedFileDeleted(file.Path); err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) applyChangedUnits(ctx context.Context, log logr.Logger, changes *operatingSystemConfigChanges) error {
	for _, unit := range slices.Clone(changes.Units.Changed) {
		unitFilePath := path.Join(etcSystemdSystem, unit.Name)

		if unit.Content != nil {
			oldUnitContent, err := r.FS.ReadFile(unitFilePath)
			if err != nil && !errors.Is(err, afero.ErrFileNotFound) {
				return fmt.Errorf("unable to read existing unit file %q for %q: %w", unitFilePath, unit.Name, err)
			}

			newUnitContent := []byte(*unit.Content)
			if !bytes.Equal(newUnitContent, oldUnitContent) {
				if err := r.FS.WriteFile(unitFilePath, newUnitContent, defaultFilePermissions); err != nil {
					return fmt.Errorf("unable to write unit file %q for %q: %w", unitFilePath, unit.Name, err)
				}
				log.Info("Successfully applied new or changed unit file", "path", unitFilePath)
			}

			// ensure file permissions are restored in case somebody changed them manually
			if err := r.FS.Chmod(unitFilePath, defaultFilePermissions); err != nil {
				return fmt.Errorf("unable to ensure permissions for unit file %q for %q: %w", unitFilePath, unit.Name, err)
			}
		}

		dropInDirectory := unitFilePath + ".d"

		if len(unit.DropIns) == 0 {
			if err := r.FS.RemoveAll(dropInDirectory); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
				return fmt.Errorf("unable to delete systemd drop-in folder for unit %q: %w", unit.Name, err)
			}
		} else {
			if err := r.FS.MkdirAll(dropInDirectory, defaultDirPermissions); err != nil {
				return fmt.Errorf("unable to create drop-in directory %q for unit %q: %w", dropInDirectory, unit.Name, err)
			}

			for _, dropIn := range slices.Clone(unit.DropInsChanges.Changed) {
				dropInFilePath := path.Join(dropInDirectory, dropIn.Name)

				oldDropInContent, err := r.FS.ReadFile(dropInFilePath)
				if err != nil && !errors.Is(err, afero.ErrFileNotFound) {
					return fmt.Errorf("unable to read existing drop-in file %q for unit %q: %w", dropInFilePath, unit.Name, err)
				}

				newDropInContent := []byte(dropIn.Content)
				if !bytes.Equal(newDropInContent, oldDropInContent) {
					if err := r.FS.WriteFile(dropInFilePath, newDropInContent, defaultFilePermissions); err != nil {
						return fmt.Errorf("unable to write drop-in file %q for unit %q: %w", dropInFilePath, unit.Name, err)
					}
					log.Info("Successfully applied new or changed drop-in file for unit", "path", dropInFilePath, "unit", unit.Name)
				}

				// ensure file permissions are restored in case somebody changed them manually
				if err := r.FS.Chmod(dropInFilePath, defaultFilePermissions); err != nil {
					return fmt.Errorf("unable to ensure permissions for drop-in file %q for unit %q: %w", unitFilePath, unit.Name, err)
				}
				if err := changes.completedUnitDropInChanged(unit.Name, dropIn.Name); err != nil {
					return err
				}
			}

			for _, dropIn := range slices.Clone(unit.DropInsChanges.Deleted) {
				dropInFilePath := path.Join(dropInDirectory, dropIn.Name)
				if err := r.FS.Remove(dropInFilePath); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
					return fmt.Errorf("unable to delete drop-in file %q for unit %q: %w", dropInFilePath, unit.Name, err)
				}
				log.Info("Successfully removed no longer needed drop-in file for unit", "path", dropInFilePath, "unitName", unit.Name)
				if err := changes.completedUnitDropInDeleted(unit.Name, dropIn.Name); err != nil {
					return err
				}
			}
		}

		if unit.Name == nodeagentconfigv1alpha1.UnitName || ptr.Deref(unit.Enable, true) {
			if err := r.DBus.Enable(ctx, unit.Name); err != nil {
				return fmt.Errorf("unable to enable unit %q: %w", unit.Name, err)
			}
			log.Info("Successfully enabled unit", "unitName", unit.Name)
		} else {
			if err := r.DBus.Disable(ctx, unit.Name); err != nil {
				return fmt.Errorf("unable to disable unit %q: %w", unit.Name, err)
			}
			log.Info("Successfully disabled unit", "unitName", unit.Name)
		}

		if err := changes.completedUnitChanged(unit.Name); err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) removeDeletedUnits(ctx context.Context, log logr.Logger, node client.Object, changes *operatingSystemConfigChanges) error {
	for _, unit := range slices.Clone(changes.Units.Deleted) {
		// The unit has been created by gardener-node-agent if it has content.
		// Otherwise, it might be a default OS unit which was enabled/disabled or where drop-ins were added.
		unitCreatedByNodeAgent := unit.Content != nil

		unitFilePath := path.Join(etcSystemdSystem, unit.Name)

		unitFileExists, err := r.FS.Exists(unitFilePath)
		if err != nil {
			return fmt.Errorf("unable to check whether unit file %q exists: %w", unitFilePath, err)
		}

		// Only stop and remove the unit file if it was created by gardener-node-agent. Otherwise, this could affect
		// default OS units where we add and remove drop-ins only. If operators want to stop and disable units,
		// they can do it by adding a unit to OSC which applies the `stop` command.
		if unitFileExists && unitCreatedByNodeAgent {
			if err := r.DBus.Disable(ctx, unit.Name); err != nil {
				return fmt.Errorf("unable to disable deleted unit %q: %w", unit.Name, err)
			}

			if err := r.DBus.Stop(ctx, r.Recorder, node, unit.Name); err != nil {
				return fmt.Errorf("unable to stop deleted unit %q: %w", unit.Name, err)
			}

			if err := r.FS.Remove(unitFilePath); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
				return fmt.Errorf("unable to delete systemd unit file of deleted unit %q: %w", unit.Name, err)
			} else {
				log.Info("Unit was not created by gardener-node-agent, skipping deletion of unit file", "unitName", unit.Name)
			}
		}

		dropInFolder := unitFilePath + ".d"

		if exists, err := r.FS.Exists(dropInFolder); err != nil {
			return fmt.Errorf("unable to check whether drop-in folder %q exists: %w", dropInFolder, err)
		} else if exists {
			for _, dropIn := range unit.DropIns {
				dropInFilePath := path.Join(dropInFolder, dropIn.Name)
				if err := r.FS.Remove(dropInFilePath); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
					return fmt.Errorf("unable to delete drop-in file %q of deleted unit %q: %w", dropInFilePath, unit.Name, err)
				}
			}

			if empty, err := r.FS.IsEmpty(dropInFolder); err != nil {
				return fmt.Errorf("unable to check whether drop-in folder %q is empty: %w", dropInFolder, err)
			} else if empty {
				if err := r.FS.RemoveAll(dropInFolder); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
					return fmt.Errorf("unable to delete systemd drop-in folder of deleted unit %q: %w", unit.Name, err)
				}
			}
		}

		// If the unit was not created by gardener-node-agent, but it exists on the node and was removed from OSC. Restart it to apply changes.
		if unitFileExists && !unitCreatedByNodeAgent {
			if err := r.DBus.Restart(ctx, r.Recorder, node, unit.Name); err != nil {
				return fmt.Errorf("unable to restart unit %q removed from OSC but not created by gardener-node-agent: %w", unit.Name, err)
			}
		}

		log.Info("Successfully removed no longer needed unit", "unitName", unit.Name)
		if err := changes.completedUnitDeleted(unit.Name); err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) executeUnitCommands(ctx context.Context, log logr.Logger, node client.Object, oscChanges *operatingSystemConfigChanges) error {
	var (
		fns []flow.TaskFn

		restart = func(ctx context.Context, unitName string) error {
			if err := r.DBus.Restart(ctx, r.Recorder, node, unitName); err != nil {
				return fmt.Errorf("unable to restart unit %q: %w", unitName, err)
			}
			log.Info("Successfully restarted unit", "unitName", unitName)

			if unitName == v1beta1constants.OperatingSystemConfigUnitNameContainerDService {
				if err := oscChanges.completedContainerdConfigFileChange(); err != nil {
					return err
				}
			}

			return oscChanges.completedUnitCommand(unitName)
		}

		stop = func(ctx context.Context, unitName string) error {
			if err := r.DBus.Stop(ctx, r.Recorder, node, unitName); err != nil {
				return fmt.Errorf("unable to stop unit %q: %w", unitName, err)
			}
			log.Info("Successfully stopped unit", "unitName", unitName)
			return oscChanges.completedUnitCommand(unitName)
		}
	)

	var containerdChanged bool
	for _, unit := range slices.Clone(oscChanges.Units.Commands) {
		switch unit.Name {
		case nodeagentconfigv1alpha1.UnitName:
			if err := oscChanges.setMustRestartNodeAgent(true); err != nil {
				return err
			}
			if err := oscChanges.completedUnitCommand(unit.Name); err != nil {
				return err
			}
			continue
		case v1beta1constants.OperatingSystemConfigUnitNameContainerDService:
			containerdChanged = true
		}

		fns = append(fns, func(ctx context.Context) error {
			switch unit.Command {
			case extensionsv1alpha1.CommandStop:
				return stop(ctx, unit.Name)
			case extensionsv1alpha1.CommandRestart:
				return restart(ctx, unit.Name)
			case "":
				return oscChanges.completedUnitCommand(unit.Name)
			}
			return fmt.Errorf("unknown unit command %q", unit.Command)
		})
	}

	if oscChanges.Containerd.ConfigFileChanged && !containerdChanged {
		fns = append(fns, func(ctx context.Context) error {
			return restart(ctx, v1beta1constants.OperatingSystemConfigUnitNameContainerDService)
		})
	}

	return flow.Parallel(fns...)(ctx)
}
