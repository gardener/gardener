// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0package x509certificateexporter

package x509certificateexporter

func stringsToArgs(argName string, values []string) []string {
	args := make([]string, len(values))
	for i, value := range values {
		args[i] = "--" + argName + "=" + value
	}
	return args
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

