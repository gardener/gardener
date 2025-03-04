// Copyright 2021 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// This file was copied from the kubernetes-sigs/controller-runtime project
// https://github.com/kubernetes-sigs/controller-runtime/blob/v0.8.0/pkg/internal/testing/integration/addr/manager.go
//
// Modifications Copyright 2024 SAP SE or an SAP affiliate company and Gardener contributors

package net

import (
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	portReserveTime   = 1 * time.Minute
	portConflictRetry = 100
)

type portCache struct {
	lock  sync.Mutex
	ports map[int]time.Time
}

func (c *portCache) add(port int) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	// remove outdated port
	for p, t := range c.ports {
		if time.Since(t) > portReserveTime {
			delete(c.ports, p)
		}
	}
	// try allocating new port
	if _, ok := c.ports[port]; ok {
		return false
	}
	c.ports[port] = time.Now()
	return true
}

var cache = &portCache{
	ports: make(map[int]time.Time),
}

func suggest(listenHost string) (port int, resolvedHost string, err error) {
	if listenHost == "" {
		listenHost = "localhost"
	}
	addr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(listenHost, "0"))
	if err != nil {
		return
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return
	}
	port = l.Addr().(*net.TCPAddr).Port
	defer func() {
		err = l.Close()
	}()
	resolvedHost = addr.IP.String()
	return
}

// SuggestPort suggests an address a process can listen on. It returns
// a tuple consisting of a free port and the hostname resolved to its IP.
// It makes sure that new port allocated does not conflict with old ports
// allocated within 1 minute.
func SuggestPort(listenHost string) (port int, resolvedHost string, err error) {
	for i := 0; i < portConflictRetry; i++ {
		port, resolvedHost, err = suggest(listenHost)
		if err != nil {
			return
		}
		if cache.add(port) {
			return
		}
	}
	err = fmt.Errorf("no free ports found after %d retries", portConflictRetry)
	return
}
