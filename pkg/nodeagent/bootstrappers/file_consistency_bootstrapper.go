// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	"go.yaml.in/yaml/v4"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	dbuspkg "github.com/gardener/gardener/pkg/nodeagent/dbus"
	"github.com/gardener/gardener/pkg/utils"
)

const (
	// Systemd directory paths
	systemdSystemPath = "/etc/systemd/system"
	systemdUsrLibPath = "/usr/lib/systemd/system"
	systemdLibPath    = "/lib/systemd/system"
)

// OSCChecker is a bootstrapper that checks the consistency of OperatingSystemConfig resources
// against the actual state of files and systemd units on the node.
type OSCChecker struct {
	Log      logr.Logger
	FS       afero.Afero
	Client   client.Client
	Recorder record.EventRecorder
	DBus     dbuspkg.DBus
	NodeName string

	node *corev1.Node
}

// Start performs the OSC consistency checks by comparing the last applied OperatingSystemConfig
// against the actual state of files and systemd units on the node.
func (c *OSCChecker) Start(ctx context.Context) error {
	c.Log.Info("Starting OSC checker")

	var node corev1.Node
	if err := c.Client.Get(ctx, client.ObjectKey{Name: c.NodeName}, &node); err != nil {
		if apierrors.IsNotFound(err) {
			c.Log.Info("Node not found yet, skipping OSC checks", "node", c.NodeName)
			return nil
		}
		return fmt.Errorf("failed getting node: %w", err)
	}
	c.node = &node

	oscFileContent, err := c.FS.ReadFile(nodeagentconfigv1alpha1.LastAppliedOperatingSystemConfigFilePath)
	if err != nil {
		if errors.Is(err, afero.ErrFileNotFound) {
			c.Log.Info("No last-applied OSC found, skipping")
			return nil
		}
		return fmt.Errorf("cannot read last-applied OSC: %w", err)
	}

	var osc v1alpha1.OperatingSystemConfig
	if err := yaml.Unmarshal(oscFileContent, &osc); err != nil {
		return fmt.Errorf("cannot parse OSC YAML: %w", err)
	}

	unitList, err := c.DBus.List(ctx)
	if err != nil {
		return fmt.Errorf("failed listing systemd units: %w", err)
	}
	unitMap := make(map[string]dbus.UnitStatus, len(unitList))
	for _, u := range unitList {
		unitMap[u.Name] = u
	}

	for _, f := range osc.Spec.Files {
		c.checkFile(f)
	}

	for _, unit := range osc.Spec.Units {
		c.checkUnitFile(&unit)

		for _, di := range unit.DropIns {
			diPath := c.resolveDropInPath(unit.Name, di.Name)
			c.checkDropInFile(diPath, &di, unit.Name)
		}

		if unit.Enable != nil {
			c.checkUnitEnabled(unit.Name, *unit.Enable, unitMap)
		}

		for _, dep := range unit.FilePaths {
			exists, err := c.FS.Exists(dep)
			if err != nil || !exists {
				c.Log.Info("Dependency missing", "unit", unit.Name, "file", dep)
				c.emitEvent(
					"DependencyMissing",
					fmt.Sprintf("Dependency file %s for unit %s is missing", dep, unit.Name),
				)
			}
		}
	}

	c.Log.Info("OSC checker finished")
	return nil
}

