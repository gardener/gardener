// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package files

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/afero"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
)

const (
	// OperationCreated represents the file system operation of creating an object.
	OperationCreated FileSystemOperation = "created"
	// OperationModified represents the file system operation of modifying an object.
	OperationModified FileSystemOperation = "modified"
	// OperationDeleted represents the file system operation of deleting an object.
	OperationDeleted FileSystemOperation = "deleted"
	// OperationNone represents the file system operation of the object not being touched.
	OperationNone FileSystemOperation = ""

	nodeAgentFileSystemPath = nodeagentv1alpha1.BaseDir + "/node-agent-filesystem.yaml"
	getStateErrorMessage    = "unable to get state of %q: %w"
	saveStateErrorMessage   = "unable to save state of %q: %w"
)

// NodeAgentAfero is similar to afero.Afero but offers additional functionality to track file operations.
type NodeAgentAfero struct {
	afero.Afero
	NodeAgentFileSystem
}

// NewNodeAgentAfero creates a new NodeAgentAfero for a given afero.Fs.
func NewNodeAgentAfero(fs afero.Fs) (*NodeAgentAfero, error) {
	nodeAgentFileSystem, err := NewNodeAgentFileSystem(afero.Afero{Fs: fs})
	if err != nil {
		return nil, fmt.Errorf("unable to create NodeAgentFileSystem: %w", err)
	}

	return &NodeAgentAfero{Afero: afero.Afero{Fs: nodeAgentFileSystem}, NodeAgentFileSystem: nodeAgentFileSystem}, nil
}

// NodeAgentFileSystem is a file system that keeps track of the file operations performed on the files.
type NodeAgentFileSystem interface {
	afero.Fs
	// GetFileSystemOperation returns the file operation performed by this file system.
	GetFileSystemOperation(path string) FileSystemOperation
	// RemoveCreated removes the file from the file system if it was created by this file system.
	RemoveCreated(name string) error
	// RemoveAllCreated removes all files created by this file system in the given directory.
	RemoveAllCreated(path string) error
}

// FileSystemOperation represents the file operation performed by NodeAgentFileSystem.
type FileSystemOperation string

// NewNodeAgentFileSystem creates a new NodeAgentFileSystem.
func NewNodeAgentFileSystem(fs afero.Afero) (NodeAgentFileSystem, error) {
	fsOperationsRaw, err := fs.ReadFile(nodeAgentFileSystemPath)
	if errors.Is(err, afero.ErrFileNotFound) {
		return &nodeAgentFileSystem{
			fs: fs,
			nodeAgentFsOperations: nodeAgentFsOperations{
				FileSystemOperations: map[string]FileSystemOperation{},
			},
		}, nil
	} else if err != nil {
		return nil, fmt.Errorf("unable to read file system state file %q: %w", nodeAgentFileSystemPath, err)
	}

	fsOperations := nodeAgentFsOperations{}
	err = yaml.Unmarshal(fsOperationsRaw, &fsOperations)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal file system state file %q: %w", nodeAgentFileSystemPath, err)
	}

	return &nodeAgentFileSystem{
		fs:                    fs,
		nodeAgentFsOperations: fsOperations,
	}, nil
}

type nodeAgentFsOperations struct {
	FileSystemOperations map[string]FileSystemOperation `json:"fileSystemOperations"`
}

type nodeAgentFileSystem struct {
	fs                    afero.Afero
	mutex                 sync.Mutex
	nodeAgentFsOperations nodeAgentFsOperations
}

// GetFileSystemOperation returns the file operation performed by this file system.
func (n *nodeAgentFileSystem) GetFileSystemOperation(path string) FileSystemOperation {
	return n.nodeAgentFsOperations.FileSystemOperations[path]
}

// RemoveCreated removes the file from the file system if it was created by this file system.
func (n *nodeAgentFileSystem) RemoveCreated(name string) error {
	if operation := n.nodeAgentFsOperations.FileSystemOperations[name]; operation != OperationCreated {
		return nil
	}

	return n.Remove(name)
}

