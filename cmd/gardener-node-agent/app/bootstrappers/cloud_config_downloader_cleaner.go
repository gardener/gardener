// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package bootstrappers

import (
	"context"
	"errors"
	"fmt"
	"path"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"

	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
)

// CloudConfigDownloaderCleaner is a runnable for cleaning up the legacy cloud-config-downloader resources.
// TODO(rfranzke): Remove this bootstrapper when the UseGardenerNodeAgent feature gate gets removed.
type CloudConfigDownloaderCleaner struct {
	Log  logr.Logger
	FS   afero.Afero
	DBus dbus.DBus
}

// Start performs the cleanup logic. Note that this function does only delete the following directories/files:
//   - /var/lib/cloud-config-downloader
//   - /etc/systemd/system/multi-user.target.wants/cloud-config-downloader.service (typically symlinks to
//     /etc/systemd/system/cloud-config-downloader.service
//
// The /etc/systemd/system/cloud-config-downloader.service file already gets removed by cloud-config-downloader itself
// when migrating to gardener-node-agent because it is no longer part of the original OperatingSystemConfig. Hence,
// cloud-config-downloader considers it as stale and cleans it up.
// All this still leaves some artefacts on the nodes (e.g., `systemctl status cloud-config-downloader` and
// `journalctl -u cloud-config-downloader` still works), however, maybe that's even a benefit in case of operations/
// debugging activities. All nodes get rolled/replaced eventually (latest with the next OS/Kubernetes version update),
// so we leave the final cleanup for then (new nodes will have no traces of cloud-config-downloader whatsoever).
func (c *CloudConfigDownloaderCleaner) Start(ctx context.Context) error {
	c.Log.Info("Removing legacy directory if it exists", "path", downloader.PathCCDDirectory)
	if err := c.FS.RemoveAll(downloader.PathCCDDirectory); err != nil {
		return fmt.Errorf("failed to remove legacy directory %q: %w", downloader.PathCCDDirectory, err)
	}

	unitFilePath := path.Join("/", "etc", "systemd", "system", "multi-user.target.wants", downloader.UnitName)
	c.Log.Info("Removing legacy unit file if it exists", "path", unitFilePath)
	if err := c.FS.Remove(unitFilePath); err != nil {
		if !errors.Is(err, afero.ErrFileNotFound) {
			return fmt.Errorf("failed removing legacy unit file file %q: %w", unitFilePath, err)
		}
		return nil
	}

	return c.DBus.DaemonReload(ctx)
}
