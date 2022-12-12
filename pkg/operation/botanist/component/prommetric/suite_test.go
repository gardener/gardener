package prommetric

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPrometheusMetricsAdapter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PrometheusMetricsAdapter component unit test suite")
}
