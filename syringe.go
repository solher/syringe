// Package syringe provide tools to build dependency graphs using constructors.
package syringe

import (
	"errors"
	"reflect"
)

// Default provide a quick access to an instanciated injector.
var Default = New()

// Syringe is a dependency injector.
// The deps field contains the dependencies and constructors to inject.
type Syringe struct {
	deps []interface{}
}

// New returns a new Syringe object.
func New() *Syringe {
	return &Syringe{}
}

// Register takes one, multiple, or a slice of dependencies and register them
// into the injector.
func (s *Syringe) Register(dependencies ...interface{}) {
	for _, d := range dependencies {
		switch d.(type) {
		case []interface{}:
			s.deps = append(s.deps, d.([]interface{})...)
		case interface{}:
			s.deps = append(s.deps, d)
		}
	}
}

// GetOne injects an empty pointer passed as argument with a dependency corresponding
// to its type.
func (s *Syringe) GetOne(obj interface{}) error {
	if obj == nil {
		return errors.New("invalid param: is nil")
	}

	objPtr := reflect.ValueOf(obj)

	if objPtr.Type().Kind() != reflect.Ptr {
		return errors.New("invalid param: is not a pointer to a dep")
	}

	objValue := objPtr.Elem()

	if objValue.Kind() == reflect.Invalid {
		return errors.New("invalid param: the dep pointer must be initialized")
	}

	for _, dep := range s.deps {
		depPtr := reflect.ValueOf(dep)

		if objPtr.Type() == depPtr.Type() && objValue.CanSet() {
			objValue.Set(depPtr.Elem())
			return nil
		}
	}

	return errors.New("dep not found")
}

// Get injects the fields of an indirected struct passed as argument with
// dependencies corresponding to their type.
func (s *Syringe) Get(depStruct interface{}) error {
	if depStruct == nil {
		return errors.New("invalid param: is nil")
	}

	ptrStruct := reflect.ValueOf(depStruct)

	if ptrStruct.Type().Kind() != reflect.Ptr || ptrStruct.Elem().Kind() != reflect.Struct {
		return errors.New("invalid param: is not a pointer to a struct to inject")
	}

	structValue := ptrStruct.Elem()
	numField := structValue.NumField()

	for i := 0; i < numField; i++ {
		fieldPtr := structValue.Field(i)

		for _, dep := range s.deps {
			depPtr := reflect.ValueOf(dep)

			if fieldPtr.Type() == depPtr.Type() && fieldPtr.CanSet() {
				fieldPtr.Set(depPtr)
				break
			}
		}
	}

	return nil
}

type injectionParams struct {
	missingDeps              []reflect.Type
	partialContructors, deps []reflect.Value
}

// Inject builds the dependency graph.
// It is capable to resolve circular dependencies using stub injection.
//
// Example:
//   dep1 struct {
// 	  dep *dep2
//   }
//
//   dep2 struct {
// 	  dep *dep1
//   }
//
//   func newDep1(dep *dep2) *dep1 {
// 	  return &dep1{dep: dep}
//   }
//
//   func newDep2(dep *dep1) *dep2 {
//   	return &dep2{dep: dep}
//   }
//
// In this example, the injector will call the two constructors with stub
// params and then will replace their value with the instanciated dependencies.
//
// That method can be seen as not safe because it would override eventual
// modifications done in the constructors.
//
// That being said, it feels like the standard way of doing things as it is a very
// specific problem that can't be fixed by a "by hand" injection. (True ?)
func (s *Syringe) Inject() error {
	return s.inject(false)
}

// SafeInject builds the dependency graph as a human would do by hand.
// Therefore, it is not capable to resolve circular dependencies.
func (s *Syringe) SafeInject() error {
	return s.inject(true)
}

func (s *Syringe) inject(safeMode bool) error {
	// first, injection conflicts are checked
	if !checkInjectionConflicts(s.deps) {
		return errors.New("conflict detected: multiple constructors returning the same dependency type")
	}

	params := &injectionParams{}

	// then the params are injectd from the provided dependencies
	for _, dep := range s.deps {
		value := reflect.ValueOf(dep)

		switch value.Kind() {
		case reflect.Func:
			params.partialContructors = append(params.partialContructors, value)
		case reflect.Ptr:
			params.deps = append(params.deps, value)
		}
	}

	// the injection is run
	results, err := s.simpleInject(params)
	if err != nil {
		if safeMode {
			return err
		}

		results, err = s.stubInject(results)
		if err != nil {
			return err
		}
	}

	// the results are saved if no error occurred
	s.deps = []interface{}{}
	for _, dep := range results.deps {
		s.deps = append(s.deps, dep.Interface())
	}

	return nil
}

