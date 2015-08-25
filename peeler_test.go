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
		depA    *dep4
		content string
	}

	dep4 struct {
		content string
	}
)

func newDep1(depA *dep2, depB *dep3) *dep1 {
	depB.content = "foobar"
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
	a.IsType(newDep1, peeler.deps[0])
	a.IsType(newDep2, peeler.deps[1])

	peeler.Register(newDep3, newDep4)
	r.NotPanics(func() { _ = peeler.deps[2]; _ = peeler.deps[3] })
	a.IsType(newDep3, peeler.deps[2])
	a.IsType(newDep4, peeler.deps[3])

	peeler.Register(newDep4)
	r.NotPanics(func() { _ = peeler.deps[4] })
	a.IsType(newDep4, peeler.deps[4])
}

func TestSafePopulate(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	peeler := NewPeeler()

	peeler.deps = append(peeler.deps, newDep3)

	err := peeler.SafePopulate()
	if a.Error(err) {
		a.Contains(err.Error(), "*main.dep4", "the error message should indicate that the *main.dep4 dep is missing")
	}

	peeler.deps = append(peeler.deps, newDep4)

	err = peeler.SafePopulate()
	if a.NoError(err) {
		_, _, depC, depD := getDeps(peeler)

		r.NotNil(depC)
		r.NotNil(depD)
		r.IsType(&dep3{}, depC)
		r.IsType(&dep4{}, depD)

		a.NotNil(depC.depA)
		a.Equal(depC.depA.content, "foobar")
	}

	peeler.deps = append(peeler.deps, newDep1, newDep2)
	err = peeler.SafePopulate()
	a.Error(err, "we expect the safe populate to throw an error when dealing with a circular dependency")

	peeler.deps = append(peeler.deps, newDep1, newDep1)
	err = peeler.SafePopulate()
	if a.Error(err, "we expect the safe populate to throw an error when multiple constructors return the same type") {
		a.Contains(err.Error(), "conflict", "the error message should indicate an injection conflict")
	}
}

func TestPopulate(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	peeler := NewPeeler()

	peeler.deps = append(peeler.deps, newDep3)

	err := peeler.Populate()
	if a.Error(err) {
		a.Contains(err.Error(), "*main.dep4", "the error message should indicate that the *main.dep4 dep is missing")
	}

	peeler.deps = append(peeler.deps, newDep4)

	err = peeler.Populate()
	if a.NoError(err) {
		_, _, depC, depD := getDeps(peeler)

		r.NotNil(depC)
		r.NotNil(depD)
		r.IsType(&dep3{}, depC)
		r.IsType(&dep4{}, depD)

		a.NotNil(depC.depA)
		a.Equal(depC.depA.content, "foobar")
	}

	peeler.deps = append(peeler.deps, newDep1, newDep2)

	err = peeler.Populate()
	if a.NoError(err) {
		depA, depB, depC, depD := getDeps(peeler)

		r.NotNil(depA)
		r.NotNil(depB)
		r.NotNil(depC)
		r.NotNil(depD)
		r.IsType(&dep1{}, depA)
		r.IsType(&dep2{}, depB)
		r.IsType(&dep3{}, depC)
		r.IsType(&dep4{}, depD)

		a.NotNil(depA.depB)
		a.Equal(depA.depB.depA.content, "foobar")
		a.Equal(depA.depB.content, "foobar")
		a.EqualValues(depA.depA, depB)
		a.EqualValues(depB.depA, depA)
	}

	peeler.deps = append(peeler.deps, newDep1, newDep1)
	err = peeler.Populate()
	a.Error(err, "we expect the populate method to throw an error when multiple constructors return the same type")
}

func TestGet(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	peeler := NewPeeler()
	buildDeps(peeler)

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
		r.NotNil(deps.FoundDepD)
		r.NotNil(deps.FoundDepA)
		a.EqualValues(deps.FoundDepD.depB.depA, deps.FoundDepA)
	}
}

func TestGetOne(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	peeler := NewPeeler()
	buildDeps(peeler)

	var uninitDep *dep1
	integer := 2

	r.NotPanics(func() { _ = peeler.GetOne(nil) })
	r.NotPanics(func() { _ = peeler.GetOne(integer) })
	r.NotPanics(func() { _ = peeler.GetOne(&integer) })
	r.NotPanics(func() { _ = peeler.GetOne(dep4{}) })
	r.NotPanics(func() { _ = peeler.GetOne(uninitDep) })

	foundDepA := &dep4{}

	err := peeler.GetOne(foundDepA)
	if a.NoError(err) {
		r.NotNil(foundDepA)
		a.Equal(foundDepA.content, "foobar")
	}
}

func buildDeps(peeler *Peeler) {
	depA := newDep4()
	depB := newDep3(depA)
	depC := newDep2(nil, depB)
	depD := newDep1(depC, depB)
	depC.depA = depD
	peeler.deps = append(peeler.deps, depA, depB, depC, depD)
}

func getDeps(peeler *Peeler) (depA *dep1, depB *dep2, depC *dep3, depD *dep4) {
	for i := 0; i < len(peeler.deps); i++ {
		switch dep := peeler.deps[i].(type) {
		case *dep1:
			depA = dep
		case *dep2:
			depB = dep
		case *dep3:
			depC = dep
		case *dep4:
			depD = dep
		}
	}

	return
}
