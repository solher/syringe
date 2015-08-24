package main

import (
	"testing"

	"github.com/solher/zest/utils"
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

	peeler.deps = append(peeler.deps, newDep3, newDep4)

	err := peeler.SafePopulate()
	if a.NoError(err) {
		r.NotPanics(func() { _ = peeler.deps[0]; _ = peeler.deps[1] })
		depA := peeler.deps[0]
		depB := peeler.deps[1]

		switch depA.(type) {
		case *dep3:
			a.IsType(&dep3{}, depA)
			a.NotNil(depA.(*dep3).depA)
			a.Equal(depA.(*dep3).depA.content, "foobar")
		case *dep4:
			a.Equal(depA.(*dep4).content, "foobar")
		}

		switch depB.(type) {
		case *dep3:
			a.IsType(&dep3{}, depB)
			a.NotNil(depB.(*dep3).depA)
			a.Equal(depB.(*dep3).depA.content, "foobar")
		case *dep4:
			a.Equal(depB.(*dep4).content, "foobar")
		}
	}

	peeler.deps = append(peeler.deps, newDep1, newDep2)
	err = peeler.SafePopulate()
	utils.Dump(err.Error())
	a.Error(err, "we expect the safe populate to throw an error when dealing with a circular dependency")

	peeler.deps = append(peeler.deps, newDep1, newDep1)
	err = peeler.SafePopulate()
	utils.Dump(err.Error())
	a.Error(err, "we expect the safe populate to throw an error when multiple constructors return the same type")
}

// func TestPopulate(t *testing.T) {
// 	a := assert.New(t)
// 	r := require.New(t)
// 	peeler := NewPeeler()
//
// 	peeler.deps = append(peeler.deps, newDep3, newDep4)
//
// 	err := peeler.Populate()
// 	if a.NoError(err) {
// 		r.NotPanics(func() { _ = peeler.deps[0] })
// 		a.IsType(&dep3{}, peeler.deps[0])
// 		a.NotNil(peeler.deps[0].(*dep3).depA)
// 		a.Equal(peeler.deps[0].(*dep3).depA.content, "foobar")
// 	}
//
// 	peeler.deps = append(peeler.deps, newDep1, newDep2, newDep1, newDep1)
//
// 	err = peeler.Populate()
// 	if a.NoError(err) {
// 		r.NotPanics(func() { _ = peeler.deps[2]; _ = peeler.deps[3] })
// 		a.IsType(&dep1{}, peeler.deps[2])
// 		a.NotNil(peeler.deps[2].(*dep1).depB)
// 		a.Equal(peeler.deps[2].(*dep1).depB.depA.content, "foobar")
// 		a.NotEqual(peeler.deps[2].(*dep1).depB.content, "foobar")
// 		a.EqualValues(peeler.deps[2].(*dep1).depA, peeler.deps[3])
// 		a.EqualValues(peeler.deps[3].(*dep2).depA, peeler.deps[2])
// 	}
// }

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
