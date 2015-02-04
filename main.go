package main

import (
	"code.google.com/p/go.crypto/bcrypt"
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"flag"
	"fmt"
	"github.com/howeyc/gopass"
	"github.com/russross/blackfriday"
	htmltemplate "html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
)

var h1RegEx = regexp.MustCompile("<[\\w]*h1.*>(.*)</[\\w]*h1[\\w]*>")

func main() {
	var unsanitiedUserFilePath, unsanitisedDocsDirPath, unsanitiedEditorDirPath, fileExtWhitelistStr string
	var port int
	var isAddUser bool

	flag.StringVar(&unsanitiedUserFilePath, "users", "users", "Files containing users data.")
	flag.StringVar(&unsanitisedDocsDirPath, "docsdir", "content", "Directory that contains all the documentation.")
	flag.StringVar(&unsanitiedEditorDirPath, "editordir", "editor", "Directory that contains the editor files.")
	flag.IntVar(&port, "port", 9000, "The port to bind the server to.")
	flag.BoolVar(&isAddUser, "adduser", false, "Instead of running the doc server, add a user to the password file.")
	flag.StringVar(&fileExtWhitelistStr, "editablefileext", "md,html,css,js,txt,csv,java", "A comma seperated list of editable file extensions.")
	flag.Parse()

	if unsanitiedUserFilePath == "" {
		flag.PrintDefaults()
		return
	}

	usersFile, dir, err := sanitisePath(unsanitiedUserFilePath)
	if err != nil {
		log.Println(fmt.Sprintf("Failed to open users file for reading: %v, %v", unsanitiedUserFilePath, err))
		return
	}
	if(dir) {
		log.Println(fmt.Sprintf("Specified users file is a directory: %v", unsanitiedUserFilePath))
		return
	}

	users, err := decodeUserFile(usersFile)
	if err != nil {
		if !isAddUser {
			log.Println("Failed to open users file. Either it doesn't exist or cannot be accessed. Use the -adduser flag to create a new one.")
			return
		}
		users = make(map[string][]byte)
	}

	if isAddUser {
		addUser(usersFile, users)
		return
	}

	if unsanitisedDocsDirPath == "" || unsanitiedEditorDirPath == "" || port <= 0 || port > 65535 {
		flag.PrintDefaults()
		return
	}

	docsDirPath, dir, err := sanitisePath(unsanitisedDocsDirPath)
	if err != nil {
		log.Println(fmt.Sprintf("Failed to open docs directory for reading: %v, %v", unsanitisedDocsDirPath, err))
		return
	}
	if(!dir) {
		log.Println(fmt.Sprintf("Specified docs path is not a directory: %v", unsanitisedDocsDirPath))
		return
	}

	editorDirPath, dir, err := sanitisePath(unsanitiedEditorDirPath)
	if err != nil {
		log.Println(fmt.Sprintf("Failed to open editor directory for reading: %v, %v", unsanitiedEditorDirPath, err))
		return
	}
	if(!dir) {
		log.Println(fmt.Sprintf("Specified editor path is not a directory: %v", unsanitiedEditorDirPath))
		return
	}

	fileExtWhitelistMap := make(map[string]struct{})
	for _, ext := range strings.Split(fileExtWhitelistStr, ",") {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		fileExtWhitelistMap[ext] = struct{}{}
	}

	editServer := http.FileServer(http.Dir(editorDirPath))
	fileServer := http.FileServer(http.Dir(docsDirPath))

	http.Handle("/editor/", http.StripPrefix("/editor/", editServer))
	http.Handle("/", &MarkdownHandler{
		DocRoot:    docsDirPath,
		EditorRoot: editorDirPath,
		FileServer: fileServer,
		EditServer: editServer,
		Users: users,
		EditableFileWhitelist: fileExtWhitelistMap,
		})

	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))
}

type MarkdownHandler struct {
	FileServer 				http.Handler
	EditServer 				http.Handler
	DocRoot    				string
	EditorRoot 				string
	Users      				map[string][]byte
	EditableFileWhitelist	map[string]struct{}
}

func (h *MarkdownHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if strings.HasSuffix(path, "/") {
		path += "index.html"
	}
	filename := filepath.Join(h.DocRoot, path)

	if r.Method == "POST" {
		h.save(w, r, filename)
		return
	}	

	r.ParseForm()
	_, editMode := r.Form["edit"]

	if editMode {
		h.editor(w, r, filename, path)
	} else {
		h.serve(w, r, filename)
	}
}


func (h *MarkdownHandler) serve(w http.ResponseWriter, r *http.Request, filename string) {
	if _, err := os.Stat(filename); err == nil {
		h.FileServer.ServeHTTP(w, r)
		return
	}

	if strings.HasSuffix(filename, ".html") {
		mdFile := strings.TrimSuffix(filename, ".html") + ".md";
		if _, err := os.Stat(mdFile); err == nil {
			h.serveMarkdown(w, r, mdFile)
			return
		}
	}

	w.WriteHeader(http.StatusNotFound);
}

