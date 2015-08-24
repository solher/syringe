package main

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/codegangsta/inject"
	"github.com/solher/zest/utils"
)

type Peeler struct {
	injector inject.Injector
	deps     []interface{}
}

func NewPeeler() *Peeler {
	return &Peeler{injector: inject.New()}
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
	type depStruct struct {
		obj interface{}
	}

	dep := &depStruct{obj: obj}

	if err := p.Get(dep); err != nil {
		return err
	}

	utils.Dump(reflect.ValueOf(dep.obj))

	return nil
}

func (p *Peeler) Get(depStruct interface{}) error {
	if depStruct == nil {
		return errors.New("Invalid param: is nil")
	}

	ptrValue := reflect.ValueOf(depStruct)

	if ptrValue.Type().Kind() != reflect.Ptr || ptrValue.Elem().Kind() != reflect.Struct {
		return errors.New("Invalid param: is not a pointer to a struct to populate")
	}

	structValue := reflect.Indirect(ptrValue)
	numField := structValue.NumField()

	for i := 0; i < numField; i++ {
		for _, dep := range p.deps {
			if structValue.Field(i).Type() == reflect.TypeOf(dep) && structValue.Field(i).CanSet() {
				structValue.Field(i).Set(reflect.ValueOf(dep))
			}
		}
	}

	depStruct = structValue.Interface()

	return nil
}

func (p *Peeler) Populate() error {
	// mockedDeps is the list of every mocked empty dependencies instanciated during the injection
	mockedDeps := []reflect.Value{}
	// deps is the list of every dependencies available for injection
	deps := []reflect.Value{}

	// in the first step, the above lists are populated from the provided dependencies
	for _, dep := range p.deps {
		value := reflect.ValueOf(dep)

		switch value.Kind() {
		case reflect.Func:
			mockedParams := []reflect.Value{}

			for i := 0; i < value.Type().NumIn(); i++ {
				param := value.Type().In(i)
				var mockedDep reflect.Value

				switch param.Kind() {
				case reflect.Struct:
					mockedDep = reflect.New(param)

				case reflect.Ptr:
					mockedDep = reflect.New(param.Elem())
				}

				mockedParams = append(mockedParams, mockedDep)
			}

			returnValues := value.Call(mockedParams)

			mockedDeps = append(mockedDeps, mockedParams...)
			deps = append(deps, returnValues...)

		case reflect.Struct:
			deps = append(deps, value)

		case reflect.Ptr:
			deps = append(deps, value)
		}
	}

	// in the second step, the mocked dependencies are replaced by "real" ones
	for _, mockedDep := range mockedDeps {
		found := false

		for _, dep := range deps {
			if mockedDep.Type() == dep.Type() {
				mockedDep.Elem().Set(dep.Elem())
				found = true
			}
		}

		if !found {
			return errors.New("Dep value not found")
		}
	}

	p.deps = []interface{}{}
	for _, dep := range deps {
		p.deps = append(p.deps, dep.Interface())
	}

	return nil
}

func (p *Peeler) OldPopulate() error {
	injector := p.injector
	failedDeps := dependencies{}
	values := []interface{}{}

	for _, obj := range p.deps {
		failedDeps = append(failedDeps, dependency{Object: obj})
	}

	lastLen := [2]int{len(failedDeps) + 1, len(failedDeps) + 2}

	for len(failedDeps) > 0 {
		if lastLen[0] <= len(failedDeps) && lastLen[1] <= lastLen[0] {
			return errors.New(fmt.Sprintf("Dependencies not found: %v", failedDeps.GetMissing()))
		}
		lastLen[1] = lastLen[0]
		lastLen[0] = len(failedDeps)

		for _, dep := range failedDeps {
			obj := dep.Object
			kind := reflect.ValueOf(obj).Kind()

			switch kind {
			case reflect.Func:
				vals, err := injector.Invoke(obj)

				if err != nil {
				} else {
					failedDeps.Remove(dep)

					for _, val := range vals {
						injector.Map(val.Interface())
						values = append(values, val.Interface())
					}
				}
			case reflect.Struct, reflect.Ptr:
				failedDeps.Remove(dep)
				injector.Map(obj)
				values = append(values, obj)
			}
		}
	}

	p.deps = values

	return nil
}

type (
	dependencies []dependency

	dependency struct {
		Object interface{}
	}
)

func (slc *dependencies) GetMissing() []reflect.Type {
	s := *slc
	missing := []reflect.Type{}

	for _, dep := range s {
		missing = append(missing, reflect.TypeOf(dep.Object))
	}

	return missing
}

func (slc *dependencies) Add(dep dependency) {
	s := *slc
	s = append(s, dep)
	*slc = s
}

func (slc *dependencies) Remove(dep dependency) {
	s := *slc

	for i, d := range s {
		if reflect.ValueOf(d.Object) == reflect.ValueOf(dep.Object) {
			s = append(s[:i], s[i+1:]...)
			*slc = s
			return
		}
	}
}
