package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// Retorna as músicas armazenadas no sistema (ordenados por popularidade).
// O serviço é paginado. Cada página tem tamanho 100, por default.
// params: pagina. Caso não seja definida a página, o valor default é 1.
// exemplo 1: /musica?pagina=2
// exemplo 2: /musica'''
func MusicasHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	pagina, err := getPaginaFromRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	i, f := limitesDaPagina(len(musicas), pagina)
	b, err := json.Marshal(musicas[i:f])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Add("Access-Control-Allow-Origin", "*")
	fmt.Fprintf(w, string(b))
}
