package rootcapublisher_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRootCAPublisher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RootCA Publisher Controller Suite")
}
