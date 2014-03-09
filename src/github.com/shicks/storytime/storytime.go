package storytime

import (
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

func init() {
	http.Handle("/", appHandler(root))
	http.Handle("/begin", appHandler(begin))
	http.Handle("/completed", appHandler(completed))
	http.Handle("/story/", appHandler(story))
	http.Handle("/write/", appHandler(write))

	// TODO(sdh): remove this handler in prod
	// TODO(sdh): UNRELATED - ProdRequiredFlag<T> only has default value in local testing
	//   (or potentially some sort of guice module that checks annotated flags only in prod)
	http.Handle("/clear", appHandler(clearAll))
}

func clearAll(r request) response {
	clearDatastore(r.ctx())
	return errorResponse{200, "OK"}
}

func root(r request) response {
	// Depending on whether the user is logged in
	// and whether there is an outstanding story,
	// redirect...
	if r.req.URL.String() != "/" {
		return notFound
	}

	if u, _ := r.user(); u != nil {
		story := currentStory(r.ctx(), *u)
		if story != nil {
			return redirect("/story/" + story.Id + "/" + story.NextId)
		}
	}
	return redirect("/completed")
}

func begin(r request) response {
	if r.req.Method == "POST" {
		return beginPost(r)
	}
	t := &beginPage{}
	if u, url := r.user(); u == nil {
		t.LoginLink = url
	} else {
		t.User = u.Email
	}
	return execute(t)
}

// Begins a new story with the given form inputs (authors, words)
func beginPost(r request) response {
	authorList := strings.Replace(r.req.FormValue("authors"), "\n", ",", -1)
	authorList = strings.Replace(authorList, "\r", "", -1)
	authors, err := mail.ParseAddressList(authorList)
	if err != nil {
		panic(&appError{err, "Could not parse author email addresses: " + authorList, http.StatusBadRequest})
	}
	if len(authors) == 0 {
		panic(&appError{errors.New("No authors"), "No authors", http.StatusBadRequest})
	}
	words, err := strconv.ParseUint(r.req.FormValue("words"), 10, 16)
	if err != nil {
		panic(&appError{err, "Could not parse word count as an integer", http.StatusBadRequest})
	}
	id := newStory(r.ctx(), authors, int(words))
	// Now issue the redirect.
	return redirect("/story/" + id)
}

func completed(r request) response {
	return execute(&completedPage{completedStories(r.ctx())})
}

// Handles URLs of the form /story/storyID or /story/storyID/partID
// If the ID is complete, displays the story.
// If it's in progress and the logged-in user is an author
// then it either allows continuing (via a redirect) or
// else shows the status (who we're waiting on).
func story(r request) response {
	// First parse the URL
	args := r.matchPath("/story/:storyId/:partId")
	if args != nil {
		return continueStory(r, (*args)["storyId"], (*args)["partId"])
	}
	if args = r.matchPath("/story/:storyId"); args == nil {
		return errorResponse{404, "Not Found: bad format"} // notFound
	}

	// We're looking at a story, so the behavior depends on the status/user.
	// We need to look up the story and the last part to find out where it's at.
	id := (*args)["storyId"]
	story := fetchStory(r.ctx(), id)
	if story == nil {
		return errorResponse{404, "Not Found: no such id"} // notFound
	}

	// If the story is complete, display it.
	if story.Complete {
		return displayStory(r, *story)
	}

	// Otherwise, if the current user is the next author, then show continue page
	u, _ := r.user()
	if u != nil {
		if story.NextAuthor == u.Email {
			return redirect("/story/" + id + "/" + story.NextId)
		}
		// Otherwise, if the current user is an author, display the status
		for _, a := range story.Authors {
			if a == u.Email {
				return storyStatus(r, *story)
			}
		}
	}

	return errorResponse{404, "Not Found: permissions"} // TODO(sdh): notFound
}

// Handles URLs of the form /write/storyID/partID, reading the post data
// and appending the part.  Redirects to / on success.
func write(r request) response {
	args := r.matchPath("/write/:storyId/:partId")
	if args == nil {
		return errorResponse{404, "Not Found: bad format"} // notFound
	}
	text := r.req.FormValue("content")
	return writePart(r, (*args)["storyId"], (*args)["partId"], text)
}

func continueStory(r request, storyId, partId string) response {
	story := fetchStory(r.ctx(), storyId)
	if story == nil {
		return errorResponse{404, "Not Found: no such story"}
	} else if story.NextId != partId {
		return redirect("/")
		//return errorResponse{404, "Not Found: wrong part: " + story.NextId}
	}
	story.RewriteAuthors(nameFunc(r.ctx()))
	return execute(&continuePage{story})
}

func writePart(r request, storyId, partId, text string) response {
	story := fetchStory(r.ctx(), storyId)
	if story == nil {
		return errorResponse{404, "Not Found: no such story"}
	} else if story.NextId != partId {
		return errorResponse{404, "Not Found: wrong part: " + story.NextId}
	}
	savePart(r.ctx(), story, text)
	time.Sleep(500 * time.Millisecond)
	return redirect("/")
}

func displayStory(r request, story Story) response {
	return errorResponse{500, fmt.Sprintf("display: %v", story)}
}

func storyStatus(r request, story Story) response {
	return errorResponse{500, fmt.Sprintf("status: %v", story)}
}
