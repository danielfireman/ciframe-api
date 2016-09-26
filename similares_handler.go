package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sort"

	sets "github.com/deckarep/golang-set"
	"github.com/julienschmidt/httprouter"
	"github.com/newrelic/go-agent"
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
type PorMenorDiferenca []*SimilaresResponse

func (p PorMenorDiferenca) Len() int {
	return len(p)
}
func (p PorMenorDiferenca) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
func (p PorMenorDiferenca) Less(i, j int) bool {
	return len(p[i].Diferenca) < len(p[j].Diferenca)
}

type Similares struct {
	app newrelic.Application
}

var sequencias = map[string]int{
	"BmGDA":   0,
	"CGAmF":   1,
	"EmG":     2,
	"CA7DmG7": 3,
	"GmF":     4,
	"CC7FFm":  5,
}

func (s *Similares) GetHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		txn := s.app.StartTransaction("similares", w, r)
		defer txn.End()

		queryValues := r.URL.Query()
		pagina, err := getPaginaFromRequest(r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		generosABuscar := generosFromRequest(r)

		if queryValues.Get("sequencia") != "" {
			acordes := strings.Replace(queryValues.Get("sequencia"), ",", "", -1)
			var response []*SimilaresResponse
			idSeq, ok := sequencias[acordes]
			if ok {
				strIdSeq := strconv.Itoa(idSeq)
				for _, m := range applyFiltro(generosABuscar) {
					for _, seq := range m.SeqFamosas {
						if seq == strIdSeq {
							response = append(response, &SimilaresResponse{
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
				sort.Sort(PorMenorDiferenca(response))
				b, err := json.Marshal(response)
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
		buildSegment := newrelic.StartSegment(txn, "similares_find")
		musicasSimilares := sets.NewSet()
		for a := range acordes.Iter() {
			if _, ok := musicasPorAcorde[a.(string)]; ok {
				musicasSimilares.Union(musicasPorAcorde[a.(string)])
			}
		}
		if generosABuscar.Cardinality() > 0 {
			porGenero := sets.NewSet()
			for g := range generosABuscar.Iter() {
				if _, ok := musicasPorAcorde[g.(string)]; ok {
					porGenero.Union(generosMusicas[g.(string)])
				}
			}
			musicasSimilares.Intersect(porGenero)
		}
		var response []*SimilaresResponse
		for mI := range musicasSimilares.Iter() {
			m := mI.(*Musica)
			mArcordesSet := m.Acordes()
			response = append(response, &SimilaresResponse{
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
				Intersecao:   mArcordesSet.Intersect(acordes).ToSlice(),
			})
		}

		buildSegment.End()
		sortSegment := newrelic.StartSegment(txn, "similares_sort")
		sort.Sort(PorMenorDiferenca(response))
		sortSegment.End()

		i, f := limitesDaPagina(len(response), pagina)

		marshallingSegment := newrelic.StartSegment(txn, "similares_marshalling")
		b, err := json.Marshal(response[i:f])
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		marshallingSegment.End()

		w.Header().Add("Access-Control-Allow-Origin", "*")
		fmt.Fprintf(w, string(b))
	}
}
