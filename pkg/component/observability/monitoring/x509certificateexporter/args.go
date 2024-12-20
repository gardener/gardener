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

// X509CertificateArg is implementd by objects, tranformable to x509 certificate exporter args
type X509CertificateArg interface {
	// AsArg should return the argument as a string, including the `--` prefix
	AsArg() string
}

// X509CertificateArgSet is interface for objects that group multiple x509 certificate exporter args
type X509CertificateArgSet interface {
	AsArgs() []string
}

// Filepath to a certificate on the node
type CertificatePath string

// AsArg returns the certificate path as an argument
func (c CertificatePath) AsArg() string {
	return "--watch-file=" + string(c)
}

// CertificateDirPath is a path to a directory containing certificates on the node
type CertificateDirPath string

// AsArg returns the certificate dir path as an argument
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

// NewHostCertificates produces `*hostCertificates`,
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

// AsArgs returns the host certificates as arguments
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

// SecretType groups Secret types and the key name contained within that secret
// to provide an argument for the x509 certificate exporter
type SecretType struct {
	// Type of the secrets that should be searched
	Type string
	// Key within the secret that should be checked
	Key string
}

func (s SecretType) String() string {
	return s.Type + ":" + s.Key
}

// AsArg returns the secret type as an argument
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

// Note: Removes duplicates
func listToArgs(argPrefix string, data []string) []string {
	var (
		allKeys = make(map[string]bool, len(data))
	)
	for _, arg := range data {
		allKeys[arg] = true
	}

	var (
		args      = make([]string, len(allKeys))
		idx  uint = 0
	)

	for arg := range allKeys {
		args[idx] = argPrefix + arg
		idx++
	}
	return args
}

// IncludeLabels are labels used to filter certificates from the k8s API.
type IncludeLabels labels.Set

func (il IncludeLabels) AsArgs() []string {
	return labelsToArgs("--include-label=", map[string]string(il))
}

// ExcludeLabels are labels used to filter certificates from the k8s API.
type ExcludeLabels labels.Set

func (el ExcludeLabels) AsArgs() []string {
	return labelsToArgs("--exclude-label=", el)
}

// ExcludeNamespaces are namespaces used to filter out secrets from specific namespaces.
type ExcludeNamespaces []string

func (en ExcludeNamespaces) AsArgs() []string {
	return listToArgs("--exclude-namespace=", en)
}

// IncludeNamespaces are namespaces used to filter secrets from specific namespaces.
type IncludeNamespaces []string

func (in IncludeNamespaces) AsArgs() []string {
	return listToArgs("--include-namespace=", in)
}

// ConfigMapKeys are keys, containing the certificate data in the config maps
type ConfigMapKeys []string

func (c ConfigMapKeys) AsArgs() []string {
	return listToArgs("--configmap-key=", c)
}
