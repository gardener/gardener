// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

//go:generate ../../hack/generate-crds.sh -p 10-crd- extensions.gardener.cloud resources.gardener.cloud druid.gardener.cloud hvpaautoscaling.k8s.io autoscaling.k8s.io fluentbit.fluent.io machine.sapcloud.io

// Package seed_crds contains generated manifests for all CRDs that are present on a Seed cluster.
// Useful for development purposes.
package seed_crds
