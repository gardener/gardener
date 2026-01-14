// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TargetClient is an interface to be used to receive a target/shoot client.
type TargetClient interface {
	// InjectTargetClient injects the shoot client
	InjectTargetClient(client.Client)
}

// SourceClient is an interface to be used to receive a source/seed client.
type SourceClient interface {
	// InjectSourceClient injects the source client
	InjectSourceClient(client.Client)
}

// TargetClientInfo will set the target client on i if i implements TargetClient.
func TargetClientInfo(client client.Client, i any) bool {
	if s, ok := i.(TargetClient); ok {
		s.InjectTargetClient(client)
		return true
	}
	return false
}

// SourceClientInfo will set the source client on i if i implements SourceClient.
func SourceClientInfo(client client.Client, i any) bool {
	if s, ok := i.(SourceClient); ok {
		s.InjectSourceClient(client)
		return true
	}
	return false
}
