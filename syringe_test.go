package syringe

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

// TestRegister runs tests on the syringe Register method.
func TestRegister(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	syringe := New()

	deps := []interface{}{
		newDep1,
		newDep2,
	}

	syringe.Register(deps)
	r.NotPanics(func() { _ = syringe.deps[0]; _ = syringe.deps[1] })
	a.IsType(newDep1, syringe.deps[0])
	a.IsType(newDep2, syringe.deps[1])

	syringe.Register(newDep3, newDep4)
	r.NotPanics(func() { _ = syringe.deps[2]; _ = syringe.deps[3] })
	a.IsType(newDep3, syringe.deps[2])
	a.IsType(newDep4, syringe.deps[3])

	syringe.Register(newDep4)
	r.NotPanics(func() { _ = syringe.deps[4] })
	a.IsType(newDep4, syringe.deps[4])

	syringe.Register(newDep4())
	r.NotPanics(func() { _ = syringe.deps[5] })
	a.IsType(&dep4{}, syringe.deps[5])
}

// TestSafeInject runs tests on the syringe SafeInject method.
func TestSafeInject(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	syringe := New()

	syringe.deps = append(syringe.deps, newDep3)

	err := syringe.SafeInject()
	if a.Error(err) {
		a.Contains(err.Error(), "*syringe.dep4", "the error message should indicate that the *main.dep4 dep is missing")
	}

	syringe.deps = append(syringe.deps, newDep4)

	err = syringe.SafeInject()
	if a.NoError(err) {
		_, _, depC, depD := getDeps(syringe)

		r.NotNil(depC)
		r.NotNil(depD)
		r.IsType(&dep3{}, depC)
		r.IsType(&dep4{}, depD)

		a.NotNil(depC.depA)
		a.Equal(depC.depA.content, "foobar")
	}

	syringe = New()
	syringe.deps = append(syringe.deps, newDep3, newDep4()) // Should work the same if the dep is instanciated at the registering
	err = syringe.SafeInject()
	if a.NoError(err) {
		_, _, depC, depD := getDeps(syringe)

		r.NotNil(depC)
		r.NotNil(depD)
		r.IsType(&dep3{}, depC)
		r.IsType(&dep4{}, depD)

		a.NotNil(depC.depA)
		a.Equal(depC.depA.content, "foobar")
	}

	syringe.deps = append(syringe.deps, newDep1, newDep2)
	err = syringe.SafeInject()
	a.Error(err, "we expect the safe inject to throw an error when dealing with a circular dependency")

	syringe.deps = append(syringe.deps, newDep1, newDep1)
	err = syringe.SafeInject()
	if a.Error(err, "we expect the safe inject to throw an error when multiple constructors return the same type") {
		a.Contains(err.Error(), "conflict", "the error message should indicate an injection conflict")
	}
}

// TestInject runs tests on the syringe Inject method.
func TestInject(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	syringe := New()

	syringe.deps = append(syringe.deps, newDep3)

	err := syringe.Inject()
	if a.Error(err) {
		a.Contains(err.Error(), "*syringe.dep4", "the error message should indicate that the *main.dep4 dep is missing")
	}

	syringe.deps = append(syringe.deps, newDep4)

	err = syringe.Inject()
	if a.NoError(err) {
		_, _, depC, depD := getDeps(syringe)

		r.NotNil(depC)
		r.NotNil(depD)
		r.IsType(&dep3{}, depC)
		r.IsType(&dep4{}, depD)

		a.NotNil(depC.depA)
		a.Equal(depC.depA.content, "foobar")
	}

	syringe = New()
	syringe.deps = append(syringe.deps, newDep3, newDep4()) // Should work the same if the dep is instanciated at the registering
	err = syringe.SafeInject()
	if a.NoError(err) {
		_, _, depC, depD := getDeps(syringe)

		r.NotNil(depC)
		r.NotNil(depD)
		r.IsType(&dep3{}, depC)
		r.IsType(&dep4{}, depD)

		a.NotNil(depC.depA)
		a.Equal(depC.depA.content, "foobar")
	}

	syringe.deps = append(syringe.deps, newDep1, newDep2)

	err = syringe.Inject()
	if a.NoError(err) {
		depA, depB, depC, depD := getDeps(syringe)

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

	syringe.deps = append(syringe.deps, newDep1, newDep1)
	err = syringe.Inject()
	a.Error(err, "we expect the inject method to throw an error when multiple constructors return the same type")
}

// TestGet runs tests on the syringe Get method.
func TestGet(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	syringe := New()
	buildDeps(syringe)

	type privateDep struct {
		depA *dep4
	}
	integer := 2

	r.NotPanics(func() { _ = syringe.Get(nil) })
	r.NotPanics(func() { _ = syringe.Get(integer) })
	r.NotPanics(func() { _ = syringe.Get(&integer) })
	r.NotPanics(func() { _ = syringe.Get(privateDep{}) })
	r.NotPanics(func() { _ = syringe.Get(&privateDep{}) })

	type dependencies struct {
		FoundDepD *dep1
		FoundDepA *dep4
	}

	deps := &dependencies{}

	err := syringe.Get(deps)
	if a.NoError(err) {
		r.NotNil(deps)
		r.NotNil(deps.FoundDepD)
		r.NotNil(deps.FoundDepA)
		a.EqualValues(deps.FoundDepD.depB.depA, deps.FoundDepA)
		deps.FoundDepA.content = "barfoo"
	}

	deps = &dependencies{}

	err = syringe.Get(deps)
	if a.NoError(err) {
		a.EqualValues(deps.FoundDepA.content, "barfoo")
	}
}

func buildDeps(syringe *Syringe) {
	depA := newDep4()
	depB := newDep3(depA)
	depC := newDep2(nil, depB)
	depD := newDep1(depC, depB)
	depC.depA = depD
	syringe.deps = append(syringe.deps, depA, depB, depC, depD)
}

func getDeps(syringe *Syringe) (depA *dep1, depB *dep2, depC *dep3, depD *dep4) {
	for i := 0; i < len(syringe.deps); i++ {
		switch dep := syringe.deps[i].(type) {
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
