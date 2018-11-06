package reconcilescheduler_test

import (
	"fmt"

	. "github.com/gardener/gardener/pkg/utils/reconcilescheduler"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

const (
	reconcileAllowed    = true
	reconcileNotAllowed = false

	specModified = 1
	isReserved   = 2
)

var _ = Describe("reconcilescheduler", func() {
	Context("Interface", func() {
		var (
			cycleAid = newID("cycleA")
			cycleBid = newID("cycleB")
			cycleCid = newID("cycleC")
			soilAid  = newID("soilA")
			seedAid  = newID("seedA")
			seedBid  = newID("seedB")
			shootAid = newID("shootA")
			shootBid = newID("shootB")
			shootCid = newID("shootC")

			cycleA = newElement(&cycleAid, &cycleCid)
			cycleB = newElement(&cycleBid, &cycleAid)
			cycleC = newElement(&cycleCid, &cycleBid)
			seedA  = newElement(&seedAid, &soilAid)
			seedB  = newElement(&seedBid, &seedAid)
			shootA = newElement(&shootAid, &seedAid)
			shootB = newElement(&shootBid, &seedBid)
			shootC = newElement(&shootCid, &seedBid)
		)

		Context("#TestAndActivate", func() {
			var (
				scheduler = New(nil)
				logger    *loggerType

				setupSeeds = []operationTest{
					t(seedA, reconcileAllowed),
					d(seedA),
					t(seedB, reconcileAllowed),
					d(seedB),
				}
			)

			BeforeEach(func() {
				scheduler.Reset()
				scheduler.MarkStatic(soilAid)
				logger = &loggerType{}
			})

			DescribeTable("test scheduling decisions",
				func(schedule []operationTest) {
					for i, o := range schedule {
						Expect(o.Execute(scheduler, logger)).To(Equal(o.Expected()), "i=%d (%s) => \n\n%s", i, o, logger.String())
					}
				},

				Entry("cycle with three nodes", []operationTest{
					t(cycleA, reconcileNotAllowed),
					t(cycleB, reconcileNotAllowed),
					t(cycleC, reconcileAllowed),
					d(cycleC),
					t(cycleB, reconcileAllowed),
					t(cycleA, reconcileNotAllowed),
					d(cycleB),
					t(cycleA, reconcileAllowed),
					d(cycleA),
				}),

				Entry("seeds with static parent", []operationTest{
					t(seedA, reconcileAllowed),
					t(seedB, reconcileNotAllowed),
					d(seedA),
					t(seedB, reconcileAllowed),
				}),

				Entry("shoot and seeds", []operationTest{
					t(shootA, reconcileNotAllowed),
					t(seedA, reconcileAllowed),
					t(shootA, reconcileNotAllowed),
					d(seedA),
					t(shootA, reconcileAllowed),
				}),

				Entry("seeds with modification", append(
					setupSeeds,
					t(seedB, reconcileAllowed),
					d(seedB),
					t(seedB, reconcileAllowed, specModified),
				)),

				Entry("shoots and seeds", append(
					setupSeeds,
					t(shootA, reconcileAllowed),
					t(shootB, reconcileAllowed),
					t(shootC, reconcileAllowed),
					d(shootA),
					d(shootB),
					d(shootC),
				)),

				Entry("no concurrent shoots and seeds", []operationTest{
					t(seedA, reconcileAllowed),
					d(seedA),
					t(seedB, reconcileAllowed),
					t(shootB, reconcileNotAllowed),
					t(shootC, reconcileNotAllowed),
					t(shootA, reconcileAllowed),
					d(shootA),
					d(seedB),
					t(shootB, reconcileAllowed),
					t(shootC, reconcileAllowed),
					d(shootB),
					d(shootC),
				}),

				Entry("children block waiting parent", append(
					setupSeeds,
					t(shootB, reconcileAllowed),
					t(seedB, reconcileNotAllowed, specModified),
					t(shootC, reconcileAllowed),
					t(seedB, reconcileNotAllowed, specModified),
					d(shootB),
					t(seedB, reconcileNotAllowed, specModified),
					d(shootC),
					t(seedB, reconcileAllowed, specModified),
				)),

				Entry("waiting parent blocks further children", append(
					setupSeeds,
					t(shootB, reconcileAllowed),
					t(seedB, reconcileNotAllowed, specModified, isReserved),
					t(shootC, reconcileNotAllowed),
					t(seedB, reconcileNotAllowed, specModified),
					d(shootB),
					t(seedB, reconcileAllowed, specModified),
					t(shootC, reconcileNotAllowed),
					d(seedB),
					t(shootC, reconcileAllowed),
					d(shootC),
				)),

				Entry("modified seeds at beginning", []operationTest{
					t(seedA, reconcileAllowed),
					d(seedA),
					t(seedB, reconcileAllowed, specModified),
					t(shootC, reconcileNotAllowed),
					d(seedB),
					t(shootC, reconcileAllowed),
					t(seedB, reconcileNotAllowed, specModified),
					d(shootC),
					t(seedB, reconcileAllowed, specModified),
				}),

				Entry("no shoots before seeds", append(
					setupSeeds,
					t(shootA, reconcileAllowed),
					d(shootA),
					t(shootA, reconcileAllowed),
				)),

				Entry("initial shoot modification", []operationTest{
					t(shootA, reconcileNotAllowed, specModified),
					t(seedA, reconcileAllowed),
					t(shootA, reconcileNotAllowed, specModified),
					d(seedA),
					t(shootA, reconcileAllowed, specModified),
				}),
			)
		})
	})
})

// Helper function for tracking scheduling decisions.

type loggerType struct {
	messages []string
}

func (l *loggerType) String() string {
	s := ""
	for _, msg := range l.messages {
		s = fmt.Sprintf("%s%s\n", s, msg)
	}
	return s
}

func (l *loggerType) Add(msgFmt string, args ...interface{}) {
	l.messages = append(l.messages, fmt.Sprintf(msgFmt, args...))
}

// Implementation of the ID interface for test purposes.

type id struct {
	name string
}

func (i id) String() string {
	return i.name
}

func (i id) GetID() ID {
	return i
}

func newID(name string) id {
	return id{name}
}

// Implementation of the Element interface for test purposes.

type element struct {
	name   *id
	parent *id
}

func (e element) String() string {
	return e.name.String()
}

func (e *element) GetID() ID {
	return *e.name
}
func (e *element) GetParentID() ID {
	return *e.parent
}

func newElement(id *id, parent *id) *element {
	return &element{id, parent}
}

// Element structure for tests

type operationTest interface {
	Execute(scheduler Interface, logger *loggerType) interface{}
	Expected() interface{}
}

// testAndActivate structure and execution

type testAndActivateTest struct {
	element         *element
	modified        bool
	inhibitChildren bool

	expectation bool
}

func (t *testAndActivateTest) Execute(scheduler Interface, logger *loggerType) interface{} {
	mayReconcile, reason := scheduler.TestAndActivate(t.element, t.modified, t.inhibitChildren)
	logger.Add("%s => %s", t, reason)
	return mayReconcile
}

func (t *testAndActivateTest) Expected() interface{} {
	return t.expectation
}

func (t *testAndActivateTest) String() string {
	return fmt.Sprintf("testAndActivate: %s, modified=%t, inhibitChildren=%t", t.element, t.modified, t.inhibitChildren)
}

func t(element *element, expectation bool, flags ...int) operationTest {
	et := &testAndActivateTest{
		element:         element,
		modified:        false,
		inhibitChildren: false,

		expectation: expectation,
	}

	for _, flag := range flags {
		switch flag {
		case isReserved:
			et.inhibitChildren = true
		case specModified:
			et.modified = true
		default:
			panic(fmt.Sprintf("invalid flag %d", flag))
		}
	}

	return et
}

// doneTest structure and execution

type doneTest struct {
	element *element

	expectation bool
}

func (d *doneTest) Execute(scheduler Interface, logger *loggerType) interface{} {
	done := scheduler.Done(d.element.GetID())
	logger.Add("%s => %t", d, done)
	return done
}

func (d *doneTest) Expected() interface{} {
	return d.expectation
}

func (d *doneTest) String() string {
	return fmt.Sprintf("done: %s", d.element)
}

func d(element *element, expectation ...bool) operationTest {
	d := &doneTest{
		element:     element,
		expectation: true,
	}

	if len(expectation) > 0 {
		d.expectation = expectation[0]
	}

	return d
}
