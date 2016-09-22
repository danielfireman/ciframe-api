package ciframe_api

import (
	"fmt"
	"net/http"
	"bufio"
	"os"

	"strings"
	"strconv"
	"regexp"
	"sort"
	"log"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"unicode"
	"encoding/json"
	"math"
)

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

func init() {
	loadData()

	http.HandleFunc("/search", search)
}

// Busca por músicas que possuem no título ou no nome do artista o argumento passado por key.
// params: key e generos (opcional). Caso generos não sejam definidos, a busca não irá filtrar por gênero.
// exemplo 1: /search?key=no dia em que eu saí de casa
// exemplo 2: /search?key=no dia em que eu saí de casa&generos=Rock,Samba '''
func search(w http.ResponseWriter, r *http.Request) {
	var generosABuscar []string
	if r.URL.Query().Get("generos") == "" {
		generosABuscar = generos
	} else {
		generosABuscar = strings.Split(r.URL.Query().Get("generos"), ",")
	}

	pagina := 1
	if r.URL.Query().Get("pagina") != "" {
		p, err := strconv.Atoi(r.URL.Query().Get("pagina"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
		}
		pagina = p
	}

	keys := strings.Split(removerCombinantes(strings.ToLower(r.URL.Query().Get("key"))), " ")
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
		// do appengine logging.
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

func loadData() {
	f, err := os.Open("data/dataset_final.csv")
	if err != nil {
		log.Fatal(err);
	}
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		// Pré-processando cada linha.
		linha := scanner.Text()
		linha = strings.Replace(linha, "\"", "", -1)
		linha = strings.Replace(linha, "NA", "", -1)
		dados := strings.Split(linha, ",")
		musica := Musica{
			Artista:dados[ARTISTA],
			IDArtista:dados[ARTISTA_ID],
			ID:dados[MUSICA_ID],
			Nome:dados[MUSICA],
			Genero: dados[GENERO],
			Tom: dados[TOM],
			UniqueID: UniqueID(dados[ARTISTA], dados[MUSICA_ID]),
			URL: URL(dados[ARTISTA], dados[MUSICA_ID]),
		}

		musica.Popularidade, err = strconv.Atoi(strings.Replace(dados[POPULARIDADE], ".", "", -1))
		if err != nil {
			log.Fatal(err);
		}

		if dados[CIFRA] != "" {
			musica.Cifra = limpaCifra(strings.Split(dados[CIFRA], ";"))
		} else {
			musica.Cifra = []string{}
		}

		musica.SeqFamosas = strings.Split(dados[SEQ_FAMOSA], ";")
		// inclui música no dict de músicas
		musicasDict[musica.UniqueID] = &musica

		// inclui a música na lista que será ordenada por popularidade
		musicas = append(musicas, &musica)

		// conjunto único de gêneros
		generosSet[musica.Genero] = struct{}{}

		// conjunto único de acordes
		for a := range musica.Acordes() {
			acordes[a] = struct{}{}
		}

		// constrói dict mapeando gênero para músicas
		// deve ser usado para melhorar o desempenho das buscas
		generosMusicas[musica.Genero] = append(generosMusicas[musica.Genero], &musica)
	}

	// ordena musicas por popularidade
	sort.Sort(PorPopularidade(musicas))

	// para trabalhar melhor com json
	for g := range generosSet {
		generos = append(generos, g)
	}

	// ordena músicas de cada gênero por popularidade
	for _, v := range generosMusicas {
		sort.Sort(PorPopularidade(v))
	}
}

func limpaCifra(rawCifra []string) []string {
	var cifra []string
	for _, m := range rawCifra {
		m = strings.Trim(m, " ")
		if len(m) != 0 {
			if strings.Contains(m, "|") {
				// filtra tablaturas
				acorde := strings.Split(m, "|")[0]
				acorde = pythonSplit(acorde)[0]
				cifra = append(cifra, acorde)
			} else {
				// lida com acordes separados por espaço
				cifra = append(cifra, pythonSplit(m)...)
			}
		}
	}
	return cifra
}

// Mais perto que consegui da função split() em python.
// A idéia é converter múltiplos espaços consecutivos em um espaço e então fazer split.
var multiplosEspacos = regexp.MustCompile(" +")

func pythonSplit(s string) []string {
	return strings.Split(multiplosEspacos.ReplaceAllString(s, " "), " ")
}
