package storytime

import (
	"errors"
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
	http.Handle("/clear", appHandler(clearAll))
	http.Handle("/repair", appHandler(repairAll))
}

func clearAll(r request) response {
	u := r.userRequired()
	if !u.Admin {
		return notFound
	}
	clearDatastore(r.ctx())
	flushUserCache(r.ctx())
	return errorResponse{200, "OK"}
}

func repairAll(r request) response {
	u := r.userRequired()
	if !u.Admin {
		return notFound
	}
	//fixStoryAuthors(r.ctx())
	//cleanDatastore(r.ctx())
	return errorResponse{200, "OK"}
}

func root(r request) response {
	// Note: "/" matches everything.
	if r.matchPath("/") == nil {
		return errorResponse{404, "Not Found: " + r.req.URL.String()}
	}

	// Build up the response.
	var root rootPage
	root.RecentlyCompleted = completedStories(r.ctx(), 5, time.Now())
	u, url := r.user()
	if u != nil {
		root.Author = u.Email
		root.CurrentStory = currentStory(r.ctx(), u.Email)
		root.InProgress = inProgressStories(r.ctx(), u.Email)
	} else {
		root.LoginLink = url
	}

	// Return the response.
	return execute(root)
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
	authorList := strings.Join(
		SplitterOnAny(",\n\r").TrimResults().OmitEmpty().SplitToList(r.req.FormValue("authors")), ",")
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
	story := newStory(r, authors, int(words))
	user, _ := r.user()
	if user == nil || story.NextAuthor != user.Email {
		maybeSendMail(r.ctx(), story)
	}
	// Now issue the redirect.
	return redirect("/story/" + story.Id)
}

func completed(r request) response {
	olderThan := time.Now()
	if before := r.req.FormValue("before"); before != "" {
		beforeSeconds, err := strconv.ParseInt(before, 10, 64)
		if err != nil {
			// TODO(sdh): log?
		} else {
			olderThan = time.Unix(beforeSeconds, 0)
		}
	}
	return execute(&completedPage{completedStories(r.ctx(), 50, olderThan)})
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
		// If not, but the current user is an author, display the status
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
		// If this is an out-of-date partId, redirect to the story status
		for _, part := range story.Parts {
			if part.Id == partId {
				return storyStatus(r, story)
			}
		}
		return errorResponse{404, "Not Found: wrong part: " + story.NextId} // notFound
	}
	story.RewriteAuthors(nameFunc(r.ctx()))
	return execute(&continuePage{story})
}

func writePart(r request, storyId, partId, text string) response {
	if len(text) > 500 {
		return errorResponse{400, "Input too long: 500 characters max."}
	}
	story := fetchStory(r.ctx(), storyId)
	user, _ := r.user()
	author := story.NextAuthor
	if story == nil {
		return errorResponse{404, "Not Found: no such story"}
	} else if story.NextId != partId {
		return errorResponse{404, "Not Found: wrong part: " + story.NextId}
	}
	savePart(r.ctx(), story, text)
	time.Sleep(500 * time.Millisecond)
	// If the user is NOT logged in, then we need to send an email with the next part
	// Also, just redirect there.
	if user == nil || author != user.Email {
		nextStory := currentStory(r.ctx(), author)
		if nextStory != nil && nextStory.NextId != partId {
			sendMail(r.ctx(), *nextStory)
			return redirect("/story/" + nextStory.Id + "/" + nextStory.NextId)
		}
	}
	// Also maybe send an email to the next author of this story
	if story.NextAuthor != author {
		maybeSendMail(r.ctx(), *story)
	}
	return redirect("/")
}

func displayStory(r request, story Story) response {
	story.RewriteAuthors(nameFunc(r.ctx()))
	return execute(&printStoryPage{story})
}

func storyStatus(r request, story Story) response {
	inProgress := story.InProgress(r.userRequired().Email)
	inProgress.RewriteAuthors(relativeNameFunc(r.ctx(), r.userRequired().Email))
	return execute(&statusPage{inProgress})
}
