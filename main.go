package main

import (
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func viewPdfHandler(w http.ResponseWriter, r *http.Request) {
	const errorHtml = `<!DOCTYPE html>
	<html>
	<head>
	<title>Something went wrong</title>
	</head>
	<body>
		<h1>Error</h1>
		<p>{{ .Message }}</p>
	</body>
	</html>
	`
	errorTempl, err := template.New("general-error").Parse(errorHtml)
	if err != nil {
		log.Printf("ERROR: Failed to parse error template: %s", err.Error())
		http.Error(w, "Something went wrong, try again.", http.StatusInternalServerError)
		return
	}

	fileURL := r.URL.Query().Get("url")
	w.Header().Set("Content-Type", "text/html")
	if fileURL == "" {
		log.Print("ERROR: Failed to parse pdf url, url is empty")
		errorTempl.Execute(w, struct{ Message string }{"Missing 'url' query parameter"})
		return
	}

	parsedURL, err := url.Parse(fileURL)
	if err != nil || !parsedURL.IsAbs() {
		log.Printf("ERROR: Failed to parse pdf url, url is invalid: '%s', error: %v", fileURL, err)
		errorTempl.Execute(w, struct{ Message string }{"Invalid url"})
		return
	}

	pdfViewerHtml := `<!DOCTYPE html>
    <html>
    <head>
	    <title>PDF Viewer</title>
	    <script src="https://cdnjs.cloudflare.com/ajax/libs/jquery/3.6.0/jquery.min.js"></script>
	    <script src="https://cdn.jsdelivr.net/gh/dealfonso/pdfjs-viewer@2.0/dist/pdfjs-viewer.min.js"></script>
	    <link rel="stylesheet" href="https://cdn.jsdelivr.net/gh/dealfonso/pdfjs-viewer@2.0/dist/pdfjs-viewer.min.css">
		<style>
		body {
		margin: 0;
		}
		</style>
    </head>
    <body>
	    <script src="https://cdnjs.cloudflare.com/ajax/libs/pdf.js/3.11.174/pdf.min.js"></script>
	    <script>
	    var pdfjsLib = window['pdfjs-dist/build/pdf'];
	    pdfjsLib.GlobalWorkerOptions.workerSrc = 'https://cdnjs.cloudflare.com/ajax/libs/pdf.js/3.11.174/pdf.worker.min.js';
	    </script>
	    <div class="pdfjs-viewer" pdf-document="/proxy?url={{.FileURL}}" initial-zoom="fit">
	    </div>
    </body>
    </html>
	`
	pdfViewerTempl, err := template.New("pdf-viewer").Parse(pdfViewerHtml)
	if err != nil {
		log.Printf("ERROR: Failed to parse pdf template for url: '%s', error: %s", fileURL, err.Error())
		errorTempl.Execute(w, struct{ Message string }{"Something went wrong, try again."})
		return
	}

	pdfViewerTempl.Execute(w, struct{ FileURL string }{parsedURL.String()})
}

func proxyFileHandler(w http.ResponseWriter, r *http.Request) {
	fileURL := r.URL.Query().Get("url")
	if fileURL == "" {
		log.Print("ERROR: Failed to parse pdf url, url is empty")
		http.Error(w, "Missing 'url' query parameter", http.StatusBadRequest)
		return
	}

	parsedURL, err := url.Parse(fileURL)
	if err != nil || !parsedURL.IsAbs() {
		log.Printf("ERROR: Failed to parse pdf url, url is invalid: '%s', error: %v", fileURL, err)
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	res, err := http.Get(fileURL)
	if err != nil {
		log.Printf("ERROR: Failed to tetch the file for url '%s': %s", parsedURL.String(), err.Error())
		http.Error(w, "Failed to fetch the file: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		log.Printf("ERROR: Failed to fetch the file: %s, status: %s", parsedURL.String(), res.Status)
		http.Error(w, "Failed to fetch the file: "+res.Status, http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", res.Header.Get("Content-Type"))
	w.Header().Set("Content-Disposition", "inline")
	w.WriteHeader(http.StatusOK)

	_, err = io.Copy(w, res.Body)
	if err != nil {
		log.Printf("ERROR: Error streaming file: %v", err)
	}
}

func loggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		query := strings.Builder{}
		for k, v := range r.URL.Query() {
			query.WriteString(fmt.Sprintf("%s %v", k, v))
		}

		log.Printf("Started %s %s | query: %s", r.Method, r.URL.Path, query.String())

		next.ServeHTTP(w, r)

		log.Printf("Completed %s %s | query: %s in %v", r.Method, r.URL.Path, query.String(), time.Since(startTime))
	})
}

func main() {
	port := flag.String("port", "", "The port which the server will listen to. e.g: 8080")
	flag.Parse()
	if port == nil || *port == "" {
		*port = os.Getenv("PORT")
	}
	if port == nil || *port == "" {
		*port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", viewPdfHandler)
	mux.HandleFunc("/proxy", proxyFileHandler)

	loggerMux := loggerMiddleware(mux)
	log.Printf("Starting server on :%s", *port)
	log.Fatal(http.ListenAndServe(":"+*port, loggerMux))
}
