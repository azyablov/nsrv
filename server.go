package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/azyablov/nsrv/urllist"
)

type Page struct {
	Title string
	Icon  string
	Data  []string
}

const (
	PathRoot    = "/"
	PathDURLApp = "durl"
	PathSave    = "save"
	PathEdit    = "edit"
	PathView    = "view"
)

type AppParams struct {
	Port       int    `json:"port,omitempty"`
	Icon       string `json:"icon,omitempty"`
	UrlListDir string `json:"urlListDir"`
}

type AppConfig struct {
	Params AppParams `json:"params"`
}

// Embedding
// Templates
//go:embed all:tmpl
var emFSTmpl embed.FS

// Static content
//go:embed index.html style.css
var emFSStatic embed.FS

// Static path validation 0 - whole path, 1 - actions, 2 - dynURL for /view
var pathREDURLAppBase = fmt.Sprintf(`^/%s(/)?$`, PathDURLApp)
var pathREDURLAppLong = fmt.Sprintf(`^/%s/(%s|%s|%s)/?([a-zA-Z0-9_\-\.]*)$`, PathDURLApp, PathSave, PathEdit, PathView)

//var pathRegexp = fmt.Sprintf(`^/%s/$`, PathDURLApp)
var validPathBase = regexp.MustCompile(pathREDURLAppBase)
var validPathLong = regexp.MustCompile(pathREDURLAppLong)

//appParam map[string]string
var dynURLDir = "./dyn-url-filtering"
var ac = new(AppConfig)

func main() {
	// Parsing arguments
	var configFile string
	flag.StringVar(&configFile, "c", "nsrv.json", "application config file.")
	flag.Parse()

	// Reading config file
	err := openJSONConfig(ac, configFile)
	if err != nil {
		log.Fatalln(err)
	}
	if len(ac.Params.UrlListDir) == 0 {
		log.Printf("Null UrlListDir value. Switching to default one: %s", dynURLDir)
	} else {
		dynURLDir = ac.Params.UrlListDir
	}

	if ac.Params.Port == 0 {
		log.Fatal("Port value can't be equal to zero!")
	}
	// Starting...
	log.Printf("Starting server at port %s\n", strconv.Itoa(ac.Params.Port))

	srv := &http.Server{
		Addr:           fmt.Sprintf(":%s", strconv.Itoa(ac.Params.Port)),
		Handler:        nil,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// Setting up handlers
	http.HandleFunc("/durl/", durlDeMUX())
	fileServer := http.FileServer(http.FS(emFSStatic))
	http.Handle("/", fileServer)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}

}

func durlDeMUX() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Logging a new request
		log.Printf("Demuxing new request from %s: %s", r.RemoteAddr, r.URL.Path)
		// Checking static path first
		var m []string
		mBase := validPathBase.FindStringSubmatch(r.URL.Path)
		if mBase == nil {
			mLong := validPathLong.FindStringSubmatch(r.URL.Path)
			log.Println("Path: ", mLong)
			if mLong == nil {
				http.NotFound(w, r)
				return
			}
			m = mLong
		} else {
			rootHander(w, r)
			return
		}

		// If /view path, then check dyn URL
		switch m[1] {
		case PathView:
			if m[2] == "" {
				http.Error(w, "Please specify dynamic URL to view.", http.StatusForbidden)
			}
			// Building list of .lst files on the fly
			var pathOk bool
			uRLFiles, err := urllist.GetURLList(dynURLDir)
			if err != nil {
				log.Println("Can't retrieve dynamic URL list.\n", err)
			}

			for _, f := range uRLFiles {
				if m[2] == f {
					pathOk = true
				}
			}
			// Unexpected path received or URL list is not a available via specified server path.
			if !pathOk {
				http.Error(w, "Unexpected path received or URL list is not a available via specified server path.", http.StatusNotFound)
				return
			}
			// Calling /view handler
			viewHander(w, r, m[2])
		case PathSave:
			saveHanlder(w, r)
		case PathEdit:
			editHandler(w, r)
		default:
			http.Error(w, "Unexpected application path.", http.StatusInternalServerError)
		}

	}

}

