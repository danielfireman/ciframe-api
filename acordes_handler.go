package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func AcordesHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	b, err := json.Marshal(acordes)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, string(b))
	w.WriteHeader(http.StatusOK)
}
