# Syringe [![Build Status](https://travis-ci.org/solher/syringe.svg)](https://travis-ci.org/solher/syringe) [![Coverage Status](https://coveralls.io/repos/solher/syringe/badge.svg?branch=master&service=github)](https://coveralls.io/github/solher/syringe?branch=master)

## Installation

To install Syringe:

    go get github.com/solher/syringe

## Usage

**main.go**

```go
package main

import (
	"log"
	"net/http"

	"github.com/dimfeld/httptreemux"
	"github.com/solher/syringe"
)

func main() {
	syringe.Default.Inject()
	router := httptreemux.New()

	controller := &Controller{}
	syringe.Default.GetOne(controller)

	router.POST("/", controller.Handler)

	log.Fatal(http.ListenAndServe(":3000", router))
}
```

**controller.go**

```go
package main

import (
	"net/http"

	"github.com/solher/syringe"
)

func init() {
	syringe.Default.Register(NewController)
}

type Controller struct {
	m *Model
}

func NewController(m *Model) *Controller {
	return &Controller{m: m}
}

func (c *Controller) Handler(w http.ResponseWriter, r *http.Request, params map[string]string) {
	c.m.Action()
	w.WriteHeader(http.StatusOK)
}
```

**model.go**

```go
package main

import "github.com/solher/syringe"

func init() {
	syringe.Default.Register(NewModel)
}

type Model struct {
	s *Store
}

func NewModel(s *Store) *Model {
	return &Model{s: s}
}

func (m *Model) Action() {
	m.s.DBAction()
}

```

**store.go**

```go
package main

import "github.com/solher/syringe"

func init() {
	syringe.Default.Register(NewStore)
}

type Store struct{}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) DBAction() {}

```

## Features

- Dependency graph builder using constructors (see [facebookgo/inject](https://github.com/facebookgo/inject) if you don't use constructors)
- Circular dependencies resolver

## License

MIT
