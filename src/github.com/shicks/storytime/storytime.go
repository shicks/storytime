package storytime

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/mail"
	"path"
	"strconv"
	"strings"

	"appengine"
	"appengine/user"
)

func init() {
	http.HandleFunc("/hello", hello)
	http.Handle("/", appHandler(root))
	http.Handle("/begin", appHandler(begin))
	http.Handle("/story/", appHandler(story))
	http.HandleFunc("/sign", sign)
	// TODO(sdh): remove this handler in prod
	// TODO(sdh): UNRELATED - ProdRequiredFlag<T> only has default value in local testing
	//   (or potentially some sort of guice module that checks annotated flags only in prod)
	http.Handle("/clear", appHandler(clearAll))
}

func clearAll(w http.ResponseWriter, r *http.Request) *appError {
	c := appengine.NewContext(r)
	clearDatastore(c)
	return nil
}

func root(w http.ResponseWriter, r *http.Request) *appError {
	c := appengine.NewContext(r)
	u := user.Current(c)
	p := &rootParams{}
	completed := completedStories(c)
	p.CompletedStories = completed
	if u == nil {
		// Not logged in, so display the login link.
		url, err := user.LoginURL(c, r.URL.String())
		if err != nil {
			return &appError{err, "Failed to get login URL", http.StatusInternalServerError}
		}
		p.LoginLink = url
	} else {
		p.User = u.Email
		p.CurrentStory = currentStory(c, *u)
	}
	return execute(w, "root", p)
}

// Begins a new story with the given form inputs (authors, words)
func begin(w http.ResponseWriter, r *http.Request) *appError {
	authorList := strings.Replace(r.FormValue("authors"), "\n", ",", -1)
	authorList = strings.Replace(authorList, "\r", "", -1)
	authors, err := mail.ParseAddressList(authorList)
	if err != nil {
		return &appError{err, "Could not parse author email addresses: " + authorList, http.StatusBadRequest}
	}
	if len(authors) == 0 {
		return &appError{errors.New("No authors"), "No authors", http.StatusBadRequest}
	}
	words, err := strconv.ParseUint(r.FormValue("words"), 10, 16)
	if err != nil {
		return &appError{err, "Could not parse word count as an integer", http.StatusBadRequest}
	}
	c := appengine.NewContext(r)
	id := newStory(c, authors, int(words))
	// Now issue the redirect.
	http.Redirect(w, r, "/story/"+id, http.StatusFound)
	return nil
}

// Handles URLs of the form /story/storyID or /story/storyID/partID
// If the ID is complete, displays the story.
// If it's in progress and the logged-in user is an author
// then it either allows continuing (via a redirect) or
// else shows the status (who we're waiting on).
func story(w http.ResponseWriter, r *http.Request) *appError {
	// First parse the URL
	dir, id := path.Split(r.URL.Path)
	dir2, id2 := path.Split(dir)
	if dir2 == "/story/" && id2 != "" { // path = "/story/<story-id>/<part-id>"
		return continueStory(id2, id, w, r)
	} else if dir != "/story/" { // bad path, i.e. more than 3 components
		return err(fmt.Sprintf("Not Found: too many path components: %v | %v | %v | %v", dir, id, dir2, id2))
		//http.NotFound(w, r)
		//return nil
	}
	// Now look up the story and the last part to find out
	// where it's at.
	c := appengine.NewContext(r)
	story, part := currentPart(c, id)

	// If the story is complete, display it.
	if story.Complete {
		return displayStory(*story, w, r)
	}

	// Otherwise, if the current user is the next author, then show continue page
	u := user.Current(c)
	if part.NextAuthor == u.Email {
		http.Redirect(w, r, "/story/"+id+"/"+part.Id, http.StatusFound)
		return nil
	} else {
		var s = "Not the next author: " + u.Email + " != " + part.NextAuthor
		return &appError{errors.New(s), s, http.StatusBadRequest}
	}

	// Otherwise, if the current user is an author, display the status
	for _, a := range story.Authors {
		if a == u.Email {
			return storyStatus(*story, *part, w, r)
		}
	}
	return err("Not Found: permissions")
	//http.NotFound(w, r)
	//return nil
}

func continueStory(storyId, partId string, w http.ResponseWriter, r *http.Request) *appError {
	return err("continue: " + storyId + " / " + partId)
}

func displayStory(story Story, w http.ResponseWriter, r *http.Request) *appError {
	return nil
}

func storyStatus(story Story, part StoryPart, w http.ResponseWriter, r *http.Request) *appError {
	return nil
}

func sign(w http.ResponseWriter, r *http.Request) {
	err := signTemplate.Execute(w, r.FormValue("contnt"))
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