func (c *OSCChecker) checkFile(file v1alpha1.File) {
	inline := file.Content.Inline
	if inline == nil || inline.Data == "" || file.Content.ImageRef != nil {
		return
	}

	var expected []byte
	switch inline.Encoding {
	case "", "plain":
		expected = []byte(inline.Data)
	case "b64", "base64":
		var err error
		if expected, err = utils.DecodeBase64(inline.Data); err != nil {
			c.emitEvent(
				"FileDecodeError",
				fmt.Sprintf("Failed to decode base64 content for file %s", file.Path),
			)
			return
		}
	default:
		c.emitEvent(
			"FileUnsupportedEncoding",
			fmt.Sprintf("Unsupported encoding %s for file %s", inline.Encoding, file.Path),
		)
		return
	}

	expectedHash := utils.ComputeSHA256Hex(expected)
	actual, err := c.FS.ReadFile(filepath.Clean(file.Path))
	if err != nil {
		if errors.Is(err, afero.ErrFileNotFound) {
			c.emitEvent(
				"FileMissing",
				fmt.Sprintf("File %s is missing", file.Path),
			)
		} else {
			c.emitEvent(
				"FileReadError",
				fmt.Sprintf("Failed to read file %s", file.Path),
			)
		}
		return
	}

	actualHash := utils.ComputeSHA256Hex(actual)
	if expectedHash != actualHash {
		c.emitEvent(
			"FileMismatch",
			fmt.Sprintf("File %s mismatch: expected %s, actual %s", file.Path, expectedHash, actualHash),
		)
	}
}

func (c *OSCChecker) checkUnitFile(unit *v1alpha1.Unit) {
	path := c.resolveUnitPath(unit.Name)

	raw, err := c.FS.ReadFile(path)
	if err != nil {
		c.emitEvent(
			"UnitFileMissing",
			fmt.Sprintf("Unit file %s is missing", unit.Name),
		)
		return
	}

	if unit.Content == nil {
		return
	}

	expectedHash := utils.ComputeSHA256Hex([]byte(*unit.Content))
	actualHash := utils.ComputeSHA256Hex(raw)

	if expectedHash != actualHash {
		c.emitEvent(
			"UnitMismatch",
			fmt.Sprintf("Unit %s content mismatch", unit.Name),
		)
	}
}

func (c *OSCChecker) checkDropInFile(path string, dropIn *v1alpha1.DropIn, unitName string) {
	raw, err := c.FS.ReadFile(path)
	if err != nil {
		c.emitEvent(
			"DropInMissing",
			fmt.Sprintf("Drop-in %s for unit %s is missing", dropIn.Name, unitName),
		)
		return
	}

	expectedHash := utils.ComputeSHA256Hex([]byte(dropIn.Content))
	actualHash := utils.ComputeSHA256Hex(raw)

	if expectedHash != actualHash {
		c.emitEvent(
			"DropInMismatch",
			fmt.Sprintf("Drop-in %s for unit %s mismatch", dropIn.Name, unitName),
		)
	}
}

func (c *OSCChecker) checkUnitEnabled(name string, expectedEnabled bool, unitMap map[string]dbus.UnitStatus) {
	_, isEnabled := unitMap[name]

	if !isEnabled && expectedEnabled {
		unitPath := c.resolveUnitPath(name)
		if data, err := c.FS.ReadFile(unitPath); err == nil {
			if !strings.Contains(string(data), "[Install]") {
				isEnabled = true // static unit
			}
		}
	}

	if isEnabled != expectedEnabled {
		c.emitEvent(
			"UnitEnableMismatch",
			fmt.Sprintf("Unit %s enable state mismatch: expected %t, actual %t",
				name, expectedEnabled, isEnabled),
		)
	}
}

func (c *OSCChecker) resolveUnitPath(name string) string {
	paths := []string{
		filepath.Join(systemdSystemPath, name),
		filepath.Join(systemdUsrLibPath, name),
		filepath.Join(systemdLibPath, name),
	}

	for _, p := range paths {
		if exists, err := c.FS.Exists(p); exists && err == nil {
			return p
		}
	}
	return filepath.Join(systemdSystemPath, name) // fallback
}

func (c *OSCChecker) resolveDropInPath(unitName, dropInName string) string {
	unitPath := c.resolveUnitPath(unitName)
	unitDir := filepath.Dir(unitPath)

	return filepath.Join(unitDir, unitName+".d", dropInName)
}

func (c *OSCChecker) emitEvent(reason, message string) {
	if c.node == nil {
		return
	}

	c.Log.Info("Emitting OSC event",
		"node", c.node.Name,
		"reason", reason,
		"message", message,
	)

	c.Recorder.Event(c.node, corev1.EventTypeWarning, reason, message)
}
