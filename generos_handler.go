package main

import (
	"net/http"
	"encoding/json"
	"fmt"
	"sort"

	sets "github.com/deckarep/golang-set"
	"github.com/julienschmidt/httprouter"
	"github.com/newrelic/go-agent"
)

type Generos struct {
	app newrelic.Application
	jsonData string
}

func NewGeneros(app newrelic.Application, generosSet sets.Set) (*Generos, error) {
	var generos []string
	for _, i := range generosSet.ToSlice() {
		generos = append(generos, i.(string))
	}
	sort.Sort(sort.StringSlice(generos))
	b, err := json.Marshal(generos)
	if err != nil {
		return nil, err
	}
	return &Generos{app, string(b)}, err
}

func (g *Generos) GetHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		txn := g.app.StartTransaction("generos", w, r)
		defer txn.End()
		w.Header().Add("Access-Control-Allow-Origin", "*")
		fmt.Fprintf(w, g.jsonData)
	}
}
