// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gardener/gardener/cmd/gardener-controller-manager/app"
	"github.com/gardener/gardener/pkg/controllermanager/features"
)

func main() {
	features.RegisterFeatureGates()

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

	command := app.NewCommandStartGardenerControllerManager(ctx, cancel)
	if err := command.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
