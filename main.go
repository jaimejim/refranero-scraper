package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

var (
	printSlugs bool
	readSlugs  bool
)

type refran struct {
	idiom      string
	usage      string
	definition string

	err error
}

const (
	// Idioms start only with the below letters (http://cvc.cervantes.es/lengua/refranero/listado.aspx)
	letters = "ABCDEFGHIJLMNOPQRSTUVYZ"

	baseURL      = "http://cvc.cervantes.es/lengua/refranero"
	alphaPageURL = "http://cvc.cervantes.es/lengua/refranero/listado.aspx?letra="

	networkConns = 10

	sectionUsage      = "Marcador de uso:"
	sectionIdiom      = "Enunciado:"
	sectionDefinition = "Significado:"

	usageComment = "Comentario al marcador de uso"

	missingMarker = ""
)

func main() {
	parseFlags()
	if printSlugs {
		outSlugs()
	} else if readSlugs {
		inSlugs()
	}
}

func parseFlags() {
	flag.BoolVar(&printSlugs, "print-slugs", false, "crawl all links for the entire alphabet, and print them line-by-line to stdout")
	flag.BoolVar(&readSlugs, "read-slugs", false, "read slugs, line by line, from stdin, and print the idiom, definition, etc if it is heavily used.")
	flag.Parse()
	if printSlugs && readSlugs {
		log.Fatal("Both -print-slugs and -read-slugs cannot be passed.")
	}
}

func inSlugs() {
	var done sync.WaitGroup
	output := make(chan refran, 0)
	jobs := make(chan string, 0)
	scanner := bufio.NewScanner(os.Stdin)
	for i := 0; i < networkConns; i++ {
		done.Add(1)
		go func() {
			for j := range jobs {
				r := refran{}
				doc, err := goquery.NewDocument(fmt.Sprintf("%s/%s", baseURL, j))
				if err != nil {
					r.err = err
					output <- r
					continue
				}
				sel := doc.Find("div.tabbertab").First()
				r.idiom = getSectionText(sel, sectionIdiom)
				r.usage = getSectionText(sel, sectionUsage)
				usageParts := strings.Split(r.usage, usageComment)
				r.usage = usageParts[0] // Remove any comments about usage
				r.definition = getSectionText(sel, sectionDefinition)
				output <- r
			}
			defer done.Done()
		}()
	}
	go func() {
		for scanner.Scan() {
			jobs <- scanner.Text()
		}
		close(jobs)
	}()
	go func() {
		done.Wait()
		close(output)
	}()
	fmt.Printf("Refran\tSignificado\tUso\n")
	for r := range output {
		if r.err != nil {
			fmt.Printf("ERROR: %s\n", r.err)
			continue
		}
		if r.isEmpty() {
			continue
		}
		fmt.Printf("%s\t%s\t%s\n", r.idiom, r.definition, r.usage)
	}
}

func (r refran) isEmpty() bool {
	return r.idiom == missingMarker && r.definition == missingMarker && r.usage == missingMarker
}

func getSectionText(sel *goquery.Selection, section string) string {
	foundIdx := -1
	headers := sel.Find("p > strong").Each(func(i int, s *goquery.Selection) {
		if strings.TrimSpace(s.Text()) == section {
			foundIdx = i
		}
	})
	if foundIdx == -1 {
		return missingMarker
	}
	parent := headers.Eq(foundIdx).Parent()
	text := strings.TrimPrefix(parent.Text(), section)
	text = strings.Replace(text, "\n", " ", -1)
	text = strings.TrimSpace(text)
	return text
}

func outSlugs() {
	var wg sync.WaitGroup
	slugs := make(chan string, 0)
	for _, letter := range letters {
		wg.Add(1)
		go func(letter rune) {
			defer wg.Done()
			doc, err := goquery.NewDocument(fmt.Sprintf("%s%c", alphaPageURL, letter))
			if err != nil {
				log.Fatal(err)
			}
			doc.Find("ol#lista_az > li > a").Each(func(i int, s *goquery.Selection) {
				link, ok := s.Attr("href")
				if !ok {
					return
				}
				slugs <- link
			})
		}(letter)
	}
	go func() {
		wg.Wait()
		close(slugs)
	}()
	for slug := range slugs {
		fmt.Println(slug)
	}
}
