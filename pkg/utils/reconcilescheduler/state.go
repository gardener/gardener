package reconcilescheduler

import (
	"io/ioutil"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ID describes the interface for an unique identifier of an object.
type ID interface {
	// String returns the string representation of the id.
	String() string
}

// Element describes the interface for getting the id an its parent.
type Element interface {
	// GetID returns the ID of the element.
	GetID() ID
	// GetParent returns the ID of the element's parent.
	GetParentID() ID
}

// Interface describes the interface for interacting with the reconcile scheduler.
// It holds an internal state of elements and answers "may I schedule this element now" questions.
type Interface interface {
	// TestAndActivate decides whether the given Element is allowed to be activated (with respected to whether it has
	// been modified and/or inhibitChildren).
	TestAndActivate(Element, bool, bool) (bool, *Reason)
	// Done marks the given ID as "done" (meaning that a reconciliation has been executed successful).
	Done(ID) bool
	// Delete deletes the given ID from the internal state.
	Delete(ID)

	// MarkStatic marks an identifier as "static" (meaning that it has no parent represented by another ID).
	MarkStatic(ID)
	// UnmarkStatic unmarks an identifer as "static".
	UnmarkStatic(ID)

	// Reset resets the internal state of the scheduler.
	Reset()
}

type entry struct {
	id     ID
	parent ID

	active          bool
	inhibitChildren bool

	lastScheduleTime time.Time
}

type state struct {
	lock   sync.Mutex
	logger logrus.FieldLogger

	staticParents map[ID]struct{}
	entries       map[ID]*entry
}

var (
	_        Interface = &state{}
	zeroTime time.Time
)

// New returns a new reconcile scheduler interface.
func New(logger logrus.FieldLogger) Interface {
	s := &state{
		staticParents: map[ID]struct{}{},
		entries:       map[ID]*entry{},
	}

	if logger == nil {
		l := logrus.New()
		l.Out = ioutil.Discard
		s.logger = l
	} else {
		s.logger = logger
	}

	return s
}

// TestAndActivate decides whether the given Element is allowed to be activated (with respected to whether it has
// been modified and/or it wants to inhibit children although they might have a higher priority).
func (s *state) TestAndActivate(element Element, modified, inhibitChildren bool) (bool, *Reason) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.logger.Debugf("TEST %q (modified=%t, inhibitChildren=%t)", element.GetID(), modified, inhibitChildren)

	entry := s.getEntry(element)

	// If the element is already active then it cannot be activated again.
	if entry.active {
		return false, NewReason(CodeAlreadyActive, "element has already been marked as 'active'")
	}

	var (
		elementID   = element.GetID()
		parentID    = element.GetParentID()
		parentEntry = s.entries[parentID]
	)

	// We want to ensure that parents are schedule first. Hence, when the object has not been
	// modified then we check whether the parent has already been reconciled.
	if !s.considerModification(entry, modified) {
		var (
			checkedIDs = sets.NewString(elementID.String())
			currentID  = parentID
		)

		// Identify the first parent of the entry to later check whether we are in a cycle or not,
		// and whether the parent needs to be scheduled first.
		for p := s.entries[currentID]; p != nil && !checkedIDs.Has(currentID.String()); {
			checkedIDs.Insert(currentID.String())
			currentID = p.parent
			p = s.entries[currentID]
		}

		// If a parent is missing then we cannot decide yet whether the given element may be scheduled.
		if !s.parentKnown(currentID) {
			return false, NewReason(CodeParentUnknown, "required parent (%q) not yet known", currentID)
		}

		// If we have not found a cycle then we check whether the first parent has been scheduled already.
		if currentID != elementID {
			// Check whether first parent has already been scheduled previously. If not then we deny as we
			// want that parents are scheduled first.
			if !alreadyScheduled(parentEntry) {
				return false, NewReason(CodeParentNotReconciled, "parent (%q) not yet scheduled", parentEntry.id)
			}
		} else {
			s.logger.Debugf("cycle found for %s", elementID)
		}
	}

	// Check that neither the parent nor any of the children is currently active. If a child has been found
	// to be active it is possible to prohibit further child schedules in order to prevent that a parent
	// starves (based on the set <inhibitChildren> argument).
	for _, e := range s.entries {
		if isMyChild(elementID, e) && e.active {
			entry.inhibitChildren = inhibitChildren
			return false, NewReason(CodeChildActive, "child (%s) active", e.id)
		}
	}

	if parentEntry != nil {
		if parentEntry.active {
			return false, NewReason(CodeParentActive, "parent (%q) active", parentEntry.id)
		}
		if parentEntry.inhibitChildren {
			return false, NewReason(CodeParentPending, "parent (%q) requested to be scheduled before any other children", parentEntry.id)
		}
	}

	// All checks have been passed - the element can be activated. Its entry is no longer marked for
	// inhibiting children (if it actually was marked at all) as it is now activated and allowed to
	// be scheduled.
	entry.lastScheduleTime = time.Now()
	entry.active = true
	entry.inhibitChildren = false

	return true, NewReason(CodeActivated, "activated %q", element.GetID())
}

