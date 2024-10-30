package server

import (
	"bytes"
	"errors"
	"html/template"

	log "metrics/internal/logger"
)

const getAllHTML = `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <title>Metrics list</title>
  </head>
  <body>
  	<h1>All metrics</h1>
	<ol>{{ range .Data}}
	<li>{{ .Met}}</li>{{ end }}
	</ol>
  </body>
</html>`

type Item struct {
	Met string
}

type templateArgs struct {
	Data []Item
}

func renderGetAll(data []Item) (*bytes.Buffer, error) {
	indexTemplate := template.Must(template.New("metrics").Parse(getAllHTML))
	buf := new(bytes.Buffer)
	err := indexTemplate.Execute(buf, templateArgs{Data: data})
	if err != nil {
		log.Warn("error html template exec")
		return nil, errors.Unwrap(err)
	}

	return buf, nil
}
