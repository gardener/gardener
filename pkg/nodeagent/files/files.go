// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	defer func() { runtime.HandleError(fs.Remove(tempDir)) }()

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

	dstFile, err := fs.OpenFile(destination, os.O_CREATE|os.O_RDWR, 0755)
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

	if err := copyAndSync(fs, source, tmpFilePath); err != nil {
		return err
	}

	if err := fs.Rename(tmpFilePath, destination); err != nil {
		return err
	}

	return fs.Remove(source)
}
