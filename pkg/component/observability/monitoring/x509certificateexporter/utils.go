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

func mappedStringsToArgs(argName string, values map[string]string) []string {
	vals := make([]string, 0, len(values))
	var value string = ""

	for k, v := range values {
		if v != "" {
			value = fmt.Sprintf("--%s=%s=%s", argName, k, v)
		} else {
			value = fmt.Sprintf("--%s=%s", argName, k)
		}
		vals = append(vals, value)
	}
	return vals
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

func includedLabelsAsArgs(includedLabels map[string]string) []string {
	return mappedStringsToArgs("include-label", includedLabels)
}

func excludedLabelsAsArgs(excludedLabels map[string]string) []string {
	return mappedStringsToArgs("exclude-label", excludedLabels)
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
