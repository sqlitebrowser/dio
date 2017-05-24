package main

import (
	"bytes"
	"log"
	"net/http"

	rest "github.com/emicklei/go-restful"
)

func main() {
	ws := new(rest.WebService)
	ws.Filter(rest.NoBrowserCacheFilter)
	ws.Route(ws.PUT("/db_upload").To(dbUpload))
	ws.Route(ws.GET("/db_download").To(dbDownload))
	ws.Route(ws.GET("/db_list").To(dbList))
	rest.Add(ws)
	http.ListenAndServe(":8080", nil)
}

// Upload a database
// Can be tested with: curl -T a.db -H "Name: a.db" -w \%{response_code} -D headers.out http://localhost:8080/db_upload
func dbUpload(r *rest.Request, w *rest.Response) {
	dbName := r.Request.Header.Get("Name")

	var buf bytes.Buffer
	buf.ReadFrom(r.Request.Body)
	log.Printf("Database uploaded.  Name: '%s', size: %d bytes\n", dbName, buf.Len())

	// Send a 201 "Created" response, along with the location of the URL for working with the (new) database
	w.AddHeader("Location", "/"+dbName)
	w.WriteHeader(http.StatusCreated)
}

// Download a database
func dbDownload(r *rest.Request, w *rest.Response) {
	log.Println("dbDownload() called")
}

// Get a list of databases
func dbList(r *rest.Request, w *rest.Response) {
	log.Println("dbList() called")
}
