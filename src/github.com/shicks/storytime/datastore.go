package storytime

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/mail"
	"sort"
	"strings"
	"time"

	"appengine"
	"appengine/datastore"
)

// Retrieves the current story for the given user.
func currentStory(c appengine.Context, author string) *Story {
	q := datastore.NewQuery("Story").
		Filter("Complete =", false).
		Filter("NextAuthor =", author).
		Order("Modified"). // TODO(sdh): what about kicks/skips? - can that mis-sequence?
		Limit(1)           // TODO(sdh): don't limit so we can count?
	var result []Story
	if _, err := q.GetAll(c, &result); err != nil {
		panic(&appError{err, "Failed to fetch current story", 500})
	}
	if len(result) > 0 {
		return &result[0]
	}
	return nil
}

// Retrieves all the in-progress stories for the given author.
func inProgressStories(c appengine.Context, author string) []InProgressStory {

	q := datastore.NewQuery("StoryAuthor").
		Filter("Author =", author).
		KeysOnly()

	keys, err := q.GetAll(c, nil)
	if err != nil {
		panic(&appError{err, "Failed to fetch in-progress story authors", 500})
	}

	storyKeys := make([]*datastore.Key, len(keys))

	for i, key := range keys {
		storyKeys[i] = key.Parent()
	}
	stories := make([]Story, len(keys))
	if err := datastore.GetMulti(c, storyKeys, stories); err != nil {
		panic(&appError{err, "Failed to fetch in-progress stories", 500})
	}
	sort.Sort(byTime(stories))

	inProgress := make([]InProgressStory, len(keys))
	for i, story := range stories {
		inProgress[i] = story.InProgress(author)
		inProgress[i].RewriteAuthors(nameFunc(c))
	}

	return inProgress
}

// ByTime implements sort.Interface for []Story based on the Modified field (descending).
type byTime []Story

func (a byTime) Len() int           { return len(a) }
func (a byTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byTime) Less(i, j int) bool { return a[i].Modified.Before(a[j].Modified) }

// Retrieves a story by ID.
func fetchStory(c appengine.Context, id string) *Story {
	k := datastore.NewKey(c, "Story", id, 0, nil)
	var story = new(Story)
	if err := datastore.Get(c, k, story); err != nil {
		panic(&appError{err, "Failed to fetch story", 500})
	}
	return story
}

// TODO(sdh): allow logged-in (or via email) users to set their name
//    - alternately, take it from the "From" line?
//    - memcache for caching email-to-name?
//    - how to inject the cache? (may not need to - just use context)
// TODO(sdh): support pagination and per-user?
// TODO(sdh): search service for fulltext story search
func completedStories(c appengine.Context, limit int, olderThan time.Time) []Story {
	q := datastore.NewQuery("Story").
		Filter("Complete =", true).
		Order("-Modified").
		Filter("Modified <", olderThan).
		Limit(limit)
	var stories []Story
	if _, err := q.GetAll(c, &stories); err != nil {
		panic(err)
	}
	return stories
}

// Generates a random string of lowercase letters and numbers of the given length.
func randomString(l int) string {
	b := make([]byte, 2*l)
	rand.Read(b)
	s := base64.StdEncoding.EncodeToString(b)
	s = strings.Replace(s, "+", "", -1)
	s = strings.Replace(s, "/", "", -1)
	s = strings.Replace(s, "=", "", -1)
	s = strings.ToLower(s)
	if len(s) > l {
		s = s[:l]
	}
	// TODO(sdh): if the length is too short, add more.
	return s
}

func putShortKey(c appengine.Context, kind string, story *Story, parent *datastore.Key, minLength int) (*datastore.Key, error) {
	// Pick a random ID and then find all 2+ character substrings
	s := randomString(32)
	var result *Story
	var key *datastore.Key
	var e error
	for i := minLength; i < len(s); i++ {
		e = datastore.RunInTransaction(c, func(c appengine.Context) error {
			result = nil
			key = datastore.NewKey(c, kind, s[:i], 0, parent)
			story.Id = s[:i]
			if err := datastore.Get(c, key, result); err != nil && err != datastore.ErrNoSuchEntity {
				return err
			}
			if result != nil {
				i++
				return datastore.ErrConcurrentTransaction
			}
			if _, err := datastore.Put(c, key, story); err != nil {
				return err
			}
			// Also store all the StoryAuthor entities (Note: we could re-abstract this to HasId
			// by adding a method finalize() but it would pull datastore details into story.go
			authorKeys := make([]*datastore.Key, 0, len(story.Authors))
			authorEntities := make([]StoryAuthor, 0, len(story.Authors))
			for _, author := range story.Authors {
				authorKeys = append(authorKeys, datastore.NewKey(c, "StoryAuthor", author, 0, key))
				authorEntities = append(authorEntities, StoryAuthor{author, story.Id})
			}
			if _, err := datastore.PutMulti(c, authorKeys, authorEntities); err != nil {
				return err
			}
			return nil
		}, nil)
		if e == nil {
			return key, nil
		}
	}
	return nil, e
}