// Done marks the given id as "done" (meaning that a reconciliation has been executed successful).
// It returns false if the no entry with the given id is known or if the respective entry is not
// active. Otherwise, true is returned.
func (s *state) Done(id ID) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.logger.Debugf("DONE %q", id)

	entry := s.entries[id]
	if entry == nil || !entry.active {
		return false
	}

	entry.active = false
	return true
}

// Delete deletes the given id from the internal state.
func (s *state) Delete(id ID) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.logger.Debugf("DELETE %q", id)

	delete(s.entries, id)
}

// Reset resets the internal state of the scheduler.
func (s *state) Reset() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.logger.Debugf("RESET")

	s.entries = map[ID]*entry{}
}

// MarkStatic marks an identifier as "static" (meaning that it has no parent represented by another ID).
func (s *state) MarkStatic(id ID) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.logger.Debugf("DECLARE STATIC PARENT %q", id)

	s.staticParents[id] = struct{}{}
}

// UnmarkStatic unmarks an identifer as "static".
func (s *state) UnmarkStatic(id ID) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.logger.Debugf("DELETE STATIC PARENT %q", id)

	delete(s.staticParents, id)
}

// parentKnown checks whether the given parent is valid. It returns true if the parent
// was either found in the state map or if has been declared as "static". It returns
// false if the parent is not (yet) known.
func (s *state) parentKnown(parent ID) bool {
	_, isStatic := s.staticParents[parent]
	return s.entries[parent] != nil || isStatic
}

// considerModification checks whether the modification should be considered for the scheduling.
// It is always considered if the parent has already been reconciled. Otherwise, the modification
// is of no relevance for the scheduling because the parents have precedence at start-up time.
func (s *state) considerModification(entry *entry, modified bool) bool {
	return modified && s.parentKnown(entry.parent) && alreadyScheduled(s.entries[entry.parent])
}

// getEntry returns the entry that has been stored for the given element in the state map.
// It also updates the parent in case it has changed.
// If the given element is not yet known to the state map then it gets added and returned.
func (s *state) getEntry(element Element) *entry {
	id := element.GetID()

	// id has been found in the state map - refresh parent in case it has changed
	if entry := s.entries[id]; entry != nil {
		entry.parent = element.GetParentID()
		return entry
	}

	// id has not been found in the state map - add and return it
	entry := &entry{
		id:               id,
		parent:           element.GetParentID(),
		active:           false,
		inhibitChildren:  false,
		lastScheduleTime: zeroTime,
	}
	s.entries[id] = entry
	return entry
}

func alreadyScheduled(entry *entry) bool {
	return entry == nil || entry.lastScheduleTime != zeroTime
}

func isMyChild(me ID, entry *entry) bool {
	return entry.parent == me
}
