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
	ARTISTA_ID = 0
	MUSICA_ID = 1
	ARTISTA = 2
	MUSICA = 3
	GENERO = 4
	POPULARIDADE = 5
	TOM = 6
	SEQ_FAMOSA = 7
	CIFRA = 8
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
	Popularidade int `json:"popularidade"`
	Cifra        []string `json:"cifra"`
	SeqFamosas   []string `json:"seq_famosas"`
	Tom          string `json:"tom"`
	Genero       string `json:"genero"`
	Artista      string `json:"artista"`
	IDArtista    string `json:"id_artista"`
	UniqueID     string `json:"id_unico_musica"`
	Nome         string `json:"popularidade"`
	ID           string `json:"id_musica"`
	URL          string `json:"url"`
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
	return p[i].Popularidade < p[j].Popularidade
}

var acordes = make(map[string]struct{})
var musicasDict = make(map[string]*Musica)
var generosMusicas = make(map[string][]*Musica)
var musicas []*Musica
var generosSet = make(map[string]struct{})
var generos []string

// Busca por músicas que possuem no título ou no nome do artista o argumento passado por key.
// params: key e generos (opcional). Caso generos não sejam definidos, a busca não irá filtrar por gênero.
// exemplo 1: /search?key=no dia em que eu saí de casa
// exemplo 2: /search?key=no dia em que eu saí de casa&generos=Rock,Samba '''
func search(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var generosABuscar []string

	if p.ByName("generos") == "" {
		generosABuscar = generos
	} else {
		generosABuscar = strings.Split(p.ByName("generos"), ",")
	}

	pagina := 1
	if p.ByName("pagina") != "" {
		p, err := strconv.Atoi(p.ByName("pagina"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
		}
		pagina = p
	}

	keys := strings.Split(removerCombinantes(strings.ToLower(p.ByName("key"))), " ")
	var resultado []*Musica
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
			resultado = append(resultado, m)
		}
	}

	b, err := json.Marshal(getPagina(resultado, pagina))
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
