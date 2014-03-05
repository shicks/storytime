package storytime

import (
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
)

func init() {
	http.Handle("/", appHandler(root))
	http.Handle("/begin", appHandler(begin))
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
	if r.req.URL.String() != "/" {
		return notFound
	}
	t := &rootPage{}
	completed := completedStories(r.ctx())
	t.CompletedStories = completed
	if u, url := r.user(); u == nil {
		t.LoginLink = url
	} else {
		t.User = u.Email
		t.CurrentPart = currentStoryPart(r.ctx(), *u)
	}
	return execute(t)
}

// Begins a new story with the given form inputs (authors, words)
func begin(r request) response {
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
	curStory, curPart := currentPart(r.ctx(), id)
	if curStory == nil {
		return errorResponse{404, "Not Found: no such id"} // notFound
	}

	// If the story is complete, display it.
	if curStory.Complete {
		return displayStory(r, *curStory)
	}

	// Otherwise, if the current user is the next author, then show continue page
	u, _ := r.user()
	if u != nil {
		if curPart.NextAuthor == u.Email {
			return redirect("/story/" + id + "/" + curPart.Id)
		}
		// Otherwise, if the current user is an author, display the status
		for _, a := range curStory.Authors {
			if a == u.Email {
				return storyStatus(r, *curStory, *curPart)
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
	return writePart(r, (*args)["storyId"], (*args)["partId"])
}

func continueStory(r request, storyId, partId string) response {
	story, part := currentPart(r.ctx(), storyId)
	if story == nil {
		return errorResponse{404, "Not Found: no such story"}
	} else if part.Id != partId {
		return errorResponse{404, "Not Found: wrong part: " + part.Id}
	}
	return execute(&continuePage{part})
}

func writePart(r request, storyId, partId string) response {
	story, part := currentPart(r.ctx(), storyId)
	if story == nil {
		return errorResponse{404, "Not Found: no such story"}
	} else if part.Id != partId {
		return errorResponse{404, "Not Found: wrong part: " + part.Id}
	}

	return redirect("/")
}

func displayStory(r request, story Story) response {
	return errorResponse{500, fmt.Sprintf("display: %v", story)}
}

func storyStatus(r request, story Story, part StoryPart) response {
	return errorResponse{500, fmt.Sprintf("status: %v", story)}
}
