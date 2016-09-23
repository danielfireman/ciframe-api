package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	sets "github.com/deckarep/golang-set"
	"github.com/julienschmidt/httprouter"
	"sort"
)

type SimilaresResponse struct {
	UniqueID     string        `json:"id_unico_musica"`
	IDArtista    string        `json:"id_artista"`
	ID           string        `json:"id_musica"`
	Artista      string        `json:"nome_artista"`
	Nome         string        `json:"nome_musica"`
	Popularidade int           `json:"popularidade"`
	Acordes      []interface{} `json:"acordes"`
	Genero       string        `json:"genero"`
	URL          string        `json:"url"`
	Diferenca    []interface{} `json:"diferenca"`
	Intersecao   []interface{} `json:"intersecao"`
}

// PorMaiorIntersecao implementa sort.Interface for []*Musica baseado no campo Popularidade
type ProMenorDiferenca []*SimilaresResponse

func (p ProMenorDiferenca) Len() int {
	return len(p)
}
func (p ProMenorDiferenca) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
func (p ProMenorDiferenca) Less(i, j int) bool {
	return len(p[i].Diferenca) < len(p[j].Diferenca)
}

var sequencias = map[string]int{
	"BmGDA":   0,
	"CGAmF":   1,
	"EmG":     2,
	"CA7DmG7": 3,
	"GmF":     4,
	"CC7FFm":  5,
}

func SimilaresHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	queryValues := r.URL.Query()
	pagina, err := getPaginaFromRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	generosABuscar := generosFromRequest(r)

	if queryValues.Get("sequencia") != "" {
		acordes := strings.Replace(queryValues.Get("sequencia"), ",", "", -1)
		var similares []*SimilaresResponse
		idSeq, ok := sequencias[acordes]
		if ok {
			strIdSeq := strconv.Itoa(idSeq)
			for _, m := range applyFiltro(generosABuscar) {
				for _, seq := range m.SeqFamosas {
					if seq == strIdSeq {
						similares = append(similares, &SimilaresResponse{
							UniqueID:     m.UniqueID,
							IDArtista:    m.IDArtista,
							ID:           m.ID,
							Artista:      m.Artista,
							Nome:         m.Nome,
							Popularidade: m.Popularidade,
							Acordes:      m.Acordes().ToSlice(),
							Genero:       m.Genero,
							URL:          m.URL,
						})
						break
					}
				}

			}
			sort.Sort(ProMenorDiferenca(similares))
			b, err := json.Marshal(similares)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Add("Access-Control-Allow-Origin", "*")
			fmt.Fprintf(w, string(b))
			return
		}
	}

	// tratamento do requist.s
	acordes := sets.NewSet()
	switch {
	case queryValues.Get("acordes") != "":
		for _, a := range strings.Split(queryValues.Get("acordes"), ",") {
			acordes.Add(a)
		}
	case queryValues.Get("id_unico_musica") != "":
		m, ok := musicasDict[queryValues.Get("id_unico_musica")]
		if ok {
			acordes = m.Acordes()
		} else {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	var similares []*SimilaresResponse
	for _, m := range applyFiltro(generosABuscar) {
		mArcordesSet := m.Acordes()
		inter := acordes.Intersect(mArcordesSet)
		if inter.Cardinality() > 0 {
			similares = append(similares, &SimilaresResponse{
				UniqueID:     m.UniqueID,
				IDArtista:    m.IDArtista,
				ID:           m.ID,
				Artista:      m.Artista,
				Nome:         m.Nome,
				Popularidade: m.Popularidade,
				Acordes:      m.Acordes().ToSlice(),
				Genero:       m.Genero,
				URL:          m.URL,
				Diferenca:    mArcordesSet.Difference(acordes).ToSlice(),
				Intersecao:   inter.ToSlice(),
			})
		}

	}
	sort.Sort(ProMenorDiferenca(similares))
	i, f := limitesDaPagina(len(similares), pagina)
	b, err := json.Marshal(similares[i:f])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Add("Access-Control-Allow-Origin", "*")
	fmt.Fprintf(w, string(b))
}
