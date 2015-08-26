# Peeler

## Installation

To install Peeler:

    go get github.com/solher/peeler

## Usage

main.go

```
package main

import (
	"log"
	"net/http"

	"github.com/dimfeld/httptreemux"
	"github.com/solher/peeler"
)

func main() {
	peeler.Default.Populate()
	router := httptreemux.New()

	controller := &Controller{}
	peeler.Default.GetOne(controller)

	router.POST("/", controller.Handler)

	log.Fatal(http.ListenAndServe(":3000", router))
}
```

controller.go

```
package main

import (
	"net/http"

	"github.com/solher/peeler"
)

func init() {
	peeler.Default.Register(NewController)
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

model.go

```
package main

import "github.com/solher/peeler"

func init() {
	peeler.Default.Register(NewModel)
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

store.go

```
package main

import "github.com/solher/peeler"

func init() {
	peeler.Default.Register(NewStore)
}

type Store struct{}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) DBAction() {}

```

## Features

* Dependency graph builder based on constructors (see [facebookgo/inject](https://github.com/facebookgo/inject) if you don't use constructors)
* Circular dependencies resolver

## License

MIT
