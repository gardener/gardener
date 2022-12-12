package hpva

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPrometheusMetricsAdapter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HPlusVAutoscaler component unit test suite")
}
