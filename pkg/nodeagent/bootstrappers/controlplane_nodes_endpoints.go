// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// ControlPlaneNodesEndpoints is a runnable for writing the IP addresses of the control plane nodes to a file on the
// host. This file is later mounted into the etcd-backup-restore container that runs as static pod on control plane
// nodes of self-hosted shoot clusters.
type ControlPlaneNodesEndpoints struct {
	Log    logr.Logger
	FS     afero.Afero
	Client client.Client
}

// Start writes the IP addresses of the control plane nodes to a file on the host if it does not exist yet.
func (c *ControlPlaneNodesEndpoints) Start(ctx context.Context) error {
	if _, err := c.FS.Stat(v1beta1constants.OperatingSystemConfigFilePathControlPlaneNodesEndpoints); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed checking for existence of file %s: %w", v1beta1constants.OperatingSystemConfigFilePathControlPlaneNodesEndpoints, err)
	} else if err == nil {
		c.Log.Info("File containing control plane nodes endpoints already exists, will not overwrite it", "file", v1beta1constants.OperatingSystemConfigFilePathControlPlaneNodesEndpoints)
		return nil
	}

	nodeList := &corev1.NodeList{}
	if err := c.Client.List(ctx, nodeList, client.MatchingLabels{v1beta1constants.LabelNodeRoleControlPlane: ""}); err != nil {
		return fmt.Errorf("failed to list control plane nodes: %w", err)
	}

	var endpoints []string
	for _, node := range nodeList.Items {
		preferIPv6 := false
		if v, ok := node.Labels[v1beta1constants.LabelNodePreferIPv6]; ok {
			var err error
			preferIPv6, err = strconv.ParseBool(v)
			if err != nil {
				return fmt.Errorf("failed to parse %q label on node %q: %w", v1beta1constants.LabelNodePreferIPv6, node.Name, err)
			}
		}

		ip, err := kubernetesutils.NodeInternalIP(node, preferIPv6)
		if err != nil {
			return fmt.Errorf("failed determining IP address of control plane node %q: %w", node.Name, err)
		}

		endpoints = append(endpoints, ip.String())
	}

	if err := c.FS.MkdirAll(filepath.Dir(v1beta1constants.OperatingSystemConfigFilePathControlPlaneNodesEndpoints), 0755); err != nil {
		return fmt.Errorf("failed creating directory %s: %w", filepath.Dir(v1beta1constants.OperatingSystemConfigFilePathControlPlaneNodesEndpoints), err)
	}

	c.Log.Info("Writing file containing IP addresses of control plane nodes",
		"path", v1beta1constants.OperatingSystemConfigFilePathControlPlaneNodesEndpoints,
		"endpoints", endpoints,
	)
	return c.FS.WriteFile(v1beta1constants.OperatingSystemConfigFilePathControlPlaneNodesEndpoints, []byte(strings.Join(endpoints, "\n")), 0600)
}
