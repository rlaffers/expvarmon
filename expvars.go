package main

import (
	"errors"
	"io"
	"net/http"

	"github.com/antonholmquist/jason"
)

// ExpvarsUrl is the default url for fetching expvar info.
const ExpvarsURL = "/debug/vars"

// Expvar represents fetched expvar variable.
type Expvar struct {
	*jason.Object
}

// FetchExpvar fetches expvar by http for the given addr (host:port)
func FetchExpvar(addr string) (*Expvar, error) {
	e := &Expvar{&jason.Object{}}
	resp, err := http.Get(addr)
	if err != nil {
		return e, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return e, errors.New("Vars not found. Did you import expvars?")
	}

	e, err = ParseExpvar(resp.Body)
	if err != nil {
		return e, err
	}
	return e, nil
}

// ParseExpvar parses expvar data from reader.
func ParseExpvar(r io.Reader) (*Expvar, error) {
	object, err := jason.NewObjectFromReader(r)
	return &Expvar{object}, err
}