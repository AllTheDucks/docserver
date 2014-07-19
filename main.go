package main

import (
	"bytes"
	"flag"
	// "fmt"
	"github.com/russross/blackfriday"
	// "github.com/shurcooL/go/github_flavored_markdown"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

var selectStatement = "select name, domain from institutions limit 10"

func main() {
	var docsDirPath string
	flag.StringVar(&docsDirPath, "docsdir", "", "Root Directory for all the Docs")
	flag.Parse()

	if docsDirPath == "" {
		flag.PrintDefaults()
		return
	}

	var err error
	docsDir, err := os.Open(docsDirPath) // For read access.
	if err != nil {
		log.Fatal(err)
		return
	}

	fi, err := docsDir.Stat()
	if err != nil {
		log.Fatal(err)
		return
	}

	if !fi.IsDir() {
		log.Fatal("%v is not a directory.", docsDirPath)
		return
	}

	fileServer := http.FileServer(http.Dir(docsDirPath))
	http.Handle("/", &MarkdownHandler{FileRoot: docsDir.Name(), FileServer: fileServer})

	http.ListenAndServe(":9000", nil)
}

type MarkdownHandler struct {
	FileServer http.Handler
	FileRoot   string
}

func (h *MarkdownHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rp := r.URL.Path

	resourcePath := filepath.Join(h.FileRoot, rp)

	var mdFilename string
	if strings.HasSuffix(rp, ".html") {
		//check for existence of html file, if it exists, serve.
		if _, err := os.Stat(resourcePath); err == nil {
			h.FileServer.ServeHTTP(w, r)
			return
		}
		//check for .md version of file, if it exists, serve.
		mdFilename = strings.TrimSuffix(resourcePath, ".html")
		mdFilename = mdFilename + ".md"
	} else if strings.HasSuffix(rp, ".md") {
		mdFilename = filepath.Join(h.FileRoot, rp)
	} else {
		h.FileServer.ServeHTTP(w, r)
		return
	}

	w.Header()["Content-Type"] = []string{"text/html"}

	flags := blackfriday.HTML_TOC | blackfriday.HTML_GITHUB_BLOCKCODE 
	
	extensions := 0
	extensions |= blackfriday.EXTENSION_NO_INTRA_EMPHASIS
	extensions |= blackfriday.EXTENSION_TABLES
	extensions |= blackfriday.EXTENSION_FENCED_CODE
	extensions |= blackfriday.EXTENSION_AUTOLINK
	extensions |= blackfriday.EXTENSION_STRIKETHROUGH
	extensions |= blackfriday.EXTENSION_SPACE_HEADERS

	body, _ := ioutil.ReadFile(mdFilename)
	output := blackfriday.Markdown(body, blackfriday.HtmlRenderer( flags,"",""), extensions)
	
	navClosingTag := []byte{'<','/','n','a','v','>'}
	navMarker := bytes.Index(output, navClosingTag)
	navMarker = navMarker + len(navClosingTag)
	
	toc := output[:navMarker]
	content := output[navMarker:] 


	data := make(map[string]string)
	data["content"] = string(content)
	data["toc"] = string(toc)

	t, _ := template.ParseFiles(filepath.Join(h.FileRoot, "template.html"))

	t.Execute(w, data)
	return
}