package storytime

import (
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
	// field, found := typ.FieldByName("TemplateName")
	// if !found {
	// 	panic(fmt.Errorf("Invalid data type for template: %v", data))
	// }
	// name := field.Tag.Get("template")
	// if name == "" {
	// 	panic(fmt.Errorf("Invalid data type for template: %v", data))
	// }
	return templateResponse{typ.Name(), data}
}

var tmpl = template.Must(template.New("template").
	Funcs(fmap).
	ParseFiles("src/github.com/shicks/storytime/template.html"))

var fmap = template.FuncMap{
	"fuzzy": fuzzyTime,
	"last":  lastStory,
	"inc":   func(i int) int { return i + 1 },
}

func lastStory(stories []Story) *Story {
	if len(stories) > 0 {
		return &stories[len(stories)-1]
	}
	return nil
}

// TODO(sdh): Rather than displaying everything on the start page,
// we should split it up.  This will also help with refreshes (i.e.
// when redirecting back to the same page, the initial view is stale).
// Instead, upon navigating to /, redirect to /story/1/foo if there
// is work to do, and provide links at the bottom/top for 'begin'
// or 'completed', etc.  If no story is ongoing, redirect to recently
// completed stories (with snippets of the first few lines).

type continuePage struct {
	CurrentStory *Story
}

type completedPage struct {
	Stories []Story
	// TODO(sdh): pagination
}

type printStoryPage struct {
	Story Story
}

type beginPage struct {
	LoginLink string
	User      string
}

type rootPage struct {
	LoginLink         string
	Author            string
	CurrentStory      *Story
	InProgress        []InProgressStory
	RecentlyCompleted []Story
}

type statusPage struct {
	Story InProgressStory
}
