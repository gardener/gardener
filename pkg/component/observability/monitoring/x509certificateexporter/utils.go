// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0package x509certificateexporter

package x509certificateexporter

import (
	"fmt"
	"path/filepath"
	"strings"
)

func stringsToArgs(argName string, values []string) []string {
	for _, value := range values {
		value = "--" + argName + "=" + value
	}
	return values
}
func stringsToPathArgs(argName string, values []string) ([]string, error) {
	for _, value := range values {
		if !filepath.IsAbs(value) {
			return nil, fmt.Errorf("value %s is not an absolute path", value)
		}
		value = "--" + argName + "=" + value
	}
	return values, nil
}

func secretTypesAsArgs(secretTypes []string) []string {
	return stringsToArgs("secret-type", secretTypes)
}

func configMapKeysAsArgs(configMapKeys []string) []string {
	return stringsToArgs("configmap-key", configMapKeys)
}

func includedLabelsAsArgs(includedLabels []string) []string {
	return stringsToArgs("include-label", includedLabels)
}

func excludedLabelsAsArgs(excludedLabels []string) []string {
	return stringsToArgs("exclude-label", excludedLabels)
}

func includedNamespacesAsArgs(includedNamespaces []string) []string {
	return stringsToArgs("include-namespace", includedNamespaces)
}

func excludedNamespacesAsArgs(excludedNamespaces []string) []string {
	return stringsToArgs("exclude-namespace", excludedNamespaces)
}

func getCertificateFileAsArg(filenames []string) ([]string, error) {
	return stringsToPathArgs("watch-file", filenames)
}

func getCertificateDirAsArg(directories []string) ([]string, error) {
	return stringsToPathArgs("watch-dir", directories)
}

func getPathArgs(paths []string) ([]string, []string, error) {
	var (
		certificateFileArgs []string
		certificateDirArgs  []string
		err                 error
	)
	for _, path := range paths {
		if strings.HasSuffix(path, "/") {
			certificateDirArgs = append(certificateDirArgs, path)
			continue
		}
		certificateFileArgs = append(certificateFileArgs, path)
	}
	if certificateFileArgs, err = getCertificateFileAsArg(certificateFileArgs); err != nil {
		return nil, nil, err
	}
	if certificateDirArgs, err = getCertificateDirAsArg(certificateDirArgs); err != nil {
		return nil, nil, err
	}
	return certificateFileArgs, certificateDirArgs, nil
}
