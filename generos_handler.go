package main

import (
	"net/http"

	"encoding/json"
	"fmt"
	"github.com/julienschmidt/httprouter"
)

func GenerosHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	b, err := json.Marshal(generos)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Add("Access-Control-Allow-Origin", "*")
	fmt.Fprintf(w, string(b))
}
