# Generator test framework

The generator test framework provides a function whith the specs for a set of
tests that can be reused for testing generators.

The tests are based on comparing the output of the generator for a set
of pre-defined cloud-init files with a generator-specific output provided
in a test file.

Each Generator implementation can use this function as shown bellow:

```go
import (
	"github.com/gobuffalo/packr"
	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/os-common/generator/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)
         
var _ Describe("My Generator Test", func(){
      var box packr.Box

      BeforeSuite(func() {
     	box = packr.NewBox("/path/to/testfiles")
      })

      Describe("Conformance Tests", test.DescribeTest(NewGenerator(),box))

      Describe("My other Tests", func(){
       ...
      })
 })
```
