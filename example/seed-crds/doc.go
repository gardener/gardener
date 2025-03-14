// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate ../../hack/generate-crds.sh -p 10-crd- extensions.gardener.cloud resources.gardener.cloud druid.gardener.cloud autoscaling.k8s.io monitoring.coreos.com_v1 monitoring.coreos.com_v1beta1 monitoring.coreos.com_v1alpha1 machine.sapcloud.io
//go:generate ../../hack/generate-crds.sh -p 10-crd- --allow-dangerous-types fluentbit.fluent.io

// Package seed_crds contains generated manifests for all CRDs that are present on a Seed cluster.
// Useful for development purposes.
package seed_crds
