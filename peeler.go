package main

import (
	"errors"
	"reflect"
)

type Peeler struct {
	deps []interface{}
}

func NewPeeler() *Peeler {
	return &Peeler{}
}

func (p *Peeler) Register(dependencies ...interface{}) error {
	for _, d := range dependencies {
		switch d.(type) {
		case []interface{}:
			p.deps = append(p.deps, d.([]interface{})...)
		case interface{}:
			p.deps = append(p.deps, d)
		}
	}

	return nil
}

func (p *Peeler) GetOne(obj interface{}) error {
	if obj == nil {
		return errors.New("Invalid param: is nil")
	}

	objPtr := reflect.ValueOf(obj)

	if objPtr.Type().Kind() != reflect.Ptr {
		return errors.New("Invalid param: is not a pointer to a dep")
	}

	objValue := objPtr.Elem()

	if objValue.Kind() == reflect.Invalid {
		return errors.New("Invalid param: the dep pointer must be initialized")
	}

	for _, dep := range p.deps {
		depPtr := reflect.ValueOf(dep)

		if objPtr.Type() == depPtr.Type() && objValue.CanSet() {
			objValue.Set(depPtr.Elem())
			break
		}
	}

	return nil
}

func (p *Peeler) Get(depStruct interface{}) error {
	if depStruct == nil {
		return errors.New("Invalid param: is nil")
	}

	ptrStruct := reflect.ValueOf(depStruct)

	if ptrStruct.Type().Kind() != reflect.Ptr || ptrStruct.Elem().Kind() != reflect.Struct {
		return errors.New("Invalid param: is not a pointer to a struct to populate")
	}

	structValue := ptrStruct.Elem()
	numField := structValue.NumField()

	for i := 0; i < numField; i++ {
		fieldPtr := structValue.Field(i)

		for _, dep := range p.deps {
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

func (p *Peeler) Populate() error {
	return p.populate(false)
}

func (p *Peeler) SafePopulate() error {
	return p.populate(true)
}

func (p *Peeler) populate(safeMode bool) error {
	// first, we check if there will be injection conflicts
	if !checkInjectionConflicts(p.deps) {
		return errors.New("conflict detected: multiple constructors returning the same dependency type")
	}

	params := &injectionParams{}

	// first, the params are populated from the provided dependencies
	for _, dep := range p.deps {
		value := reflect.ValueOf(dep)

		switch value.Kind() {
		case reflect.Func:
			params.partialContructors = append(params.partialContructors, value)
		case reflect.Ptr:
			params.deps = append(params.deps, value)
		}
	}

	results, err := p.simplePopulate(params)
	if err != nil {
		if safeMode {
			return err
		}

		results, err = p.stubPopulate(results)
		if err != nil {
			return err
		}
	}

	p.deps = []interface{}{}
	for _, dep := range results.deps {
		p.deps = append(p.deps, dep.Interface())
	}

	return nil
}

func (p *Peeler) simplePopulate(params *injectionParams) (*injectionParams, error) {
	// "missingDeps" is the list of every dependencies missing to call the constructors in "partialContructors"
	missingDeps := params.missingDeps
	// "partialContructors" is the list of the constructors which can't be called with the params in "deps"
	partialContructors := params.partialContructors
	// "deps" is the list of every dependencies available for injection
	deps := params.deps

	lastDepsLen := -1

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

	err := checkMissingDependencies(missingDeps)
	if err != nil {
		return results, err
	}

	return results, nil
}

func (p *Peeler) stubPopulate(params *injectionParams) (*injectionParams, error) {
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

		case reflect.Ptr:
			deps = append(deps, c)
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
