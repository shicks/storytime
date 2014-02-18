package storytime

import (
	"html/template"
	"net/http"
)

func execute(w http.ResponseWriter, name string, data interface{}) *appError {
	err := tmpl.ExecuteTemplate(w, name, data)
	if err != nil {
		return &appError{err, "Failed to render template", http.StatusInternalServerError}
	}
	return nil
}

var tmpl = template.Must(template.New("template").
	Funcs(fmap).
	ParseFiles("src/github.com/shicks/storytime/template.html"))

var fmap = template.FuncMap{
	"last":  last,
	"first": first,
}

func last(slice []interface{}) interface{} {
	return slice[len(slice)-1]
}

func first(slice []interface{}) interface{} {
	return slice[0]
}

type completedParams struct {
	Stories []Story
	// TODO(sdh): pagination
}

type rootParams struct {
	LoginLink        string
	User             string
	CompletedStories *completedParams
	CurrentStory     *Story
}
