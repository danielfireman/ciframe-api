package main

import (
	"sort"
	"strings"
	"regexp"
	"os"
	"bufio"
	"strconv"
	"log"
)

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


