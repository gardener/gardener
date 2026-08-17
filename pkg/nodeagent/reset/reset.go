// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reset

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/runtime"
	criclient "k8s.io/cri-client/pkg"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/nodeagent"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// NewRemoteRuntimeService create a new remote runtime service.
// Exposed for testing.
var NewRemoteRuntimeService = criclient.NewRemoteRuntimeService

// Reset resets the node by stopping and removing the systemd units and files added via the
// operating system configuration. It also cleans up the kubelet working directory /var/lib/kubelet.
func Reset(ctx context.Context, log logr.Logger, fs afero.Afero, dbus dbus.DBus) error {
	var (
		g        = flow.NewGraph("reset")
		reporter = flow.NewCommandLineProgressReporter(os.Stderr)

		stoppedContainers = g.Add(flow.Task{
			Name: "Stopping containers",
			Fn: func(ctx context.Context) error {
				return stopContainers(ctx, log, dbus)
			},
		})
		cleanedUpOSC = g.Add(flow.Task{
			Name: "Cleaning up operating system configuration",
			Fn: func(ctx context.Context) error {
				return cleanUpOperatingSystemConfig(ctx, log, fs, dbus)
			},
			Dependencies: flow.NewTaskIDs(stoppedContainers),
		})
		_ = g.Add(flow.Task{
			Name: "Cleaning up gardener node agent folder",
			Fn: func(_ context.Context) error {
				return cleanUpNodeAgentFolder(log, fs)
			},
			Dependencies: flow.NewTaskIDs(cleanedUpOSC),
		})
		_ = g.Add(flow.Task{
			Name: "Cleaning up kubelet folder",
			Fn: func(_ context.Context) error {
				return cleanUpKubeletFolder(log, fs)
			},
			Dependencies: flow.NewTaskIDs(cleanedUpOSC),
		})
	)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log:              log,
		ProgressReporter: reporter,
	}); err != nil {
		return flow.Errors(err)
	}

	fmt.Fprintf(os.Stdout, `
The node has been successfully cleaned from gardener-node-agent artifacts!
`)

	return nil
}

