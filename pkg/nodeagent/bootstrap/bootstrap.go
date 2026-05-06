// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	nodeagentcomponent "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	"github.com/gardener/gardener/pkg/nodeagent"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
)

// Bootstrap bootstraps the gardener-node-agent by adding and starting its systemd unit and afterward disabling the
// gardener-node-init. If `kubeletDataVolumeSize` is non-zero, it formats the data device.
func Bootstrap(
	ctx context.Context,
	log logr.Logger,
	fs afero.Afero,
	dbus dbus.DBus,
	bootstrapConfig *nodeagentconfigv1alpha1.BootstrapConfiguration,
) error {
	log.Info("Starting bootstrap procedure")

	log.Info("Creating directory for temporary files", "path", nodeagentconfigv1alpha1.TempDir)
	if err := fs.MkdirAll(nodeagentconfigv1alpha1.TempDir, os.ModeDir); err != nil {
		return fmt.Errorf("unable to create directory for temporary files %q: %w", nodeagentconfigv1alpha1.TempDir, err)
	}

	if bootstrapConfig != nil && bootstrapConfig.KubeletDataVolumeSize != nil {
		log.Info("Ensure mount point for kubelet data volume exists", "path", kubelet.PathKubeletDirectory)
		if err := fs.MkdirAll(kubelet.PathKubeletDirectory, os.ModeDir); err != nil {
			return fmt.Errorf("unable to create directory for kubelet %q: %w", kubelet.PathKubeletDirectory, err)
		}
		log.Info("Start kubelet data volume formatter", "kubeletDataVolumeSize", *bootstrapConfig.KubeletDataVolumeSize)
		if err := formatKubeletDataDevice(log.WithName("kubelet-data-volume-device-formatter"), fs, *bootstrapConfig.KubeletDataVolumeSize); err != nil {
			return fmt.Errorf("failed formatting kubelet data volume: %w", err)
		}
	}

	unitFilePath := path.Join("/", "etc", "systemd", "system", nodeagentconfigv1alpha1.UnitName)
	log.Info("Writing unit file for gardener-node-agent", "path", unitFilePath)
	if err := fs.WriteFile(unitFilePath, []byte(nodeagentcomponent.UnitContent()), 0644); err != nil {
		return fmt.Errorf("unable to write unit file %q to path %q: %w", nodeagentconfigv1alpha1.UnitName, unitFilePath, err)
	}

	log.Info("Reloading systemd daemon")
	if err := dbus.DaemonReload(ctx); err != nil {
		return fmt.Errorf("failed reloading systemd daemon: %w", err)
	}

	log.Info("Enabling gardener-node-agent unit")
	if err := dbus.Enable(ctx, nodeagentconfigv1alpha1.UnitName); err != nil {
		return fmt.Errorf("unable to enable unit %q: %w", nodeagentconfigv1alpha1.UnitName, err)
	}

	log.Info("Starting gardener-node-agent unit")
	if err := dbus.Start(ctx, nil, nil, nodeagentconfigv1alpha1.UnitName); err != nil {
		return fmt.Errorf("unable to start unit %q: %w", nodeagentconfigv1alpha1.UnitName, err)
	}

	log.Info("Disabling gardener-node-init unit")
	if err := dbus.Disable(ctx, nodeagentconfigv1alpha1.InitUnitName); err != nil {
		return fmt.Errorf("unable to disable system unit %q: %w", nodeagentconfigv1alpha1.InitUnitName, err)
	}

	log.Info("Testing API server connectivity")
	if err := testAPIServerConnectivity(log, fs); err != nil {
		log.Error(err, "API server connectivity test failed")
		fmt.Printf("WARNING: API server connectivity test failed: %v\n", err)
		fmt.Println("Node may fail to join the cluster - check network configuration and API server endpoint")
		// Don't return error - let gardener-node-agent continue and retry with backoff
	}

	// After this line, the execution of the gardener-node-agent bootstrap command terminates. It is not possible to
	// perform any logic after this line.
	log.Info("Bootstrap procedure finished, terminating")
	return nil
}

// testAPIServerConnectivity performs a connectivity test to the Kubernetes API server during bootstrap.
// This provides early feedback on network issues before the bootstrap phase completes.
func testAPIServerConnectivity(log logr.Logger, fs afero.Afero) error {
	// Read API server config from the gardener-node-agent configuration
	apiServerConfig, err := nodeagent.GetAPIServerConfig(fs, nodeagentconfigv1alpha1.BaseDir)
	if err != nil {
		return fmt.Errorf("failed reading API server config: %w", err)
	}

	// Create REST config with bootstrap token and short timeout
	restConfig := &rest.Config{
		Host:            apiServerConfig.Server,
		TLSClientConfig: rest.TLSClientConfig{CAData: apiServerConfig.CABundle},
		BearerTokenFile: nodeagentconfigv1alpha1.BootstrapTokenFilePath,
		Timeout:         10 * time.Second,
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed creating kubernetes client: %w", err)
	}

	// Simple connectivity test - try to discover server version
	log.Info("Attempting to connect to API server", "server", apiServerConfig.Server)
	_, err = client.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("API server unreachable at %s: %w", apiServerConfig.Server, err)
	}

	log.Info("Successfully connected to API server", "server", apiServerConfig.Server)
	return nil
}
