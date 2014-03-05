package storytime

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"appengine"
	"appengine/datastore"
	"appengine/user"
)

// TODO - THIS IS BROKEN
//  - it returns parts that are no longer current...
//  - we need to either (1) re-store the previous part
//    with a "no longer open" flag set, or else (2)
//    update the story with a nextAuthor and modTime
func currentStoryPart(c appengine.Context, u user.User) *StoryPart {
	q := datastore.NewQuery("StoryPart").
		Filter("NextAuthor =", u.Email).
		Order("Written"). // TODO(sdh): what about kicks/skips? - can that mis-sequence?
		Limit(1)          // TODO(sdh): don't limit so we can count?
	var result []StoryPart
	if _, err := q.GetAll(c, &result); err != nil {
		panic(&appError{err, "Failed to fetch current story", 500})
	}
	if len(result) > 0 {
		return &result[0]
	}
	return nil
}

// Retreives the most recent part of the given story.
func currentPart(c appengine.Context, id string) (*Story, *StoryPart) {
	k := datastore.NewKey(c, "Story", id, 0, nil)
	q := datastore.NewQuery("StoryPart").
		Ancestor(k).
		Order("-Written").
		Limit(1)

	var part []StoryPart
	if _, err := q.GetAll(c, &part); err != nil {
		panic(&appError{err, "Failed to fetch current story part", 500})
	} else if len(part) == 0 {
		return nil, nil
	}

	var story = new(Story)
	if err := datastore.Get(c, k, story); err != nil {
		panic(&appError{err, "Failed to fetch story", 500})
	}
	return story, &part[0]
}

// TODO(sdh): allow logged-in (or via email) users to set their name
//    - alternately, take it from the "From" line?
//    - memcache for caching email-to-name?
//    - how to inject the cache? (may not need to - just use context)
// TODO(sdh): support pagination and per-user?
// TODO(sdh): search service for fulltext story search
func completedStories(c appengine.Context) *completedTemplate {
	q := datastore.NewQuery("Story").
		Filter("Complete =", true).
		Order("-Finished").
		Limit(10)
	var stories []Story
	if _, err := q.GetAll(c, &stories); err != nil {
		panic(err)
	}
	return &completedTemplate{stories}
}

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
	return s
}

func putShortKey(c appengine.Context, kind string, data hasId, parent *datastore.Key, minLength int) (*datastore.Key, error) {
	// Pick a random ID and then find all 2+ character substrings
	s := randomString(32)
	var result *Story
	var key *datastore.Key
	var e error
	for i := minLength; i < len(s); i++ {
		e = datastore.RunInTransaction(c, func(c appengine.Context) error {
			result = nil
			key = datastore.NewKey(c, kind, s[:i], 0, parent)
			data.SetId(s[:i])
			if err := datastore.Get(c, key, result); err != nil && err != datastore.ErrNoSuchEntity {
				return err
			}
			if result != nil {
				i++
				return datastore.ErrConcurrentTransaction
			}
			if _, err := datastore.Put(c, key, data); err != nil {
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

func rev(x int64) int64 {
	var y int64
	y = 0
	for i := 0; i < 56; i++ {
		y <<= 1
		y += x % 2
		x >>= 1
	}
	return y
}

func encodeInt(x int64) string {
	// x is a 56-bit int, we want to reverse the bit order
	y := rev(x)
	b := make([]byte, 10)
	l := binary.PutVarint(b, y)
	return base64.URLEncoding.EncodeToString(b[:l])
}

// Makes a new story and saves it to the datastore.
// Returns the ID.
func newStory(c appengine.Context, authors []*mail.Address, words int) string {
	// TODO(sdh): incorporate names from email addresses?
	//id := randomString(10)
	//nextId := randomString(16)
	//key := datastore.NewKey(c, "Story", id, 0, nil)
	//key := datastore.NewIncompleteKey(c, "Story", nil)
	u := user.Current(c)
	addrs := make([]string, len(authors))
	//parts := make([]StoryPart, 0)
	for i, author := range authors {
		addrs[i] = author.Address
	}
	now := time.Now()
	story := &Story{
		Created: now,
		Creator: u.Email,
		//NextId:     nextId,
		//NextAuthor: addrs[0],
		Authors: addrs,
		//Modified: time.Now(),
		//Parts: parts,
		Words: words,
	}
	key, err := putShortKey(c, "Story", story, nil, 3)
	if err != nil {
		panic(&appError{err, "Failed to put story in datastore", http.StatusInternalServerError})
	}
	part := &StoryPart{
		Story:      key.StringID(),
		Hidden:     "",
		Visible:    "",
		Written:    now,
		Author:     u.Email,
		NextAuthor: addrs[0],
	}
	_, err = putShortKey(c, "StoryPart", part, key, 8)
	if err != nil {
		panic(&appError{err, "Failed to put first part in datastore", http.StatusInternalServerError})
	}
	return key.StringID()
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
func savePart(c appengine.Context, story *Story, prev *StoryPart, text string) {
	maxVisible := 16
	var part StoryPart
	part.Story = story.Id
	part.Author = prev.NextAuthor
	part.NextAuthor = findNextAuthor(story.Authors, prev.NextAuthor)
	part.Written = time.Now()
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
	storyKey := datastore.NewKey(c, "Story", story.Id, 0, nil)
	_, err := putShortKey(c, "StoryPart", &part, storyKey, 8)
	if err != nil {
		panic(&appError{err, "Failed to put part in datastore", http.StatusInternalServerError})
	}
}

func clearKind(c appengine.Context, kind string) {
	q := datastore.NewQuery("Story").KeysOnly()
	var keys []*datastore.Key
	if _, err := q.GetAll(c, &keys); err != nil {
		panic(&appError{err, "Failed to fetch all " + kind, 500})
	}
	if err := datastore.DeleteMulti(c, keys); err != nil {
		panic(&appError{err, "Failed to delete all " + kind, 500})
	}
}

func clearDatastore(c appengine.Context) {
	clearKind(c, "Story")
	clearKind(c, "StoryPart")
}
