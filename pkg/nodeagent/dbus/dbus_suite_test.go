package dbus_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDbus(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dbus")
}