// Makes a new story and saves it to the datastore.
// Returns the ID.
func newStory(r request, authors []*mail.Address, words int) Story {
	u, _ := r.user()
	if u == nil {
		panic(fmt.Errorf("Must be logged in to start a new story."))
	}
	addrs := make([]string, len(authors))
	parts := make([]StoryPart, 0)
	found := false
	for i, author := range authors {
		if author.Name != "" {
			putNameForEmailIfAbsent(r.ctx(), author.Name, author.Address)
		}
		addrs[i] = author.Address
		found = found || author.Address == u.Email
	}
	if !found {
		panic(errorResponse{400, "Error: New stories must include yourself as an author."})
	}
	now := time.Now()
	story := &Story{
		Created:    now,
		Creator:    u.Email,
		NextId:     randomString(8),
		NextAuthor: addrs[0],
		Modified:   now,
		Complete:   false,
		Parts:      parts,
		Authors:    addrs,
		Words:      words,
	}
	key, err := putShortKey(r.ctx(), "Story", story, nil, 3)
	if err != nil {
		panic(&appError{err, "Failed to put story in datastore", http.StatusInternalServerError})
	}
	if story.Id != key.StringID() {
		panic(fmt.Errorf("Expected story.Id == key.StringID(): %s vs %s", story.Id, key.StringID()))
	}
	return *story
}

// Returns the next author in the cycle, panics if the
// current author is not found.
func findNextAuthor(authors []string, author string) string {
	for i, a := range authors {
		if a == author {
			return authors[(i+1)%len(authors)]
		}
	}
	panic(fmt.Errorf("Could not find author %s in author list %s", author, authors))
}

// Makes a new story and saves it to the datastore.  Panics in case of
// an error.
func savePart(c appengine.Context, story *Story, text string) {
	maxVisible := 16
	var part StoryPart
	now := time.Now()

	part.Id = story.NextId
	story.NextId = randomString(8)
	part.Author = story.NextAuthor
	story.NextAuthor = findNextAuthor(story.Authors, story.NextAuthor)
	part.Written = now
	story.Modified = now
	// Sanitize the text
	lines := SplitterOnAny("\n\r").TrimResults().OmitEmpty().SplitToList(text)
	if len(lines) < 1 {
		panic(fmt.Errorf("No text: %s", text))
	}
	hidden := strings.Join(lines[:len(lines)-1], " ")
	visible := lines[len(lines)-1]
	// Make sure we don't have too much visible
	wordSplitter := SplitterOn(" ").TrimResults().OmitEmpty()
	words := wordSplitter.SplitToList(visible)
	if len(words) > maxVisible {
		lastWord := len(words) - maxVisible
		hidden = hidden + " " + strings.Join(words[:lastWord], " ")
		visible = strings.Join(words[lastWord:], " ")
	}
	part.Hidden = strings.Join(wordSplitter.SplitToList(hidden), " ")
	part.Visible = visible
	story.Parts = append(story.Parts, part)
	if story.WordCount() >= story.Words {
		story.Complete = true
	}
	e := datastore.RunInTransaction(c, func(c appengine.Context) error {
		existing := new(Story)
		key := datastore.NewKey(c, "Story", story.Id, 0, nil)
		if err := datastore.Get(c, key, existing); err != nil {
			return err
		}
		if existing.NextId != part.Id {
			panic(fmt.Errorf("Part was written concurrently."))
		}
		if _, err := datastore.Put(c, key, story); err != nil {
			return err
		}
		if story.Complete {
			// We need to delete all the author keys
			q := datastore.NewQuery("StoryAuthor").
				Ancestor(key).
				KeysOnly()
			authorKeys, err := q.GetAll(c, nil)
			if err != nil {
				return err
			}
			if err := datastore.DeleteMulti(c, authorKeys); err != nil {
				return err
			}
		}
		return nil
	}, nil)
	if e != nil {
		panic(&appError{e, "Failed to update story", http.StatusInternalServerError})
	}
}

func clearKind(c appengine.Context, kind string) {
	q := datastore.NewQuery(kind).KeysOnly()
	keys, err := q.GetAll(c, nil)
	if err != nil {
		panic(&appError{err, "Failed to fetch all " + kind, 500})
	}
	if err := datastore.DeleteMulti(c, keys); err != nil {
		panic(&appError{err, "Failed to delete all " + kind, 500})
	}
}

func clearDatastore(c appengine.Context) {
	clearKind(c, "Story")
	clearKind(c, "StoryAuthor")
	clearKind(c, "UserInfo")
}

func deleteStoryAuthors(c appengine.Context, id string) {
}
