// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fake

import (
	"context"
	"errors"

	"github.com/Masterminds/semver/v3"
	containerd "github.com/containerd/containerd/v2/client"
)

const defaultContainerdVersion = "2.1.2"

// Client is a fake containerd client used for testing
type Client struct {
	returnError bool
	version     string
}

// NewClient returns a new fake containerd client
func NewClient() *Client {
	return &Client{
		returnError: false,
		version:     defaultContainerdVersion,
	}
}

// Version returns the version of the (fake) containerd represented by the FakeContainerdClient
func (f Client) Version(_ context.Context) (containerd.Version, error) {
	if f.returnError {
		return containerd.Version{}, errors.New("calling fake containerd socket error")
	}
	return containerd.Version{
		Version:  f.version,
		Revision: f.version + "-fake",
	}, nil
}

// SetFakeContainerdVersion sets the version of the (fake) containerd to the desired value
func (f *Client) SetFakeContainerdVersion(version string) {
	semver.MustParse(version)
	f.version = version
}

// SetReturnError sets whether or not the fake containerd client returns an error
func (f *Client) SetReturnError(returnError bool) {
	f.returnError = returnError
}
