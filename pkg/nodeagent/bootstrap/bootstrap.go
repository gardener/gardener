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

package bootstrap

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"text/template"

	"github.com/go-logr/logr"

	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/downloader"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/controller/common"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
)

//go:embed templates/gardener-node-agent.service.tpl
var systemdUnit string

// Bootstraps the node agent by downloading binary, creating systemd unit and finally terminating itself.
func Bootstrap(ctx context.Context, log logr.Logger, db dbus.Dbus) error {
	log.Info("bootstrap")

	if err := renderSystemdUnit(); err != nil {
		return fmt.Errorf("unable to render system unit %s: %w", nodeagentv1alpha1.NodeAgentUnitName, err)
	}

	if err := db.Enable(ctx, nodeagentv1alpha1.NodeAgentUnitName); err != nil {
		return fmt.Errorf("unable to enable system unit %s: %w", nodeagentv1alpha1.NodeAgentUnitName, err)
	}

	if err := db.Start(ctx, nil, nil, nodeagentv1alpha1.NodeAgentUnitName); err != nil {
		return fmt.Errorf("unable to start system unit %s: %w", nodeagentv1alpha1.NodeAgentUnitName, err)
	}

	if err := db.Disable(ctx, nodeagentv1alpha1.NodeInitUnitName); err != nil {
		return fmt.Errorf("unable to disable system unit %s: %w", nodeagentv1alpha1.NodeInitUnitName, err)
	}

	if err := cleanupLegacyCloudConfigDownloader(ctx, db); err != nil {
		return fmt.Errorf("unable to cleanup cloud-config-downloader: %w", err)
	}

	if err := formatDataDevice(log); err != nil {
		return err
	}

	// Stop itself, must be the last action because it will not get executed anyway.
	// With this command the execution of the gardener-node-agent bootstrap command terminates.
	// It is not possible to do any logic after calling this stop command anymore here.
	return db.Stop(ctx, nil, nil, nodeagentv1alpha1.NodeInitUnitName)
}

func renderSystemdUnit() error {
	tpl := template.Must(
		template.New("v4").
			Funcs(template.FuncMap{"StringsJoin": strings.Join}).
			Parse(systemdUnit),
	)

	var target bytes.Buffer
	if err := tpl.Execute(&target, nil); err != nil {
		return err
	}

	return os.WriteFile(path.Join("/etc", "systemd", "system", nodeagentv1alpha1.NodeAgentUnitName), target.Bytes(), 0644)
}

func cleanupLegacyCloudConfigDownloader(ctx context.Context, db dbus.Dbus) error {
	if _, err := os.Stat(path.Join("/etc", "systemd", "system", downloader.UnitName)); err != nil && os.IsNotExist(err) {
		return nil
	}

	if err := db.Stop(ctx, nil, nil, downloader.UnitName); err != nil {
		return fmt.Errorf("unable to stop system unit %s: %w", downloader.UnitName, err)
	}

	if err := db.Disable(ctx, downloader.UnitName); err != nil {
		return fmt.Errorf("unable to disable system unit %s: %w", downloader.UnitName, err)
	}

	return nil
}

