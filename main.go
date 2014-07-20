package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/russross/blackfriday"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"strconv"
	"text/template"
	htmltemplate "html/template"
	"regexp"
)

var h1RegEx = regexp.MustCompile("<[\\w]*h1.*>(.*)</[\\w]*h1[\\w]*>")	

func main() {
	var docsDirPath string
	var port int

	flag.StringVar(&docsDirPath, "docsdir", "", "Root Directory for all the Docs")
	flag.IntVar(&port, "port", 9000, "Port to run the server on")
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

	editServer := http.FileServer(http.Dir("editor"))
	fileServer := http.FileServer(http.Dir(docsDirPath))

 	http.Handle("/editor/", http.StripPrefix("/editor/",editServer)) 
	http.Handle("/", &MarkdownHandler{FileRoot: docsDir.Name(), FileServer: fileServer, EditServer: editServer})

	log.Fatal(http.ListenAndServe(":" + strconv.Itoa(port), nil))
}

type MarkdownHandler struct {
	FileServer http.Handler
	EditServer http.Handler
	FileRoot   string
}

func (h *MarkdownHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rp := r.URL.Path

	var hasMdFile = false
	var filename, mdUrl string
	if strings.HasSuffix(rp, ".html") {
		mdUrl = strings.TrimSuffix(rp, ".html") + ".md"
		filename = filepath.Join(h.FileRoot, mdUrl)

		if _, err := os.Stat(filename); err == nil {
			hasMdFile = true;
		}
	} 

	if !hasMdFile {
		filename = filepath.Join(h.FileRoot, rp)
	}

	if r.Method == "POST" {
		if err := os.MkdirAll(filepath.Dir(filename), 0744); err != nil {
			log.Printf("Error Making Directories: %v %v", filename, err)
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err == nil {
			ioutil.WriteFile(filename, body, 0644)
		}
		return
	}

	r.ParseForm()
	_, editMode := r.Form["edit"]

	if editMode {
		if hasMdFile {
			w.Header()["Location"] = []string{fmt.Sprintf("%s?%s", mdUrl, r.URL.RawQuery)}
			w.WriteHeader(http.StatusFound)
		}
		w.Header()["Content-Type"] = []string{"text/html"}

		t, _ := htmltemplate.ParseFiles("editor/editor.html")

		content, _ := ioutil.ReadFile(filename)

		data := make(map[string]string)
		data["path"] = rp;
		data["content"] = string(content)


		t.Execute(w, data)
		return
	}

	if !hasMdFile {
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

	body, _ := ioutil.ReadFile(filename)
	output := blackfriday.Markdown(body, blackfriday.HtmlRenderer( flags,"",""), extensions)
	
	navClosingTag := []byte{'<','/','n','a','v','>'}
	navMarker := bytes.Index(output, navClosingTag)
	navMarker = navMarker + len(navClosingTag)
	
	toc := output[:navMarker]
	content := output[navMarker:]

	var title []byte
	if headingSearch := h1RegEx.FindSubmatch(content); headingSearch != nil && len(headingSearch) >= 2 {
		title = headingSearch[1]
	}

	data := make(map[string]string)
	data["content"] = string(content)
	data["toc"] = string(toc)
	data["title"] = string(title)

	t, _ := template.ParseFiles(filepath.Join(h.FileRoot, "template.html"))

	t.Execute(w, data)
	return
}
