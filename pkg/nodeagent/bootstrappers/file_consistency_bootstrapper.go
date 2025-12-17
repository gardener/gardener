package bootstrappers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"

	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

const lastAppliedOSCPath = "/var/lib/gardener-node-agent/last-applied-osc.yaml"
const oscEventsPath = "/var/lib/gardener-node-agent/osc-events.yaml"

type OSCChecker struct {
	Log logr.Logger
	FS  afero.Afero
}

func (c *OSCChecker) Start(ctx context.Context) error {
	c.Log.Info("Starting OSC checker bootstrapper")

	// Load last applied OSC
	raw, err := c.FS.ReadFile(lastAppliedOSCPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.Log.Info("No last-applied OSC found, skipping")
			return nil
		}
		return fmt.Errorf("cannot read last-applied OSC: %w", err)
	}

	var osc v1alpha1.OperatingSystemConfig
	if err := yaml.Unmarshal(raw, &osc); err != nil {
		return fmt.Errorf("cannot parse OSC YAML: %w", err)
	}

	// File check
	for _, f := range osc.Spec.Files {
		c.checkFile(f)
	}

	// Unit Check
	for _, unit := range osc.Spec.Units {
		unitPath := filepath.Join("/etc/systemd/system", unit.Name)
		c.checkUnitFile(unitPath, &unit)

		// Drop-ins
		for _, di := range unit.DropIns {
			diPath := filepath.Join("/etc/systemd/system", unit.Name+".d", di.Name)
			c.checkDropInFile(diPath, &di, unit.Name)
		}

		// Enable/Disable
		if unit.Enable != nil {
			c.checkUnitEnabled(unit.Name, *unit.Enable)
		}

		// Dependency files
		for _, dep := range unit.FilePaths {
			if _, err := os.Stat(dep); os.IsNotExist(err) {
				c.Log.Info("DEPENDENCY MISSING", "unit", unit.Name, "file", dep)
				c.emitEvent(corev1.EventTypeWarning, "DependencyMissing", fmt.Sprintf("Dependency file %s for unit %s is missing", dep, unit.Name))
			}
		}
	}

	c.Log.Info("Finished OSC checking bootstrapper")
	return nil

}

// Check files
func (c *OSCChecker) checkFile(f v1alpha1.File) {
	if f.Content.Inline == nil {
		return
	}

	if f.Content.ImageRef != nil {
		return
	}

	inline := f.Content.Inline
	if inline == nil || inline.Data == "" {
		return
	}

	var expectedBytes []byte
	var decErr error
	switch inline.Encoding {
	case "", "plain":
		expectedBytes = []byte(inline.Data)
	case "b64", "base64":
		expectedBytes, decErr = utils.DecodeBase64(inline.Data)
		if decErr != nil {
			c.Log.Error(decErr, "Failed to decode inline base64", "path", f.Path)
			c.emitEvent(corev1.EventTypeWarning, "FileDecodeError", fmt.Sprintf("Failed to decode base64 content for file %s", f.Path))
			return
		}
	default:
		c.Log.Error(nil, "Unsupported encoding", "encoding", inline.Encoding, "path", f.Path)
		c.emitEvent(corev1.EventTypeWarning, "FileUnsupportedEncoding", fmt.Sprintf("Unsupported encoding %s for file %s", inline.Encoding, f.Path))
		return
	}

	expectedHash := utils.ComputeSHA256Hex(expectedBytes)
	realPath := filepath.Clean(f.Path)
	actualBytes, err := c.FS.ReadFile(realPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.Log.Info("FILE MISSING", "path", f.Path)
			c.emitEvent(corev1.EventTypeWarning, "FileMissing", fmt.Sprintf("File %s is missing", f.Path))
		} else {
			c.Log.Error(err, "Failed to read existing file", "path", f.Path)
			c.emitEvent(corev1.EventTypeWarning, "FileReadError", fmt.Sprintf("Failed to read file %s", f.Path))
		}
		return
	}

	actualHash := utils.ComputeSHA256Hex(actualBytes)
	if expectedHash == actualHash {
		c.Log.Info("FILE OK", "path", f.Path)
	} else {
		c.Log.Info("FILE MISMATCH", "path", f.Path, "expectedSHA256", expectedHash, "actualSHA256", actualHash)
		c.emitEvent(corev1.EventTypeWarning, "FileMismatch", fmt.Sprintf("File %s mismatch: expected SHA256 %s, actual SHA256 %s", f.Path, expectedHash, actualHash))
	}

}

