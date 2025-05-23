// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package files

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
)

// Copy copies a source file to destination file and sets the given permissions.
func Copy(fs afero.Afero, source, destination string, permissions os.FileMode) error {
	if destinationFileStat, err := fs.Stat(destination); err == nil {
		if !destinationFileStat.Mode().IsRegular() {
			return fmt.Errorf("destination %q exists but is not a regular file", destination)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := fs.MkdirAll(path.Dir(destination), permissions); err != nil {
		return fmt.Errorf("destination directory %q could not be created", path.Dir(destination))
	}

	tempDir, err := fs.TempDir(v1alpha1.TempDir, "copy-image-")
	if err != nil {
		return fmt.Errorf("error creating temp directory: %w", err)
	}

	defer func() { runtime.HandleError(fs.RemoveAll(tempDir)) }()

	tmpFilePath := filepath.Join(tempDir, filepath.Base(destination))

	if err := copyAndSync(fs, source, tmpFilePath); err != nil {
		return err
	}

	if err := fs.Chmod(tmpFilePath, permissions); err != nil {
		return err
	}

	return Move(fs, tmpFilePath, destination)
}

func copyAndSync(fs afero.Afero, source, destination string) error {
	sourceFileStat, err := fs.Stat(source)
	if err != nil {
		return err
	}
	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("source %q is not a regular file", source)
	}

	if destinationFileStat, err := fs.Stat(destination); err == nil {
		if !destinationFileStat.Mode().IsRegular() {
			return fmt.Errorf("destination %q exists but is not a regular file", destination)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	srcFile, err := fs.Open(source)
	if err != nil {
		return err
	}
	defer func() { runtime.HandleError(srcFile.Close()) }()

	if err := fs.MkdirAll(path.Dir(destination), os.ModeDir); err != nil {
		return fmt.Errorf("destination directory %q could not be created", path.Dir(destination))
	}

	dstFile, err := fs.OpenFile(destination, os.O_CREATE|os.O_RDWR|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() { runtime.HandleError(dstFile.Close()) }()

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	if err := fs.Chmod(dstFile.Name(), sourceFileStat.Mode()); err != nil {
		return err
	}

	return dstFile.Sync()
}

// CrossDeviceModeOnly is exposed for testing cross-device moving mode.
var CrossDeviceModeOnly = false

// Move moves files on the same or across different devices.
func Move(fs afero.Afero, source, destination string) error {
	sourceFileStat, err := fs.Stat(source)
	if err != nil {
		return err
	}
	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("source %q is not a regular file", source)
	}

	if destinationFileStat, err := fs.Stat(destination); err == nil {
		if !destinationFileStat.Mode().IsRegular() {
			return fmt.Errorf("destination %q exists but is not a regular file", destination)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if CrossDeviceModeOnly {
		return moveCrossDevice(fs, source, destination)
	}

	err = fs.Rename(source, destination)
	if err != nil && strings.Contains(err.Error(), "cross-device link") {
		return moveCrossDevice(fs, source, destination)
	}
	return err
}

func moveCrossDevice(fs afero.Afero, source, destination string) error {
	tmpFilePath := destination + ".tmp"

	if tmpFile, err := fs.Stat(tmpFilePath); err == nil && tmpFile.Mode().IsRegular() {
		if err := fs.Remove(tmpFilePath); err != nil {
			return fmt.Errorf("error removing previously existing temporary file %q: %w", tmpFilePath, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := copyAndSync(fs, source, tmpFilePath); err != nil {
		return err
	}

	if err := fs.Rename(tmpFilePath, destination); err != nil {
		return err
	}

	return fs.Remove(source)
}
