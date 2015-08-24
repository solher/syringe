package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	dep1 struct {
		depA *dep2
		depB *dep3
	}

	dep2 struct {
		depA *dep1
		depB *dep3
	}

	dep3 struct {
		depA *dep4
	}

	dep4 struct {
		content string
	}
)

func newDep1(depA *dep2, depB *dep3) *dep1 {
	return &dep1{depA: depA, depB: depB}
}

func newDep2(depA *dep1, depB *dep3) *dep2 {
	return &dep2{depA: depA, depB: depB}
}

func newDep3(depA *dep4) *dep3 {
	return &dep3{depA: depA}
}

func newDep4() *dep4 {
	return &dep4{content: "foobar"}
}

func TestRegister(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	peeler := NewPeeler()

	deps := []interface{}{
		newDep1,
		newDep2,
	}

	peeler.Register(deps)
	r.NotPanics(func() { _ = peeler.deps[0]; _ = peeler.deps[1] })
	a.IsType(newDep1, peeler.deps[0], "Array register failed")
	a.IsType(newDep2, peeler.deps[1], "Array register failed")

	peeler.Register(newDep3, newDep4)
	r.NotPanics(func() { _ = peeler.deps[2]; _ = peeler.deps[3] })
	a.IsType(newDep3, peeler.deps[2], "Multiple register failed")
	a.IsType(newDep4, peeler.deps[3], "Multiple register failed")

	peeler.Register(newDep4)
	r.NotPanics(func() { _ = peeler.deps[4] })
	a.IsType(newDep4, peeler.deps[4], "Simple register failed")
}

func TestPopulate(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	peeler := NewPeeler()

	peeler.deps = append(peeler.deps, newDep3, newDep4)

	err := peeler.Populate()
	if a.NoError(err, "Simple injection failed") {
		errMsg := "Simple injection didn't return errors but "
		r.NotPanics(func() { _ = peeler.deps[0] })
		a.IsType(&dep3{}, peeler.deps[0], errMsg+"the internal dependencies are not refreshed")
		a.NotNil(peeler.deps[0].(*dep3).depA, errMsg+"dependencies are nil")
		a.Equal(peeler.deps[0].(*dep3).depA.content, "foobar", errMsg+"dependencies seems to be mocks")
	}

	peeler.deps = append(peeler.deps, newDep1, newDep2, newDep1, newDep1)

	err = peeler.Populate()
	if a.NoError(err, "Circular dependency injection failed") {
		errMsg := "Circular dependency injection didn't return errors but "
		r.NotPanics(func() { _ = peeler.deps[2]; _ = peeler.deps[3] })
		a.IsType(&dep1{}, peeler.deps[2], errMsg+"the internal dependencies are not refreshed")
		a.NotNil(peeler.deps[2].(*dep1).depB, errMsg+"dependencies are nil")
		a.Equal(peeler.deps[2].(*dep1).depB.depA.content, "foobar", errMsg+"dependencies seems to be mocks")
		a.EqualValues(peeler.deps[2].(*dep1).depA, peeler.deps[3], errMsg+"dependencies seems to be mocks")
		a.EqualValues(peeler.deps[3].(*dep2).depA, peeler.deps[2], errMsg+"dependencies seems to be mocks")
	}
}

func TestGet(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	peeler := NewPeeler()

	depA := newDep4()
	depB := newDep3(depA)
	depC := newDep2(nil, depB)
	depD := newDep1(depC, depB)
	depC.depA = depD
	peeler.deps = append(peeler.deps, depA, depB, depC, depD)

	type privateDep struct {
		depA *dep4
	}
	integer := 2

	r.NotPanics(func() { _ = peeler.Get(nil) })
	r.NotPanics(func() { _ = peeler.Get(integer) })
	r.NotPanics(func() { _ = peeler.Get(&integer) })
	r.NotPanics(func() { _ = peeler.Get(privateDep{}) })
	r.NotPanics(func() { _ = peeler.Get(&privateDep{}) })

	type dependencies struct {
		FoundDepD *dep1
		FoundDepA *dep4
	}

	deps := &dependencies{}

	err := peeler.Get(deps)
	if a.NoError(err) {
		r.NotNil(deps)
		a.EqualValues(deps.FoundDepD.depB.depA, deps.FoundDepA)
	}
}
