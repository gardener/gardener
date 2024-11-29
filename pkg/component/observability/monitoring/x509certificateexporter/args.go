// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package x509certificateexporter

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"k8s.io/apimachinery/pkg/labels"
)

type X509CertificateArg interface {
	AsArg() string
}

type X509CertificateArgSet interface {
	AsArgs() []string
}

type CertificatePath string

func (c CertificatePath) AsArg() string {
	return "--watch-file=" + string(c)
}

type CertificateDirPath string

func (c CertificateDirPath) AsArg() string {
	return "--watch-dir=" + string(c)
}

// HostCertificates describes certificates that will be configured for monitoring
// from the host nodes
type HostCertificates struct {
	// MountPath is the host path that will be mounted
	MountPath string
	// Certificate paths is a list of certificates withion the specified mount
	// All relative paths are configured base on the specified mount
	CertificatePaths []CertificatePath
	// Similat to CertificatePaths but for dirs
	CertificateDirPaths []CertificateDirPath
}

// Produces `*hostCertificates`,
// will fail if mountPath is not an absolute dir
// if any certificatePath is not an abs path, mountPath will be prepend
// mountPath: host path that will be mounted from the node
// filePaths: paths that will be configured in certificate exporter ds. Paths can be either file paths or dirs. If not absolute - mountPath is prepended.
// dirPaths: similar as above, but will be configured as dirs
func NewHostCertificates(
	mountPath string, filePaths []string, dirPaths []string,
) (*HostCertificates, error) {
	var (
		ensureAbsolutePaths = func(paths []string) {
			for idx, path := range paths {
				if !filepath.IsAbs(path) {
					paths[idx] = filepath.Join(mountPath, path)
				}
			}
			sort.Strings(paths)
		}
		certificateDirs  []CertificateDirPath
		certificatePaths []CertificatePath
	)

	if !filepath.IsAbs(mountPath) {
		return nil, errors.New("Path " + mountPath + "is not absolute file path")
	}
	ensureAbsolutePaths(filePaths)
	if len(dirPaths) == 0 {
		dirPaths = []string{mountPath}
	}
	ensureAbsolutePaths(dirPaths)

	certificateDirs = make([]CertificateDirPath, len(dirPaths))
	for i, path := range dirPaths {
		certificateDirs[i] = CertificateDirPath(path)
	}

	certificatePaths = make([]CertificatePath, len(filePaths))
	for i, path := range filePaths {
		certificatePaths[i] = CertificatePath(path)
	}

	return &HostCertificates{
		MountPath:           mountPath,
		CertificatePaths:    certificatePaths,
		CertificateDirPaths: certificateDirs,
	}, nil
}

func (h HostCertificates) AsArgs() []string {
	var (
		offset = len(h.CertificatePaths)
		args   = make([]string, offset+len(h.CertificateDirPaths))
	)

	for idx, arg := range h.CertificatePaths {
		args[idx] = arg.AsArg()
	}
	for idx, arg := range h.CertificateDirPaths {
		args[offset+idx] = arg.AsArg()
	}
	return args
}

// Secret types and the key name contained within that secret
type SecretType struct {
	// Type of the secrets that should be searched
	Type string
	// Key within the secret that should be checked
	Key string
}

func (s SecretType) String() string {
	return s.Type + ":" + s.Key
}

func (s SecretType) AsArg() string {
	return fmt.Sprintf("--secret-type=%s", s)
}

type SecretTypeList []SecretType

func (s SecretTypeList) AsArgs() []string {
	var (
		args = make([]string, len(s))
	)
	for idx, arg := range s {
		args[idx] = arg.AsArg()
	}
	return args
}

func labelsToArgs(argPrefix string, data map[string]string) []string {
	var (
		args = []string{}
	)
	for k, v := range data {
		arg := argPrefix + k
		if v != "" {
			arg += "=" + v
		}
		args = append(args, arg)
	}
	sort.Strings(args)
	return args
}

type IncludeLabels labels.Set

func (il IncludeLabels) AsArgs() []string {
	return labelsToArgs("--include-label=", map[string]string(il))
}

type ExcludeLabels labels.Set

func (el ExcludeLabels) AsArgs() []string {
	return labelsToArgs("--exclude-label=", el)
}

type ExcludeNamespaces []string

func (en ExcludeNamespaces) AsArgs() []string {
	var (
		args = make([]string, len(en))
	)
	for idx, arg := range en {
		args[idx] = "--exclude-namespace=" + arg
	}
	return args
}

type IncludeNamespaces []string

func (in IncludeNamespaces) AsArgs() []string {
	var (
		args = make([]string, len(in))
	)
	for idx, arg := range in {
		args[idx] = "--include-namespace=" + arg
	}
	return args
}
