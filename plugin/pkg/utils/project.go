// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// ProjectForNamespaceFromLister returns the Project responsible for a given <namespace>. It lists all Projects
// via the given lister, iterates over them and tries to identify the Project by looking for the namespace name
// in the project spec.
// Deprecated: Use github.com/gardener/gardener/pkg/utils/gardener.ProjectForNamespaceFromLister
// TODO(vpnachev): Remove this alias after v1.102.0 has been released.
var ProjectForNamespaceFromLister = gardenerutils.ProjectForNamespaceFromLister
