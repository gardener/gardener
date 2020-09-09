package main

import (
	"fmt"
	"os"

	"github.com/gardener/gardener/cmd/gardener-admission-controller/app"
	"github.com/gardener/gardener/cmd/utils"

	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

func main() {
	ctx := utils.ContextFromStopChannel(signals.SetupSignalHandler())
	command := app.NewCommandStartGardenerAdmissionController(ctx)

	if err := command.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
