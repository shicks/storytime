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
	fmt.Fprint(w, guestbookForm)
}

const guestbookForm = `
<!DOCTYPE html>
<form action="/sign" method="post">
<div><textarea name="content" rots="3" cols="60"></textarea></div>
<div><input type="submit" value="Sign"></div>
</form>
`

func sign(w http.ResponseWriter, r *http.Request) {
	err := signTemplate.Execute(w, r.FormValue("content"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var signTemplate = template.Must(template.New("sign").Parse(signTemplateHTML))
const signTemplateHTML = `
<!DOCTYPE html>
<p>You wrote:</p>
<pre>{{.}}</pre>
`

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
