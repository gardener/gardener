# Generator test framework

The generator test framework provides a function whith the specs for a set of
tests that can be reused for testing generators.

The tests are based on comparing the output of the generator for a set
of pre-defined cloud-init files with a generator-specific output provided
in a test file.

Each Generator implementation can use this function as shown bellow:

```go
package my_generator_test

import (
	"embed"

	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/generator/test"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//go:embed /path/to/testfiles
var files embed.FS

var _ = Describe("My Generator Test", func(){
    Describe("Conformance Tests", 
        test.DescribeTest(NewGenerator(), files),
    )

    Describe("My other Tests", func(){
        // ...
    })
})
```
