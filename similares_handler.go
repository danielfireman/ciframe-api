package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	sets "github.com/deckarep/golang-set"
	"github.com/julienschmidt/httprouter"
	"github.com/newrelic/go-agent"
	"gopkg.in/go-redis/cache.v4"
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

var sequencias = map[string]int{
	"BmGDA":   0,
	"CGAmF":   1,
	"EmG":     2,
	"CA7DmG7": 3,
	"GmF":     4,
	"CC7FFm":  5,
}

type Similares struct {
	app   newrelic.Application
	fila  chan struct{}
	cache *cache.Codec
}

func (s *Similares) GetHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		// Controlando acesso concorrente;
		s.fila <- struct{}{}
		defer func() {
			<-s.fila
		}()

		txn := s.app.StartTransaction("similares", w, r)
		defer txn.End()

		queryValues := r.URL.Query()
		pagina, err := getPaginaFromRequest(r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Primeiro coisa a fazer é olhar o cache.
		var response []*SimilaresResponse
		if err := s.cache.Get(r.URL.RawQuery, &response); err == nil && len(response) != 0 {
			b, err := s.toBytes(r.URL.RawQuery, response, pagina)
			if err != nil {
				log.Printf("Erro processando request [%s]: '%q'", r.URL.String(), err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Add("Access-Control-Allow-Origin", "*")
			fmt.Fprintf(w, string(b))
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
				b, err := s.toBytes(r.URL.RawQuery, response, pagina)
				if err != nil {
					log.Printf("Erro processando request [%s]: '%q'", r.URL.String(), err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.Header().Add("Access-Control-Allow-Origin", "*")
				fmt.Fprintf(w, string(b))
				return
			}
		}

		// tratamento do requists
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
			if m, ok := musicasPorAcorde[a.(string)]; ok {
				musicasSimilares = musicasSimilares.Union(m)
			}
		}
		if generosABuscar.Cardinality() > 0 {
			porGenero := sets.NewSet()
			for g := range generosABuscar.Iter() {
				if m, ok := musicasPorGenero[g.(string)]; ok {
					porGenero = porGenero.Union(m)
				}
			}
			musicasSimilares = musicasSimilares.Intersect(porGenero)
		}

		for mID := range musicasSimilares.Iter() {
			m := musicasDict[mID.(string)]
			mAcordesSet := m.Acordes()
			if mAcordesSet.Cardinality() > 1 && queryValues.Get("id_unico_musica") != m.UniqueID {
				response = append(response, &SimilaresResponse{
					UniqueID:     m.UniqueID,
					IDArtista:    m.IDArtista,
					ID:           m.ID,
					Artista:      m.Artista,
					Nome:         m.Nome,
					Popularidade: m.Popularidade,
					Acordes:      mAcordesSet.ToSlice(),
					Genero:       m.Genero,
					URL:          m.URL,
					Diferenca:    mAcordesSet.Difference(acordes).ToSlice(),
					Intersecao:   mAcordesSet.Intersect(acordes).ToSlice(),
				})
			}
		}
		buildSegment.End()
		b, err := s.toBytes(r.URL.RawQuery, response, pagina)
		if err != nil {
			log.Printf("Erro processando request [%s]: '%q'", r.URL.String(), err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Add("Access-Control-Allow-Origin", "*")
		fmt.Fprintf(w, string(b))
	}
}

func (s *Similares) toBytes(cacheKey string, response []*SimilaresResponse, pagina int) ([]byte, error) {
	// Para retornar, primeiro ordenamos
	sort.Sort(PorMenorDiferenca(response))

	// Consideramos os limites da página.
	i, f := limitesDaPagina(len(response), pagina)

	// Colocamos no cache.
	s.cache.Set(&cache.Item{
		Key:        cacheKey,
		Object:     response[i:f],
		Expiration: time.Hour,
	})

	// Convertemos para JSON.
	b, err := json.Marshal(response[i:f])
	if err != nil {
		return nil, err
	}
	return b, nil
}
