package unmanagedinfra

import (
	"context"
	"io"
	"os/exec"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

// RunInMachine executes a command inside a machine container identified by its ordinal.
func RunInMachine(ctx context.Context, ordinal int, command ...string) (*gbytes.Buffer, *gbytes.Buffer, error) {
	return dockerCommand(ctx, append([]string{"exec", machineContainerName(ordinal)}, command...)...)
}

// RunInNode executes a command inside a node container identified by its name.
func RunInNode(ctx context.Context, nodeName string, command ...string) (*gbytes.Buffer, *gbytes.Buffer, error) {
	return dockerCommand(ctx, append([]string{"exec", nodeName}, command...)...)
}

func dockerCommand(ctx context.Context, args ...string) (*gbytes.Buffer, *gbytes.Buffer, error) {
	var stdOutBuffer, stdErrBuffer = gbytes.NewBuffer(), gbytes.NewBuffer()

	cmd := exec.CommandContext(ctx, "docker", args...) // #nosec G204 -- Used for e2e tests only.
	cmd.Stdout = io.MultiWriter(stdOutBuffer, gexec.NewPrefixedWriter("[out] ", GinkgoWriter))
	cmd.Stderr = io.MultiWriter(stdErrBuffer, gexec.NewPrefixedWriter("[err] ", GinkgoWriter))

	return stdOutBuffer, stdErrBuffer, cmd.Run()
}

func machineContainerName(ordinal int) string {
	return "gind-machine-" + strconv.Itoa(ordinal)
}
