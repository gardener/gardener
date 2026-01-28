// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

const (
	lastAppliedOSCPath = nodeagentconfigv1alpha1.BaseDir + "/last-applied-osc.yaml"

	// Systemd directory paths
	systemdSystemPath      = "/etc/systemd/system"
	systemdUsrLibPath      = "/usr/lib/systemd/system"
	systemdLibPath         = "/lib/systemd/system"
	systemdMultiUserTarget = "/etc/systemd/system/multi-user.target.wants"
)

// OSCChecker is a bootstrapper that checks the consistency of OperatingSystemConfig resources
// against the actual state of files and systemd units on the node.
type OSCChecker struct {
	Log      logr.Logger
	FS       afero.Afero
	Client   client.Client
	Recorder record.EventRecorder
	NodeName string

	node *corev1.Node
}

// Start performs the OSC consistency checks by comparing the last applied OperatingSystemConfig
// against the actual state of files and systemd units on the node.
func (c *OSCChecker) Start(ctx context.Context) error {
	c.Log.Info("Starting OSC checker bootstrapper")

	var node corev1.Node
	if err := c.Client.Get(ctx, client.ObjectKey{Name: c.NodeName}, &node); err != nil {
		if apierrors.IsNotFound(err) {
			c.Log.Info("Node not found yet, skipping OSC checks", "node", c.NodeName)
			return nil
		}
		return fmt.Errorf("failed getting node: %w", err)
	}
	c.node = &node

	oscFileContent, err := c.FS.ReadFile(lastAppliedOSCPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.Log.Info("No last-applied OSC found, skipping")
			return nil
		}
		return fmt.Errorf("cannot read last-applied OSC: %w", err)
	}

	var osc v1alpha1.OperatingSystemConfig
	if err := yaml.Unmarshal(oscFileContent, &osc); err != nil {
		return fmt.Errorf("cannot parse OSC YAML: %w", err)
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
			c.checkUnitEnabled(unit.Name, *unit.Enable)
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

	c.Log.Info("Finished OSC checking bootstrapper")
	return nil
}

func (c *OSCChecker) checkFile(f v1alpha1.File) {
	inline := f.Content.Inline
	if inline == nil || inline.Data == "" || f.Content.ImageRef != nil {
		return
	}

	var expected []byte
	switch inline.Encoding {
	case "", "plain":
		expected = []byte(inline.Data)
	case "b64", "base64":
		var err error
		if expected, err = utils.DecodeBase64(inline.Data); err != nil {
			c.emitEvent("FileDecodeError",
				fmt.Sprintf("Failed to decode base64 content for file %s", f.Path))
			return
		}
	default:
		c.emitEvent("FileUnsupportedEncoding",
			fmt.Sprintf("Unsupported encoding %s for file %s", inline.Encoding, f.Path))
		return
	}

	expectedHash := utils.ComputeSHA256Hex(expected)
	actual, err := c.FS.ReadFile(filepath.Clean(f.Path))
	if err != nil {
		if os.IsNotExist(err) {
			c.emitEvent("FileMissing",
				fmt.Sprintf("File %s is missing", f.Path))
		} else {
			c.emitEvent("FileReadError",
				fmt.Sprintf("Failed to read file %s", f.Path))
		}
		return
	}

	actualHash := utils.ComputeSHA256Hex(actual)
	if expectedHash != actualHash {
		c.emitEvent("FileMismatch",
			fmt.Sprintf("File %s mismatch: expected %s, actual %s",
				f.Path, expectedHash, actualHash))
	}
}

func (c *OSCChecker) checkUnitFile(unit *v1alpha1.Unit) {
	path := c.resolveUnitPath(unit.Name)

	raw, err := c.FS.ReadFile(path)
	if err != nil {
		c.emitEvent("UnitFileMissing",
			fmt.Sprintf("Unit file %s is missing", unit.Name))
		return
	}

	if unit.Content == nil {
		return
	}

	expectedHash := utils.ComputeSHA256Hex([]byte(*unit.Content))
	actualHash := utils.ComputeSHA256Hex(raw)

	if expectedHash != actualHash {
		c.emitEvent("UnitMismatch",
			fmt.Sprintf("Unit %s content mismatch", unit.Name))
	}
}

func (c *OSCChecker) checkDropInFile(path string, di *v1alpha1.DropIn, unitName string) {
	raw, err := c.FS.ReadFile(path)
	if err != nil {
		c.emitEvent("DropInMissing",
			fmt.Sprintf("Drop-in %s for unit %s is missing", di.Name, unitName))
		return
	}

	expectedHash := utils.ComputeSHA256Hex([]byte(di.Content))
	actualHash := utils.ComputeSHA256Hex(raw)

	if expectedHash != actualHash {
		c.emitEvent("DropInMismatch",
			fmt.Sprintf("Drop-in %s for unit %s mismatch", di.Name, unitName))
	}
}

func (c *OSCChecker) checkUnitEnabled(name string, expectedEnabled bool) {
	want := filepath.Join(systemdMultiUserTarget, name)
	isEnabled, err := c.FS.Exists(want)
	if err != nil {
		isEnabled = false
	}

	if !isEnabled && expectedEnabled {
		unitPath := c.resolveUnitPath(name)
		if data, err := c.FS.ReadFile(unitPath); err == nil {
			if !strings.Contains(string(data), "[Install]") {
				isEnabled = true // static unit
			}
		}
	}

	if isEnabled != expectedEnabled {
		c.emitEvent("UnitEnableMismatch",
			fmt.Sprintf("Unit %s enable state mismatch: expected %t, actual %t",
				name, expectedEnabled, isEnabled))
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

// NeedLeaderElection returns false as OSC checks should run on every node independently.
func (c *OSCChecker) NeedLeaderElection() bool { return false }
