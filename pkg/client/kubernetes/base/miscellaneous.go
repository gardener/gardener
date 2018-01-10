// Copyright 2018 The Gardener Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubernetesbase

import (
	"k8s.io/client-go/rest"
)

// Curl performs an HTTP GET request to the API server and returns the result.
func (c *Client) Curl(path string) (*rest.Result, error) {
	res := c.
		RESTClient.
		Get().
		AbsPath(path).
		Do()
	err := res.Error()
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// QueryVersion queries the version of the API server and returns the GitVersion (e.g., v1.8.0).
func (c *Client) QueryVersion() (string, error) {
	version, err := c.
		Clientset.
		Discovery().
		ServerVersion()
	if err != nil {
		return "", err
	}
	return version.GitVersion, nil
}

// Version returns the GitVersion of the Kubernetes client stored on the object.
func (c *Client) Version() string {
	return c.version
}
