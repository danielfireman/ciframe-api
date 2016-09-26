package main

import (
	"bufio"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	sets "github.com/deckarep/golang-set"
)

const (
	ARTISTA_ID   = 0
	MUSICA_ID    = 1
	ARTISTA      = 2
	MUSICA       = 3
	GENERO       = 4
	POPULARIDADE = 5
	TOM          = 6
	SEQ_FAMOSA   = 7
	CIFRA        = 8
)

func loadData() {
	f, err := os.Open("data/dataset_final.csv")
	if err != nil {
		log.Fatal(err)
	}
	scanner := bufio.NewScanner(f)

	acordesSet := sets.NewSet()
	for scanner.Scan() {
		// Pré-processando cada linha.
		linha := scanner.Text()
		linha = strings.Replace(linha, "\"", "", -1)
		linha = strings.Replace(linha, "NA", "", -1)
		dados := strings.Split(linha, ",")
		musica := Musica{
			Artista:    dados[ARTISTA],
			IDArtista:  dados[ARTISTA_ID],
			ID:         dados[MUSICA_ID],
			Nome:       dados[MUSICA],
			Genero:     dados[GENERO],
			Tom:        dados[TOM],
			UniqueID:   UniqueID(dados[ARTISTA], dados[MUSICA_ID]),
			URL:        URL(dados[ARTISTA_ID], dados[MUSICA_ID]),
			SeqFamosas: strings.Split(dados[SEQ_FAMOSA], ";"),
		}

		musica.Popularidade, err = strconv.Atoi(strings.Replace(dados[POPULARIDADE], ".", "", -1))
		if err != nil {
			log.Fatal(err)
		}

		if dados[CIFRA] != "" {
			musica.Cifra = limpaCifra(strings.Split(dados[CIFRA], ";"))
		} else {
			musica.Cifra = []string{}
		}

		// inclui música no dict de músicas
		musicasDict[musica.UniqueID] = &musica

		// conjunto único de gêneros
		generosSet.Add(musica.Genero)

		// Acordes.
		mAcordes := musica.Acordes()
		// conjunto único de acordes
		acordesSet.Union(mAcordes)
		// Populando mapa de músicas por acorde.
		for a := range mAcordes.Iter() {
			if _, ok := musicasPorAcorde[a.(string)]; !ok {
				musicasPorAcorde[a.(string)] = sets.NewSet()
			}
			musicasPorAcorde[a.(string)].Add(musica.UniqueID)
		}

		// constrói dict mapeando gênero para músicas
		// deve ser usado para melhorar o desempenho das buscas
		if _, ok := musicasPorGenero[musica.Genero]; !ok {
			musicasPorGenero[musica.Genero] = sets.NewSet()
		}
		musicasPorGenero[musica.Genero].Add(musica.UniqueID)

		// popula lista com todas as músicas.
		musicas = append(musicas, &musica)
	}

	// Ordena todas as músicas por popularidade.
	sort.Sort(PorPopularidade(musicas))

	// transformando o conjunto único de acordes numa lista.
	// melhor eficiência e melhor para trabalhar com json.
	for a := range acordesSet.Iter() {
		acordes = append(acordes, a.(string))
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