func stopContainers(ctx context.Context, log logr.Logger, dbus dbus.DBus) error {
	log.Info("Stopping gardener-node-agent systemd unit")
	if err := dbus.Stop(ctx, nil, nil, nodeagentconfigv1alpha1.UnitName); err != nil {
		return fmt.Errorf("failed stopping gardener-node-agent systemd unit: %w", err)
	}

	log.Info("Stopping kubelet systemd unit")
	if err := dbus.Stop(ctx, nil, nil, v1beta1constants.OperatingSystemConfigUnitNameKubeletService); err != nil {
		return fmt.Errorf("failed stopping kubelet systemd unit: %w", err)
	}

	log.Info("Stopping containers")
	runtimeService, err := NewRemoteRuntimeService(ctx, "unix:///var/run/containerd/containerd.sock", 2*time.Second, nil, false)
	if err != nil {
		return fmt.Errorf("failed creating CRI client: %w", err)
	}
	defer runtimeService.Close(ctx)

	pods, err := runtimeService.ListPodSandbox(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed listing pod sandboxes: %w", err)
	}

	var errs []error
	for _, pod := range pods {
		log.Info("Stopping pod sandbox", "podID", pod.GetId())
		var lastErr error
		for range 5 { // Retry up to 5 times
			if err := runtimeService.StopPodSandbox(ctx, pod.GetId()); err != nil {
				lastErr = err
				log.Error(err, "Failed stopping pod sandbox", "podID", pod.Id)
				continue
			}

			if err := runtimeService.RemovePodSandbox(ctx, pod.GetId()); err != nil {
				lastErr = err
				log.Error(err, "Failed removing pod sandbox", "podID", pod.Id)
				continue
			}

			lastErr = nil
			break
		}

		if lastErr != nil {
			errs = append(errs, lastErr)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed stopping some pods: %w", errors.Join(errs...))
	}

	return nil
}

func cleanUpOperatingSystemConfig(ctx context.Context, log logr.Logger, fs afero.Afero, dbus dbus.DBus) error {
	log.Info("Checking last-applied OperatingSystemConfig for cleanup")
	oscFileContent, err := fs.ReadFile(nodeagentconfigv1alpha1.LastAppliedOperatingSystemConfigFilePath)
	if err != nil {
		if errors.Is(err, afero.ErrFileNotFound) {
			log.Info("No last-applied OperatingSystemConfig found, skipping cleanup")
			return nil
		}
		return fmt.Errorf("cannot read last-applied OperatingSystemConfig: %w", err)
	}

	osc := &extensionsv1alpha1.OperatingSystemConfig{}
	if err := runtime.DecodeInto(nodeagent.Codec, oscFileContent, osc); err != nil {
		return fmt.Errorf("unable to decode OperatingSystemConfig read from file path %s: %w", nodeagentconfigv1alpha1.LastAppliedOperatingSystemConfigFilePath, err)
	}

	var errs []error
	log.Info("Stopping systemd units")
	for _, unit := range append(osc.Spec.Units, osc.Status.ExtensionUnits...) {
		log.Info("Stopping systemd unit", "unit", unit.Name)
		if err := dbus.Stop(ctx, nil, nil, unit.Name); err != nil {
			errs = append(errs, err)
		}
		log.Info("Disabling systemd unit", "unit", unit.Name)
		if err := dbus.Disable(ctx, unit.Name); err != nil {
			errs = append(errs, err)
		}
	}

	log.Info("Removing installed files")
	for _, file := range append(osc.Spec.Files, osc.Status.ExtensionFiles...) {
		log.Info("Removing file", "path", file.Path)
		if err := fs.Remove(file.Path); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed cleaning up OperatingSystemConfig: %w", errors.Join(errs...))
	}

	return nil
}

// Failing to clean up the gardener-node-agent folder will block the node from joining the cluster again.
func cleanUpNodeAgentFolder(log logr.Logger, fs afero.Afero) error {
	log.Info("Cleaning up gardener-node-agent folder")
	entries, err := fs.ReadDir(nodeagentconfigv1alpha1.BaseDir)
	if err != nil && err != afero.ErrFileNotFound {
		return fmt.Errorf("failed listing garden-node-agent folder: %w", err)
	}
	for _, entry := range entries {
		log.Info("Removing gardener-node-agent file", "file", entry.Name())
		if err := fs.RemoveAll(nodeagentconfigv1alpha1.BaseDir + "/" + entry.Name()); err != nil {
			return fmt.Errorf("failed removing %q in gardener-node-agent dir %q: %w", entry.Name(), nodeagentconfigv1alpha1.BaseDir, err)
		}
	}

	return nil
}

func cleanUpKubeletFolder(log logr.Logger, fs afero.Afero) error {
	log.Info("Cleaning up kubelet folder")
	entries, err := fs.ReadDir(kubelet.PathKubeletDirectory)
	if err != nil && err != afero.ErrFileNotFound {
		return fmt.Errorf("failed listing kubelet folder: %w", err)
	}

	if err := unmountKubeletSubFolders(log, fs); err != nil {
		return fmt.Errorf("failed unmounting kubelet sub folders: %w", err)
	}

	for _, entry := range entries {
		log.Info("Removing kubelet file", "file", entry.Name())
		if err := fs.RemoveAll(kubelet.PathKubeletDirectory + "/" + entry.Name()); err != nil {
			return fmt.Errorf("failed removing %q in kubelet dir %q: %w", entry.Name(), kubelet.PathKubeletDirectory, err)
		}
	}

	return nil
}

func unmountKubeletSubFolders(log logr.Logger, fs afero.Afero) error {
	// Add trailing '/' to prevent unmounting the actual directory.
	kubeletFolderPrefix := kubelet.PathKubeletDirectory + "/"

	mounts, err := fs.ReadFile("/proc/mounts")
	if err != nil {
		return fmt.Errorf("failed reading /proc/mounts: %w", err)
	}

	var errs []error
	for _, mount := range strings.Split(string(mounts), "\n") {
		// Looking for entries like "tmpfs /var/lib/kubelet/pods/..."
		words := strings.Split(mount, " ")
		if len(words) < 2 || !strings.HasPrefix(words[1], kubeletFolderPrefix) {
			continue
		}

		log.Info("Unmounting kubelet sub folder", "path", words[1])
		if err := syscall.Unmount(words[1], 0); err != nil {
			if err == syscall.EINVAL {
				// Entry may have already been unmounted due to duplicate entries => ignore it
				continue
			}

			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed unmounting folders: %w", errors.Join(errs...))
	}

	return nil
}