func formatDataDevice(log logr.Logger) error {
	config, err := common.ReadNodeAgentConfiguration(nil)
	if err != nil {
		return err
	}

	if config.KubeletDataVolumeSize == nil {
		return nil
	}

	var (
		size  = *config.KubeletDataVolumeSize
		label = "KUBEDEV"
	)

	out, err := execCommand("blkid", "--label", "label="+label)
	if err != nil {
		return fmt.Errorf("unable to execute blkid output:%s %w", out, err)
	}
	if out != nil {
		log.Info("kubernetes data volume already mounted", "blkid-output", string(out))
		return nil
	}

	out, err = execCommand("lsblk", "-dbsnP", "-o", "NAME,PARTTYPE,MOUNTPOINT,FSTYPE,SIZE,PATH,TYPE")
	if err != nil {
		return fmt.Errorf("unable to execute lsblk: output %s; error: %w", out, err)
	}

	targetDevice := ""
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "TYPE=\"disk\"") {
			continue
		}
		if strings.Contains(line, " MOUNTPOINT=\"/") {
			continue
		}
		if strings.Contains(line, " SIZE="+strconv.FormatInt(*config.KubeletDataVolumeSize, 10)) {
			var found bool
			targetDevice, _, found = strings.Cut(line, ":")
			if !found {
				continue
			}
		}
	}

	if targetDevice == "" {
		log.Info("no kubernetes data volume with matching size found", "size", size)
		return nil
	}

	log.Info("kubernetes data volume with matching size found", "device", targetDevice, "size", size)

	out, err = execCommand("mkfs.ext4", "-L", label, "-O", "quota", "-E", "lazy_itable_init=0,lazy_journal_init=0,quotatype=usrquota:grpquota:prjquota", "/dev/"+targetDevice)
	if err != nil {
		return fmt.Errorf("unable to execute mkfs: output %s; error: %w", out, err)
	}

	if err := os.MkdirAll("/tmp/varlibcp", fs.ModeDir); err != nil {
		return fmt.Errorf("unable to create temporary mount dir: %w", err)
	}

	out, err = execCommand("mount", "LABEL="+label, "/tmp/varlibcp")
	if err != nil {
		return fmt.Errorf("unable to execute mkfs: output %s; error: %w", out, err)
	}

	out, err = execCommand("cp", "-r", "/var/lib/*", "/tmp/varlibcp")
	if err != nil {
		return fmt.Errorf("unable to copy: output %s; error: %w", out, err)
	}

	out, err = execCommand("umount", "/tmp/varlibcp")
	if err != nil {
		return fmt.Errorf("unable to execute umount: output %s; error: %w", out, err)
	}

	out, err = execCommand("mount", "LABEL="+label, "/var/lib", "-o", "defaults,prjquota,errors=remount-ro")
	if err != nil {
		return fmt.Errorf("unable to execute mount: output %s; error: %w", out, err)
	}

	log.Info("kubelet data volume mounted to /var/lib", "device", targetDevice, "size", size)

	// INFO: for reference a copy of the original function from the executor.
	// function format-data-device() {
	// 	LABEL=KUBEDEV
	// 	if ! blkid --label $LABEL >/dev/null; then
	// 	  DISK_DEVICES=$(lsblk -dbsnP -o NAME,PARTTYPE,MOUNTPOINT,FSTYPE,SIZE,PATH,TYPE | grep 'TYPE="disk"')
	// 	  while IFS= read -r line; do
	// 		MATCHING_DEVICE_CANDIDATE=$(echo "$line" | grep 'PARTTYPE="".*FSTYPE="".*SIZE="{{ .kubeletDataVolume.size }}"')
	// 		if [ -z "$MATCHING_DEVICE_CANDIDATE" ]; then
	// 		  continue
	// 		fi

	// 		# Skip device if it's already mounted.
	// 		DEVICE_NAME=$(echo "$MATCHING_DEVICE_CANDIDATE" | cut -f2 -d\")
	// 		DEVICE_MOUNTS=$(lsblk -dbsnP -o NAME,MOUNTPOINT,TYPE | grep -e "^NAME=\"$DEVICE_NAME.*\".*TYPE=\"part\"$")
	// 		if echo "$DEVICE_MOUNTS" | awk '{print $2}' | grep "MOUNTPOINT=\"\/.*\"" > /dev/null; then
	// 		  continue
	// 		fi

	// 		TARGET_DEVICE_NAME="$DEVICE_NAME"
	// 		break
	// 	  done <<< "$DISK_DEVICES"

	// 	  if [ -z "$TARGET_DEVICE_NAME" ]; then
	// 		echo "No kubelet data device found"
	// 		return
	// 	  fi

	// 	  echo "Matching kubelet data device by size : {{ .kubeletDataVolume.size }}"
	// 	  echo "detected kubelet data device $TARGET_DEVICE_NAME"
	// 	  mkfs.ext4 -L $LABEL -O quota -E lazy_itable_init=0,lazy_journal_init=0,quotatype=usrquota:grpquota:prjquota  /dev/$TARGET_DEVICE_NAME
	// 	  echo "formatted and labeled data device $TARGET_DEVICE_NAME"
	// 	  mkdir /tmp/varlibcp
	// 	  mount LABEL=$LABEL /tmp/varlibcp
	// 	  echo "mounted temp copy dir on data device $TARGET_DEVICE_NAME"
	// 	  cp -a /var/lib/* /tmp/varlibcp/
	// 	  umount /tmp/varlibcp
	// 	  echo "copied /var/lib to data device $TARGET_DEVICE_NAME"
	// 	  mount LABEL=$LABEL /var/lib -o defaults,prjquota,errors=remount-ro
	// 	  echo "mounted /var/lib on data device $TARGET_DEVICE_NAME"
	// 	fi
	//   }

	//   format-data-device

	return nil
}

func execCommand(name string, arg ...string) ([]byte, error) {
	cmd, err := exec.LookPath(name)
	if err != nil {
		return nil, fmt.Errorf("unable to locate program:%q in path %w", name, err)
	}

	out, err := exec.Command(cmd, arg...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("unable to execute %q output:%s %w", name, out, err)
	}
	return out, nil
}