func editHandler(w http.ResponseWriter, r *http.Request) {
	// Checking HTTP method
	if r.Method != "GET" {
		http.Error(w, "Method is not accepted.", http.StatusForbidden)
		return
	}

	Data := &Page{
		Icon:  html.EscapeString(ac.Params.Icon),
		Title: "URL submission",
		Data:  []string{},
	}

	// Retrieving a list of .lst files for dynamic URL filtering
	uRLFiles, err := urllist.GetURLList(dynURLDir)
	if err != nil {
		log.Printf("Can't retrieve dynamic URL list.\n%s", err)
		http.Error(w, "Can't retrieve dynamic URL list.", http.StatusInternalServerError)
	} else {
		Data.Data = uRLFiles
	}

	renderTemplate(w, strings.Join([]string{"tmpl/", PathEdit, ".tmpl"}, ""), Data)
}

func saveHanlder(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {

		if err := r.ParseForm(); err != nil {
			fmt.Fprintf(w, "ParseForm() err: %v", err)
			return
		}
		uRLlist := r.FormValue("URLlist")
		newURL := r.FormValue("newURL")

		fmt.Fprintf(w, "Test: URLlist = %s\n", uRLlist)
		fmt.Fprintf(w, "Test: newURL = %s\n", newURL)

		err := urllist.UpdateURLList(dynURLDir, uRLlist, newURL)
		if err != nil {
			fmt.Fprintf(w, "Can't update URL list %v w/ %v! %v", uRLlist, newURL, err)
		} else {
			fmt.Fprintf(w, "Modification request was successfully executed.")
		}

	} else {
		http.Error(w, "Unexpected path is provided or URL list is no longer available for modification.", http.StatusNotFound)
	}
}

func viewHander(w http.ResponseWriter, r *http.Request, urlList string) {
	// Checking HTTP method
	if r.Method != "GET" {
		http.Error(w, "Method is not accepted.", http.StatusForbidden)
		return
	}

	urls, err := urllist.GetURLContents(dynURLDir, urlList)
	if err != nil {
		http.Error(w, fmt.Sprintf("Can't retrieve URL list contents.\n %s", err), http.StatusInternalServerError)
	}

	Data := &Page{
		Icon:  html.EscapeString(ac.Params.Icon),
		Title: fmt.Sprintf("URL list %s", urlList),
		Data:  urls,
	}

	renderTemplate(w, strings.Join([]string{"tmpl/", PathView, ".tmpl"}, ""), Data)
}

func rootHander(w http.ResponseWriter, r *http.Request) {
	// Checking HTTP method
	if r.Method != "GET" {
		http.Error(w, "Method is not accepted.", http.StatusForbidden)
		return
	}

	// Building list of .lst files on the fly
	uRLFiles, err := urllist.GetURLList(dynURLDir)
	if err != nil {
		log.Println("Can't retrieve dynamic URL list.\n", err)
	}

	// Creating [] for list of paths
	var verPathList []string
	for _, f := range uRLFiles {
		verPathList = append(verPathList, strings.Join([]string{"/durl/view", f}, "/"))
	}

	Data := &Page{
		Icon:  html.EscapeString(ac.Params.Icon),
		Title: "Dynamic URL lists",
		Data:  verPathList,
	}

	renderTemplate(w, strings.Join([]string{"tmpl/", PathDURLApp, ".tmpl"}, ""), Data)

}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	var tmplFS fs.FS = emFSTmpl
	t, err := template.ParseFS(tmplFS, tmpl)

	if err != nil {
		log.Println(err)
	}

	err = t.Execute(w, p)
	if err != nil {
		log.Println(err)
	}
}

func openJSONConfig(ac *AppConfig, fileName string) error {

	// Opening file
	fh, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("openJSONConfig: can't open specifies file %v: %v", fileName, err)
	}
	defer fh.Close()

	// Reading contents
	fileStream, err := ioutil.ReadAll(fh)
	if err != nil {
		return fmt.Errorf("openJSONConfig: can't read contents of %v: %v", fh.Name(), err)
	}

	err = json.Unmarshal(fileStream, ac)
	if err != nil {
		return fmt.Errorf("openJSONConfig: can't unmarshall file %v: %v", fh.Name(), err)
	}

	return nil
}
