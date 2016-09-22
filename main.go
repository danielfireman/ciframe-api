package main

import (
	"fmt"
	"net/http"
	"os"

	"strings"
	"strconv"
	"log"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"unicode"
	"encoding/json"
	"math"
	"github.com/julienschmidt/httprouter"
	"github.com/newrelic/go-agent"
	"sort"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("$PORT must be set")
	}
	log.Println("Porta utilizada", port)

	nrLicence := os.Getenv("NEW_RELIC_LICENSE_KEY")
	if nrLicence == "" {
		log.Fatal("$NEW_RELIC_LICENSE_KEY must be set")
	}
	config := newrelic.NewConfig("ciframe-api", nrLicence)
	app, err := newrelic.NewApplication(config)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Monitoramento NewRelic configurado com sucesso.", port)

	loadData()
	log.Println("Dados carregados com sucesso.")

	router := httprouter.New()
	router.GET("/search", MonitoredEndpoint(app, "search", search))

	log.Println("Serviço inicializado.")

	log.Fatal(http.ListenAndServe(":" + port, router))
}

func MonitoredEndpoint(app newrelic.Application, name string, h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		txn := app.StartTransaction(name, w, r)
		defer txn.End()
		h(w, r, p)
	}
}

const (
	TAM_PAGINA = 100
)

var sequencias = map[string]int{
	"BmGDA" : 0,
	"CGAmF" : 1,
	"EmG" : 2,
	"CA7DmG7" : 3,
	"GmF" : 4,
	"CC7FFm" : 5,
}

type Musica struct {
	IDArtista    string `json:"id_artista"`
	UniqueID     string `json:"id_unico_musica"`
	Genero       string `json:"genero"`
	ID           string `json:"id_musica"`
	Artista      string `json:"nome_artista"`
	Nome         string `json:"nome_musica"`
	URL          string `json:"url"`
	Popularidade int `json:"popularidade"`
	Cifra        []string `json:"cifra"`
	SeqFamosas   []string `json:"seq_famosas"`
	Tom          string `json:"tom"`
}

type SearchResponse struct {
	IDArtista string `json:"id_artista"`
	UniqueID  string `json:"id_unico_musica"`
	Genero    string `json:"genero"`
	ID        string `json:"id_musica"`
	Artista   string `json:"nome_artista"`
	Nome      string `json:"nome_musica"`
	URL       string `json:"url"`
	Popularidade int `json:"popularidade"`
}

func UniqueID(artista, id string) string {
	return fmt.Sprintf("%s_%s", artista, id)
}

func URL(artista, id string) string {
	return fmt.Sprintf("http://www.cifraclub.com.br/%s/%s", artista, id)
}

func (m *Musica) Acordes() map[string]struct{} {
	acordes := make(map[string]struct{})
	for _, c := range m.Cifra {
		acordes[c] = struct{}{}
	}
	return acordes
}

// PorPopularidade implementa sort.Interface for []*Musica baseado no campo Popularidade
type PorPopularidade []*Musica

func (p PorPopularidade) Len() int {
	return len(p)
}
func (p PorPopularidade) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
func (p PorPopularidade) Less(i, j int) bool {
	return p[i].Popularidade > p[j].Popularidade
}

var acordes = make(map[string]struct{})
var musicasDict = make(map[string]*Musica)
var generosMusicas = make(map[string][]*Musica)
var generosSet = make(map[string]struct{})
var generos []string

// Busca por músicas que possuem no título ou no nome do artista o argumento passado por key.
// params: key e generos (opcional). Caso generos não sejam definidos, a busca não irá filtrar por gênero.
// exemplo 1: /search?key=no dia em que eu saí de casa
// exemplo 2: /search?key=no dia em que eu saí de casa&generos=Rock,Samba '''
func search(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	queryValues := r.URL.Query()

	var generosABuscar []string
	if queryValues.Get("generos") == "" {
		generosABuscar = generos
	} else {
		generosABuscar = strings.Split(queryValues.Get("generos"), ",")
	}

	pagina := 1
	if queryValues.Get("pagina") != "" {
		p, err := strconv.Atoi(queryValues.Get("pagina"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
		}
		pagina = p
	}

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
	for _, m := range getPagina(musicasRes, pagina) {
		resultado = append(resultado, SearchResponse{
			IDArtista: m.IDArtista,
			UniqueID : m.UniqueID,
			Genero: m.Genero,
			ID : m.ID,
			Artista: m.Artista,
			Nome: m.Nome,
			URL: m.URL,
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

func getPagina(m []*Musica, pagina int) []*Musica {
	inicioPagina := (pagina - 1) * TAM_PAGINA
	finalPagina := int(math.Min(float64(inicioPagina + TAM_PAGINA), float64(len(m))))
	return m[inicioPagina: finalPagina]
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

func applyFiltro(generos []string) []*Musica {
	var collection []*Musica
	for _, g := range generos {
		if _, ok := generosSet[g]; ok {
			collection = append(collection, generosMusicas[g]...)
		}
	}
	return collection
}
