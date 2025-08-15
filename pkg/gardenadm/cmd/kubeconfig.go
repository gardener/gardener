// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/utils/ptr"
)

// DefaultKubeconfig sets the given kubeconfig pointer to the value of the KUBECONFIG environment variable, or to the
// default kubeconfig path in the user's home directory if KUBECONFIG is not set.
func DefaultKubeconfig(kubeconfig *string) error {
	if kubeconfig == nil {
		return fmt.Errorf("kubeconfig pointer must not be nil")
	}

	if ptr.Deref(kubeconfig, "") != "" {
		return nil
	}

	if kubeconfigEnv := os.Getenv("KUBECONFIG"); kubeconfigEnv != "" {
		*kubeconfig = os.Getenv("KUBECONFIG")
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}
	*kubeconfig = filepath.Join(homeDir, ".kube", "config")

	return nil
}