func (h *MarkdownHandler) serveMarkdown(w http.ResponseWriter, r *http.Request, mdFile string) {
	w.Header()["Content-Type"] = []string{"text/html"}

	//flags := blackfriday.HTML_TOC | blackfriday.HTML_GITHUB_BLOCKCODE
	flags := blackfriday.HTML_TOC

	extensions := 0
	extensions |= blackfriday.EXTENSION_NO_INTRA_EMPHASIS
	extensions |= blackfriday.EXTENSION_TABLES
	extensions |= blackfriday.EXTENSION_FENCED_CODE
	extensions |= blackfriday.EXTENSION_AUTOLINK
	extensions |= blackfriday.EXTENSION_STRIKETHROUGH
	extensions |= blackfriday.EXTENSION_SPACE_HEADERS

	body, _ := ioutil.ReadFile(mdFile)
	output := blackfriday.Markdown(body, blackfriday.HtmlRenderer(flags, "", ""), extensions)

	navClosingTag := []byte{'<', '/', 'n', 'a', 'v', '>'}
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

	t, _ := template.ParseFiles(filepath.Join(h.DocRoot, "template.html"))

	t.Execute(w, data)
}

func (h *MarkdownHandler) editor(w http.ResponseWriter, r *http.Request, filename string, path string) {
	if requiresAuth(w, r, h.Users) || blockedFileExt(w, h.EditableFileWhitelist, filename) {
		return
	}

	if _, err := os.Stat(filename); err == nil {
		h.serveEditor(w, filename, path)
		return
	}

	if strings.HasSuffix(filename, ".html") {
		mdFile := strings.TrimSuffix(filename, ".html") + ".md";
		if _, err := os.Stat(mdFile); err == nil {
			mdPath := strings.TrimSuffix(path, ".html") + ".md";
			w.Header()["Location"] = []string{fmt.Sprintf("%s?%s", mdPath, r.URL.RawQuery)}
			w.WriteHeader(http.StatusFound)
			return
		}
	}

	h.serveEditor(w, filename, path)
}

func (h *MarkdownHandler) serveEditor(w http.ResponseWriter, filename string, path string) {
	w.Header()["Content-Type"] = []string{"text/html"}

	editorHtml := filepath.Join(h.EditorRoot, "editor.html")
	t, _ := htmltemplate.ParseFiles(editorHtml)

	content, _ := ioutil.ReadFile(filename)

	data := make(map[string]string)
	data["path"] = path
	data["content"] = string(content)

	t.Execute(w, data)
}

func (h *MarkdownHandler) save(w http.ResponseWriter, r *http.Request, filename string) {
	if requiresAuth(w, r, h.Users) || blockedFileExt(w, h.EditableFileWhitelist, filename) {
		return
	}

	if err := os.MkdirAll(filepath.Dir(filename), 0744); err != nil {
		log.Printf("Error making directories: %v %v", filename, err)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err == nil {
		ioutil.WriteFile(filename, body, 0644)
	}
	return
}


func sanitisePath(path string) (string, bool, error) {
	directory, err := os.Open(path)
	defer directory.Close()
	if err != nil {
		return "", false, err
	}

	fi, err := directory.Stat()
	if err != nil {
		return "", false, err
	}

	return directory.Name(), fi.IsDir(), nil
}

func addUser(path string, usersMap map[string][]byte) {
	bio := bufio.NewReader(os.Stdin)

	fmt.Printf("Username: ")
	username, _ := bio.ReadString('\n')
	username = strings.TrimSpace(username)
	fmt.Printf("Password: ")
    password := string(gopass.GetPasswdMasked())

    passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), 0)
    if err != nil {
    	panic(err)
    }

    usersMap[username] = passwordHash

    encodeUserFile(path, usersMap)
}

func decodeUserFile(path string) (map[string][]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := gob.NewDecoder(file)

	var users = make(map[string][]byte)

	if err := decoder.Decode(&users); err != nil {
		return nil, err
	}
	
	return users, nil
}

func encodeUserFile(path string, users map[string][]byte) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)

	if err := encoder.Encode(users); err != nil {
		return err
	}

	return nil
}

func requiresAuth(w http.ResponseWriter, r *http.Request, users map[string][]byte) bool {
	authHeader := r.Header.Get("Authorization");

	authHeaderParts := strings.SplitN(authHeader, " ", 2)
	if len(authHeaderParts) != 2 || authHeaderParts[0] != "Basic" {
		sendAuthHeaders(w)
		return true
	}

	decoded, err := base64.StdEncoding.DecodeString(authHeaderParts[1])
	if err != nil {
		sendAuthHeaders(w)
		return true
	}

	credentials := strings.SplitN(string(decoded), ":", 2)
	if len(credentials) != 2 {
		sendAuthHeaders(w)
		return true
	}

	username := credentials[0]
	password := credentials[1]
	storedHash := users[username]

	if err := bcrypt.CompareHashAndPassword(storedHash, []byte(password)); err != nil {
		sendAuthHeaders(w)
		return true
	}
	
	return false;
}

func sendAuthHeaders(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="editdocs"`)
	w.WriteHeader(http.StatusUnauthorized)
}

func blockedFileExt(w http.ResponseWriter, whiteList map[string]struct{}, path string) bool {
	if _, ok := whiteList[filepath.Ext(path)]; !ok {
		w.WriteHeader(http.StatusNotImplemented)
		return true
	}
	return false
}