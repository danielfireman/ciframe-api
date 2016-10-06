package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	sets "github.com/deckarep/golang-set"
	"github.com/julienschmidt/httprouter"
	"github.com/newrelic/go-agent"
	"gopkg.in/go-redis/cache.v4"
	"gopkg.in/redis.v4"
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
	log.Println("Monitoramento NewRelic configurado com sucesso.")

	redisCache, err := Redis(os.Getenv("REDIS_URL"))
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Redis cache conectado.")

	loadData()
	log.Println("Dados carregados com sucesso.")

	router := httprouter.New()
	g, err := NewGeneros(app, generosSet)
	if err != nil {
		log.Fatal(err)
	}
	router.GET("/generos", MonitoredEndpoint(app, "generos", g.GetHandler()))
	router.OPTIONS("/generos", openCORS)

	// Controlando o acesso concorrente: 10 requisições por segundo.
	s := Similares{app, make(chan struct{}, 10), redisCache}
	router.GET("/similares", s.GetHandler())
	router.OPTIONS("/similares", openCORS)

	router.GET("/search", MonitoredEndpoint(app, "search", SearchHandler))
	router.OPTIONS("/search", MonitoredEndpoint(app, "search_cors", openCORS))

	router.GET("/musicas", MonitoredEndpoint(app, "musicas", MusicasHandler))
	router.OPTIONS("/musicas", MonitoredEndpoint(app, "musicas_cors", openCORS))

	router.GET("/musica/:id", MonitoredEndpoint(app, "get_musica", MusicasHandler))
	router.OPTIONS("/musica/:id", MonitoredEndpoint(app, "get_musica_cors", openCORS))

	router.GET("/acordes", MonitoredEndpoint(app, "acordes", AcordesHandler))
	router.OPTIONS("/acordes", MonitoredEndpoint(app, "acordes_cors", openCORS))

	log.Println("Serviço inicializado na porta ", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func openCORS(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	w.Header().Set("Access-Control-Allow-Headers:", "accept, content-type")
	w.Header().Set("Access-Control-Allow-Methods:", "POST")
	w.Header().Set("Access-Control-Allow-Origin", "http://lp.usemyto.com")
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

type Musica struct {
	IDArtista    string   `json:"id_artista"`
	UniqueID     string   `json:"id_unico_musica"`
	Genero       string   `json:"genero"`
	ID           string   `json:"id_musica"`
	Artista      string   `json:"nome_artista"`
	Nome         string   `json:"nome_musica"`
	URL          string   `json:"url"`
	Popularidade int      `json:"popularidade"`
	Cifra        []string `json:"cifra"`
	SeqFamosas   []string `json:"seq_famosas"`
	Tom          string   `json:"tom"`
}

func (m *Musica) Acordes() sets.Set {
	acordes := sets.NewSet()
	for _, c := range m.Cifra {
		acordes.Add(c)
	}
	return acordes
}

func UniqueID(artista, id string) string {
	return fmt.Sprintf("%s_%s", artista, id)
}

func URL(artista, id string) string {
	return fmt.Sprintf("http://www.cifraclub.com.br/%s/%s", artista, id)
}

// PorPopularidade implementa sort.Interface for []*Musica baseado no campo Popularidade
type PorPopularidade []*Musica

func (p PorPopularidade) Len() int           { return len(p) }
func (p PorPopularidade) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p PorPopularidade) Less(i, j int) bool { return p[i].Popularidade > p[j].Popularidade }

var acordes []string
var musicasDict = make(map[string]*Musica) // Mapa de músicas indexado por ids únicos.
var generosSet = sets.NewSet()
var musicas []*Musica // todas as músicas, ordenadas por popularidade.

// Os conjuntos contém ids das músicas
var musicasPorAcorde = make(map[string]sets.Set)
var musicasPorGenero = make(map[string]sets.Set)

func limitesDaPagina(size int, pagina int) (int, int) {
	i := (pagina - 1) * TAM_PAGINA
	return i, int(math.Min(float64(i+TAM_PAGINA), float64(size)))
}

func getPaginaFromRequest(r *http.Request) (int, error) {
	pagina := 1
	if r.URL.Query().Get("pagina") != "" {
		p, err := strconv.Atoi(r.URL.Query().Get("pagina"))
		if err != nil {
			return -1, err
		}
		pagina = p
	}
	return pagina, nil
}

// retorns generos do request (podem ser separados por vírgula).
func generosFromRequest(r *http.Request) sets.Set {
	returned := sets.NewSet()
	if r.URL.Query().Get("generos") != "" {
		for _, g := range strings.Split(r.URL.Query().Get("generos"), ",") {
			returned.Add(g)
		}
	}
	return returned
}

func applyFiltro(generos sets.Set) []*Musica {
	if generos.Cardinality() == 0 {
		return musicas
	}
	var collection []*Musica
	for g := range generos.Iter() {
		if generosSet.Contains(g.(string)) {
			for m := range musicasPorGenero[g.(string)].Iter() {
				collection = append(collection, musicasDict[m.(string)])
			}
		}
	}
	return collection
}

func Redis(u string) (*cache.Codec, error) {
	if u == "" {
		return nil, fmt.Errorf("$REDIS_URL must be set")
	}
	redisURL, err := url.Parse(u)
	if err != nil || redisURL.User == nil {
		return nil, fmt.Errorf("Ocorreu um erro no parse da URL REDIS ou o usuário não foi determinado. err:'%q'\n", err)
	}
	pwd, ok := redisURL.User.Password()
	if !ok {
		return nil, fmt.Errorf("Não foi possível extrair a senha de REDIS_URL: %s", redisURL)
	}
	return &cache.Codec{
		Redis: redis.NewClient(&redis.Options{
			Addr:     redisURL.Host,
			Password: pwd,
		}),
		Marshal:   json.Marshal,
		Unmarshal: json.Unmarshal,
	}, nil
}
