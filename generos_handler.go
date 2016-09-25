package main

import (
	"net/http"

	"encoding/json"
	"fmt"

	"github.com/julienschmidt/httprouter"
	sets "github.com/deckarep/golang-set"
	"sort"
)

type Generos struct {
	jsonData string
}

func NewGeneros(generosSet sets.Set) (*Generos, error) {
	var generos []string
	for _, i := range generosSet.ToSlice() {
		generos = append(generos, i.(string))
	}
	sort.Sort(sort.StringSlice(generos))
	b, err := json.Marshal(generos)
	if err != nil {
		return nil, err
	}
	return &Generos{string(b)}, err
}

func (g *Generos) GetHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		fmt.Fprintf(w, g.jsonData)
	}
}
