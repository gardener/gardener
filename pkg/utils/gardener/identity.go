// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"k8s.io/component-base/version"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils"
)

// DetermineIdentity determines the Gardener component identity.
// We want to determine the Docker container id of the currently running instance because we need to identify for still
// ongoing operations whether another instance is still operating the respective Shoots. When running locally, we
// generate a random string because there is no container id.
func DetermineIdentity() (*gardencorev1beta1.Gardener, error) {
	var (
		validID = regexp.MustCompile(`([0-9a-f]{64})`)
		id      string

		name string
		err  error
	)

	name, err = os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("unable to get hostname: %w", err)
	}

	// If running inside a Kubernetes cluster (as container) we can read the container id from the proc file system.
	// Otherwise generate a random string for the id
	if cGroupFile, err := os.Open("/proc/self/cgroup"); err == nil {
		defer cGroupFile.Close()
		reader := bufio.NewReader(cGroupFile)

		var cgroupV1 string
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}

			// Store cgroup-v1 result for fall back
			if strings.HasPrefix(line, "1:name=systemd") {
				cgroupV1 = line
			}

			// Always prefer cgroup-v2
			if strings.HasPrefix(line, "0::") {
				if containerID := extractID(line); validID.MatchString(containerID) {
					id = containerID
					break
				}
			}
		}

		// Fall-back to cgroup-v1 if possible
		if len(id) == 0 && len(cgroupV1) > 0 {
			id = extractID(cgroupV1)
		}
	}

	if id == "" {
		id, err = utils.GenerateRandomString(64)
		if err != nil {
			return nil, fmt.Errorf("unable to generate id: %w", err)
		}
	}

	return &gardencorev1beta1.Gardener{
		ID:      id,
		Name:    name,
		Version: version.Get().GitVersion,
	}, nil
}

func extractID(line string) string {
	var (
		id           string
		splitBySlash = strings.Split(line, "/")
	)

	if len(splitBySlash) == 0 {
		return ""
	}

	id = strings.TrimSpace(splitBySlash[len(splitBySlash)-1])
	id = strings.TrimSuffix(id, ".scope")
	id = strings.TrimPrefix(id, "docker-")

	return id
}