func (s *Syringe) simpleInject(params *injectionParams) (*injectionParams, error) {
	// "missingDeps" is the list of every dependencies missing to call the constructors in "partialContructors"
	missingDeps := params.missingDeps
	// "partialContructors" is the list of the constructors which can't be called with the params in "deps"
	partialContructors := params.partialContructors
	// "deps" is the list of every dependencies available for injection
	deps := params.deps

	lastDepsLen := -1

	// try to inject while new dependencies are instanciated
	for len(deps) > lastDepsLen {
		lastDepsLen = len(deps)

		for _, cons := range partialContructors {
			params := []reflect.Value{}
			numIn := cons.Type().NumIn()

			for i := 0; i < numIn; i++ {
				param := cons.Type().In(i)
				if foundDep, err := find(deps, param); err == nil {
					params = append(params, foundDep)
				} else {
					missingDeps = append(missingDeps, param)
				}
			}

			if len(params) == numIn {
				returnValues := cons.Call(params)
				for _, v := range returnValues {
					deps = append(deps, v)
					missingDeps, _ = removeType(missingDeps, v.Type())
				}

				partialContructors, _ = removeValue(partialContructors, cons)
			}
		}
	}

	results := &injectionParams{
		missingDeps:        missingDeps,
		partialContructors: partialContructors,
		deps:               deps,
	}

	// generate an error if there are missing dependencies
	err := checkMissingDependencies(missingDeps)
	if err != nil {
		return results, err
	}

	return results, nil
}

func (s *Syringe) stubInject(params *injectionParams) (*injectionParams, error) {
	// "stubDeps" is the list of every stub empty dependencies instanciated during the injection
	stubDeps := []reflect.Value{}
	// "missingDeps" is the list of every dependencies missing to replace stub ones
	missingDeps := params.missingDeps
	// "partialContructors" is the list of the constructors which can't be called with the params in "deps"
	partialContructors := params.partialContructors
	// "deps" is the list of every dependencies available for injection
	deps := params.deps

	// in the first step, the partial constructors are called with stubs when deps are missing
	for _, c := range partialContructors {
		switch c.Kind() {
		case reflect.Func:
			params := []reflect.Value{}

			// create stubs for each missing dependency
			for i := 0; i < c.Type().NumIn(); i++ {
				param := c.Type().In(i)

				if foundDep, err := find(deps, param); err == nil {
					params = append(params, foundDep)
				} else {
					if param.Kind() == reflect.Ptr {
						stubDep := reflect.New(param.Elem())
						params = append(params, stubDep)
						stubDeps = append(stubDeps, stubDep)
					}
				}
			}

			returnValues := c.Call(params)
			deps = append(deps, returnValues...)
		}
	}

	// in the second step, the stub dependencies are replaced by "real" ones
	missingDeps = []reflect.Type{}

	for _, stubDep := range stubDeps {
		found := false

		for _, dep := range deps {
			if stubDep.Type() == dep.Type() {
				stubDep.Elem().Set(dep.Elem())
				found = true
			}
		}

		if !found {
			missingDeps = append(missingDeps, stubDep.Type())
		}
	}

	results := &injectionParams{
		missingDeps: missingDeps,
		deps:        deps,
	}

	// generate an error if there are missing dependencies
	err := checkMissingDependencies(missingDeps)
	if err != nil {
		return results, err
	}

	return results, nil
}

func removeValue(a []reflect.Value, value reflect.Value) ([]reflect.Value, error) {
	for i, v := range a {
		if v == value {
			return append(a[:i], a[i+1:]...), nil
		}
	}

	return nil, errors.New("value not found")
}

func removeType(a []reflect.Type, value reflect.Type) ([]reflect.Type, error) {
	for i, t := range a {
		if t == value {
			return append(a[:i], a[i+1:]...), nil
		}
	}

	return nil, errors.New("value not found")
}

func find(a []reflect.Value, depType reflect.Type) (reflect.Value, error) {
	for _, v := range a {
		if v.Type() == depType {
			return v, nil
		}
	}

	return reflect.Value{}, errors.New("value not found")
}

func checkInjectionConflicts(deps []interface{}) bool {
	t := []reflect.Type{}

	for _, d := range deps {
		dv := reflect.ValueOf(d)
		if dv.Kind() == reflect.Func {
			for i := 0; i < dv.Type().NumOut(); i++ {
				param := dv.Type().Out(i)
				for _, v1 := range t {
					if v1 == param {
						return false
					}
				}

				t = append(t, param)
			}
		}
	}

	return true
}

func checkMissingDependencies(missingDeps []reflect.Type) error {
	if len(missingDeps) > 0 {
		errMsg := "injection failed. Missing dependencies: "
		for _, d := range missingDeps {
			errMsg = errMsg + d.String() + ", "
		}
		return errors.New(errMsg[:len(errMsg)-2])
	}

	return nil
}
