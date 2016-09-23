package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"unicode"

	"github.com/julienschmidt/httprouter"

	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

type SearchResponse struct {
	IDArtista    string `json:"id_artista"`
	UniqueID     string `json:"id_unico_musica"`
	Genero       string `json:"genero"`
	ID           string `json:"id_musica"`
	Artista      string `json:"nome_artista"`
	Nome         string `json:"nome_musica"`
	URL          string `json:"url"`
	Popularidade int    `json:"popularidade"`
}

// Busca por músicas que possuem no título ou no nome do artista o argumento passado por key.
// params: key e generos (opcional). Caso generos não sejam definidos, a busca não irá filtrar por gênero.
// exemplo 1: /search?key=no dia em que eu saí de casa
// exemplo 2: /search?key=no dia em que eu saí de casa&generos=Rock,Samba '''
func SearchHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	generosABuscar := generosFromRequest(r)
	pagina, err := getPaginaFromRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	queryValues := r.URL.Query()
	keys := strings.Split(removerCombinantes(strings.ToLower(queryValues.Get("key"))), " ")
	var musicasRes []*Musica
	for _, m := range applyFiltro(generosABuscar) {
		text := fmt.Sprintf("%s %s", strings.ToLower(m.Artista), strings.ToLower(m.Nome))
		toCheck := make(map[string]struct{})
		for _, t := range strings.Split(removerCombinantes(text), " ") {
			toCheck[t] = struct{}{}
		}
		if all(keys, func(s string) bool {
			_, ok := toCheck[s]
			return ok
		}) {
			musicasRes = append(musicasRes, m)
		}
	}
	sort.Sort(PorPopularidade(musicasRes))

	var resultado []SearchResponse
	i, f := limitesDaPagina(len(musicasRes), pagina)
	for _, m := range musicasRes[i:f] {
		resultado = append(resultado, SearchResponse{
			IDArtista:    m.IDArtista,
			UniqueID:     m.UniqueID,
			Genero:       m.Genero,
			ID:           m.ID,
			Artista:      m.Artista,
			Nome:         m.Nome,
			URL:          m.URL,
			Popularidade: m.Popularidade,
		})

	}

	b, err := json.Marshal(resultado)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, string(b))
}

func applyFiltro(generos []string) []*Musica {
	var collection []*Musica
	for _, g := range generos {
		if _, ok := generosSet[g]; ok {
			collection = append(collection, generosMusicas[g]...)
		}
	}
	return collection
}

func all(vs []string, f func(string) bool) bool {
	for _, v := range vs {
		if !f(v) {
			return false
		}
	}
	return true
}

var diacriticosTransformer = transform.Chain(
	norm.NFD,
	transform.RemoveFunc(
		func(r rune) bool {
			return unicode.Is(unicode.Mn, r) // Mn: nonspacing marks
		}),
	norm.NFC)

func removerCombinantes(s string) string {
	r, _, _ := transform.String(diacriticosTransformer, s)
	return r
}
