// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -package events -destination=mocks.go k8s.io/client-go/tools/events EventRecorder

package events
