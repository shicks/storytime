package storytime

import (
	"fmt"
	"html/template"
	"net/http"

	"appengine"
	"appengine/user"
)

func init() {
	http.HandleFunc("/hello", hello)
	http.HandleFunc("/", root)
	http.HandleFunc("/sign", sign)
}

func root(w http.ResponseWriter, r *http.Request) {
	err := rootTmpl.Execute(w, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
var rootTmpl = template.Must(template.ParseFiles("src/github.com/shicks/storytime/root.html"))

func sign(w http.ResponseWriter, r *http.Request) {
	err := signTemplate.Execute(w, r.FormValue("content"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
var signTemplate = template.Must(template.New("sign").Parse(`
<!DOCTYPE html>
<p>You wrote:</p>
<pre>{{.}}</pre>
`))

func hello(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	u := user.Current(c)
	if u == nil {
		url, err := user.LoginURL(c, r.URL.String())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Location", url)
		w.WriteHeader(http.StatusFound)
		return
	}

	fmt.Fprintf(w, "Hello, %v!", u)
}
