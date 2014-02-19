package storytime

import (
	"fmt"
	"html/template"
	"net/http"
	"reflect"
)

type templateResponse struct {
	name string
	data interface{}
}

func (r templateResponse) Write(w http.ResponseWriter) {
	err := tmpl.ExecuteTemplate(w, r.name, r.data)
	if err != nil {
		panic(&appError{err, "Failed to render template", http.StatusInternalServerError})
	}
}

func execute(data interface{}) response {
	typ := reflect.TypeOf(data)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	field, found := typ.FieldByName("TemplateName")
	if !found {
		panic(fmt.Errorf("Invalid data type for template: %v", data))
	}
	name := field.Tag.Get("template")
	if name == "" {
		panic(fmt.Errorf("Invalid data type for template: %v", data))
	}
	return templateResponse{name, data}
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

type completedTemplate struct {
	Stories []Story
	// TODO(sdh): pagination
}

type rootTemplate struct {
	TemplateName     interface{} `template:"root"`
	LoginLink        string
	User             string
	CompletedStories *completedTemplate
	CurrentStory     *Story
}
