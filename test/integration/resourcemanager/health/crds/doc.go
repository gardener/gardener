// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate ../../../../../hack/generate-crds.sh -p 10-crd- cert.gardener.cloud
package crds

import (
	_ "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
)