// RemoveAllCreated removes all files created by this file system in the given directory.
func (n *nodeAgentFileSystem) RemoveAllCreated(path string) error {
	isDir, err := n.fs.IsDir(path)
	if err != nil {
		return fmt.Errorf("unable to check if %q is a directory: %w", path, err)
	}

	if !isDir {
		if err := n.Remove(path); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
			return fmt.Errorf("unable to remove file %q: %w", path, err)
		}

		return nil
	}

	files, err := n.fs.ReadDir(path)
	if err != nil {
		return fmt.Errorf("unable to read directory %q: %w", path, err)
	}

	for _, file := range files {
		fileName := filepath.Join(path, file.Name())

		if n.GetFileSystemOperation(fileName) != OperationCreated {
			continue
		}

		if file.IsDir() {
			if err := n.RemoveAll(fileName); err != nil {
				return fmt.Errorf("unable to remove directory %q: %w", fileName, err)
			}
			continue
		}

		if err := n.Remove(fileName); err != nil {
			return fmt.Errorf("unable to remove file %q: %w", fileName, err)
		}
	}

	if empty, err := n.fs.IsEmpty(path); err != nil {
		return fmt.Errorf("unable to check if directory %q is empty: %w", path, err)
	} else if empty && n.GetFileSystemOperation(path) == OperationCreated {
		return n.RemoveAll(path)
	}

	return nil
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens.
func (n *nodeAgentFileSystem) Create(name string) (afero.File, error) {
	operation, err := n.getState(name)
	if err != nil {
		return nil, fmt.Errorf(getStateErrorMessage, name, err)
	}
	file, err := n.fs.Create(name)
	if err != nil {
		return file, err
	}

	if err := n.saveState(name, operation); err != nil {
		return nil, fmt.Errorf("unable to save state for file %q: %w", name, err)
	}

	return file, nil
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (n *nodeAgentFileSystem) Mkdir(name string, perm os.FileMode) error {
	operation, err := n.getState(name)
	if err != nil {
		return fmt.Errorf(getStateErrorMessage, name, err)
	}

	if err := n.fs.Mkdir(name, perm); err != nil {
		return err
	}

	if err := n.saveState(name, operation); err != nil {
		return fmt.Errorf(saveStateErrorMessage, name, err)
	}

	return nil
}

// MkdirAll creates a directory path and all parents that does not exist
// yet.
func (n *nodeAgentFileSystem) MkdirAll(path string, perm os.FileMode) error {
	operation, err := n.getState(path)
	if err != nil {
		return fmt.Errorf(getStateErrorMessage, path, err)
	}
	pathOperation := map[string]*FileSystemOperation{path: operation}

	for i := range len(path) {
		if path[i] == filepath.Separator {
			subPath := path[:i]
			if len(subPath) == 0 {
				continue
			}
			operation, err := n.getState(subPath)
			if err != nil {
				return fmt.Errorf(getStateErrorMessage, subPath, err)
			}
			pathOperation[subPath] = operation
		}
	}

	if err := n.fs.MkdirAll(path, perm); err != nil {
		return err
	}

	for path, operation := range pathOperation {
		if err := n.saveState(path, operation); err != nil {
			return fmt.Errorf(saveStateErrorMessage, path, err)
		}
	}

	return nil
}

// Open opens a file, returning it or an error, if any happens.
func (n *nodeAgentFileSystem) Open(name string) (afero.File, error) {
	return n.fs.Open(name)
}

// OpenFile opens a file using the given flags and the given mode.
func (n *nodeAgentFileSystem) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if flag == os.O_RDONLY {
		return n.fs.OpenFile(name, flag, perm)
	}

	operation, err := n.getState(name)
	if err != nil {
		return nil, fmt.Errorf(getStateErrorMessage, name, err)
	}

	file, err := n.fs.OpenFile(name, flag, perm)
	if err != nil {
		return file, err
	}

	if err := n.saveState(name, operation); err != nil {
		return nil, fmt.Errorf(saveStateErrorMessage, name, err)
	}

	return file, nil
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (n *nodeAgentFileSystem) Remove(name string) error {
	if err := n.fs.Remove(name); err != nil {
		return err
	}

	if err := n.deleteState(name); err != nil {
		return fmt.Errorf(saveStateErrorMessage, name, err)
	}

	return nil
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
func (n *nodeAgentFileSystem) RemoveAll(path string) error {
	if err := n.fs.RemoveAll(path); err != nil {
		return err
	}

	if err := n.deleteState(path); err != nil {
		return fmt.Errorf(saveStateErrorMessage, path, err)
	}

	for nodeAgentFile := range n.nodeAgentFsOperations.FileSystemOperations {
		if !strings.HasPrefix(nodeAgentFile, path) {
			continue
		}
		if err := n.deleteState(nodeAgentFile); err != nil {
			return fmt.Errorf(saveStateErrorMessage, nodeAgentFile, err)
		}
	}

	return nil
}

// Rename renames a file.
func (n *nodeAgentFileSystem) Rename(oldname, newname string) error {
	operation, err := n.getState(newname)
	if err != nil {
		return fmt.Errorf(getStateErrorMessage, newname, err)
	}

	if err := n.fs.Rename(oldname, newname); err != nil {
		return err
	}

	if err := n.saveState(newname, operation); err != nil {
		return fmt.Errorf(saveStateErrorMessage, newname, err)
	}

	if err := n.deleteState(oldname); err != nil {
		return fmt.Errorf(saveStateErrorMessage, oldname, err)
	}

	return nil
}

// Stat returns a FileInfo describing the named file, or an error, if any
// happens.
func (n *nodeAgentFileSystem) Stat(name string) (os.FileInfo, error) {
	return n.fs.Stat(name)
}

// Name returns the name of this FileSystem
func (n *nodeAgentFileSystem) Name() string {
	return "NodeAgentFileSystem"
}

// Chmod changes the mode of the named file to mode.
func (n *nodeAgentFileSystem) Chmod(name string, mode os.FileMode) error {
	return n.fs.Chmod(name, mode)
}

// Chown changes the uid and gid of the named file.
func (n *nodeAgentFileSystem) Chown(name string, uid, gid int) error {
	return n.fs.Chown(name, uid, gid)
}

// Chtimes changes the access and modification times of the named file
func (n *nodeAgentFileSystem) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return n.fs.Chtimes(name, atime, mtime)
}

func (n *nodeAgentFileSystem) getState(path string) (*FileSystemOperation, error) {
	if operation, ok := n.nodeAgentFsOperations.FileSystemOperations[path]; operation == OperationDeleted {
		return ptr.To(OperationDeleted), nil
	} else if ok {
		return nil, nil
	}

	exists, err := n.fs.Exists(path)
	if err != nil {
		return nil, fmt.Errorf("unable to check if path %q exists: %w", path, err)
	}

	operation := OperationCreated
	if exists {
		if isDir, err := n.fs.IsDir(path); err != nil {
			return nil, fmt.Errorf("unable to check if path %q is a directory: %w", path, err)
		} else if isDir {
			// A directory cannot be modified but only its content.
			return nil, nil
		}
		operation = OperationModified
	}

	return &operation, nil
}

func (n *nodeAgentFileSystem) deleteState(path string) error {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	if operation := n.nodeAgentFsOperations.FileSystemOperations[path]; operation == OperationCreated {
		delete(n.nodeAgentFsOperations.FileSystemOperations, path)
	} else {
		n.nodeAgentFsOperations.FileSystemOperations[path] = OperationDeleted
	}

	return n.marshallStateAndSave()
}

func (n *nodeAgentFileSystem) saveState(path string, operation *FileSystemOperation) error {
	if operation == nil {
		return nil
	}

	n.mutex.Lock()
	defer n.mutex.Unlock()

	n.nodeAgentFsOperations.FileSystemOperations[path] = *operation

	return n.marshallStateAndSave()
}

func (n *nodeAgentFileSystem) marshallStateAndSave() error {
	nodeAgentFilesRaw, err := yaml.Marshal(n.nodeAgentFsOperations)
	if err != nil {
		return fmt.Errorf("unable to marshal file system state file %q: %w", nodeAgentFileSystemPath, err)
	}

	if err = n.fs.WriteFile(nodeAgentFileSystemPath, nodeAgentFilesRaw, 0600); err != nil {
		return fmt.Errorf("unable to write file system state file %q: %w", nodeAgentFileSystemPath, err)
	}

	return nil
}