// Check unit file content
func (c *OSCChecker) checkUnitFile(path string, unit *v1alpha1.Unit) {
	raw, err := os.ReadFile(path)
	if err != nil {
		c.Log.Info("UNIT FILE MISSING", "unit", unit.Name, "path", path)
		c.emitEvent(corev1.EventTypeWarning, "UnitFileMissing", fmt.Sprintf("Unit file %s is missing", unit.Name))
		return
	}

	if unit.Content == nil {
		c.Log.Info("UNIT CONTENT NOT SPECIFIED IN OSC (skipping content check)", "unit", unit.Name)
		return
	}

	expectedBytes := []byte(*unit.Content)
	actualBytes := raw
	expectedHash := utils.ComputeSHA256Hex(expectedBytes)
	actualHash := utils.ComputeSHA256Hex(actualBytes)

	if expectedHash == actualHash {
		c.Log.Info("UNIT FILE OK", "unit", unit.Name, "path", path)
	} else {
		c.Log.Info("UNIT CONTENT MISMATCH", "unit", unit.Name, "path", path, "expectedSHA256", expectedHash, "actualSHA256", actualHash)
		c.emitEvent(corev1.EventTypeWarning, "UnitMismatch", fmt.Sprintf("Unit %s content mismatch: expected SHA256 %s, actual SHA256 %s", unit.Name, expectedHash, actualHash))
	}

}

// Check drop-in files
func (c *OSCChecker) checkDropInFile(path string, di *v1alpha1.DropIn, unitName string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		c.Log.Info("DROP-IN MISSING", "path", path, "unit", unitName, "dropIn", di.Name)
		c.emitEvent(corev1.EventTypeWarning, "DropInMissing", fmt.Sprintf("Drop-in %s for unit %s is missing", di.Name, unitName))
		return
	}

	expectedBytes := []byte(di.Content)
	actualBytes := raw
	expectedHash := utils.ComputeSHA256Hex(expectedBytes)
	actualHash := utils.ComputeSHA256Hex(actualBytes)

	if expectedHash == actualHash {
		c.Log.Info("DROP-IN FILE OK", "unit", unitName, "dropIn", di.Name)
	} else {
		c.Log.Info("DROP-IN CONTENT MISMATCH", "unit", unitName, "dropIn", di.Name, "expectedSHA256", expectedHash, "actualSHA256", actualHash)
		c.emitEvent(corev1.EventTypeWarning, "DropInMismatch", fmt.Sprintf("Drop-in %s for unit %s mismatch: expected SHA256 %s, actual SHA256 %s", di.Name, unitName, expectedHash, actualHash))
	}

}

// Check enable/disable state
func (c *OSCChecker) checkUnitEnabled(name string, expect bool) {
	wantDir := filepath.Join("/etc/systemd/system/multi-user.target.wants", name)
	_, err := os.Stat(wantDir)
	isEnabled := err == nil

	if isEnabled == expect {
		c.Log.Info("UNIT ENABLE STATE OK", "unit", name, "enabled", expect)
	} else {
		c.Log.Info("UNIT ENABLE STATE MISMATCH", "unit", name, "expected", expect, "actual", isEnabled)
		c.emitEvent(corev1.EventTypeWarning, "UnitEnableMismatch", fmt.Sprintf("Unit %s enable state mismatch: expected %t, actual %t", name, expect, isEnabled))
	}

}

func (c *OSCChecker) emitEvent(eventType, reason, message string) {
	event := OSCEvent{
		Timestamp: time.Now().UTC(),
		Type:      eventType,
		Reason:    reason,
		Message:   message,
	}

	var list OSCEventList

	raw, err := c.FS.ReadFile(oscEventsPath)
	if err == nil {
		_ = yaml.Unmarshal(raw, &list)
	} else {
		list = OSCEventList{
			APIVersion: "nodeagent.gardener.cloud/v1alpha1",
			Kind:       "OSCEventList",
		}
	}

	list.Items = append(list.Items, event)

	out, err := yaml.Marshal(&list)
	if err != nil {
		c.Log.Error(err, "Failed to marshal OSC events")
		return
	}

	_ = c.FS.MkdirAll(filepath.Dir(oscEventsPath), 0o755)
	_ = c.FS.WriteFile(oscEventsPath, out, 0o644)
}

func (c *OSCChecker) NeedLeaderElection() bool { return false }
