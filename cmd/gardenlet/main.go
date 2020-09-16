// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/gardener/gardener/cmd/gardenlet/app"
	"github.com/gardener/gardener/pkg/gardenlet/features"
)

func main() {
	features.RegisterFeatureGates()

	if err := exec.Command("which", "openvpn").Run(); err != nil {
		panic("openvpn is not installed or not executable. cannot start gardenlet.")
	}

	// Setup signal handler if running inside a Kubernetes cluster
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer func() {
		signal.Stop(c)
		cancel()
	}()
	go func() {
		<-c
		cancel()
		<-c
		os.Exit(1)
	}()

	command := app.NewCommandStartGardenlet(ctx)
	if err := command.Execute(); err != nil {
		panic(err)
	}
}
