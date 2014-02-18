package storytime

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	//"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"appengine"
	"appengine/datastore"
	"appengine/user"
)

type hasId interface {
	SetId(string)
	GetId() string
}

func currentStory(c appengine.Context, u user.User) *Story {
	q := datastore.NewQuery("Story").
		Filter("NextAuthor =", u.Email).
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
	}
	if len(part) > 0 {
		var story = new(Story)
		if err := datastore.Get(c, k, story); err != nil {
			panic(&appError{err, "Failed to fetch story", 500})
		}
		return story, &part[0]
	}
	return nil, nil
}

// TODO(sdh): allow logged-in (or via email) users to set their name
//    - alternately, take it from the "From" line?
//    - memcache for caching email-to-name?
//    - how to inject the cache? (may not need to - just use context)
// TODO(sdh): support pagination and per-user?
// TODO(sdh): search service for fulltext story search
func completedStories(c appengine.Context) *completedParams {
	q := datastore.NewQuery("Story").
		Filter("Complete =", true).
		Order("-Finished").
		Limit(10)
	var stories []Story
	if _, err := q.GetAll(c, &stories); err != nil {
		panic(err)
	}
	return &completedParams{stories}
}

func randomString(l int) string {
	b := make([]byte, 2*l)
	rand.Read(b)
	s := base64.StdEncoding.EncodeToString(b)
	s = strings.Replace(s, "+", "", -1)
	s = strings.Replace(s, "/", "", -1)
	s = strings.Replace(s, "=", "", -1)
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
		Author:     "",
		NextAuthor: addrs[0],
	}
	_, err = putShortKey(c, "StoryPart", part, key, 8)
	if err != nil {
		panic(&appError{err, "Failed to put first part in datastore", http.StatusInternalServerError})
	}
	return key.StringID()
}

type Story struct {
	// The ID of this story.
	Id string
	// The time the story was created.
	Created time.Time
	// Email address that created the story.
	Creator string
	// ID required for writing the next part of the story.
	// This will be empty if the story is complete.
	//NextId string
	// Email address of the next author.
	//NextAuthor string
	// Timestamp this story was finished.
	Finished time.Time
	// Whether the story is complete.
	Complete bool
	// The parts of the story, filled in upon completion.
	Parts []StoryPart
	// Email addresses of each author.
	Authors []string
	// Total number of words in the story.  Once the story
	// reaches this length (or longer), it will be closed.
	Words int
}

func (s *Story) SetId(id string) {
	s.Id = id
}

func (s Story) GetId() string {
	return s.Id
}

// Can we store the Parts as a separate kind whose
// parents are the story?  Will we be able to make all
// the queries we need for most recent modified,
// next author, etc?

// queries:
//  1. given story id & part, is it the last? (generate an empty part first) -> easy keysonly
//  2. next part for given author (across stories) -> find parts, order by time
// complete stories can be rewritten w/ all its parts?
// issue: what data structure to return from queries if story doesn't contain parts?
//   - (Story, []StoryPart) ?

type StoryPart struct {
	// The ID of this part.
	Id string
	// ID of the story this part belongs to.
	Story string
	// Text that the next writer does not get to see.
	Hidden string
	// Text that the next writer does get to see.
	Visible string
	// The time that this part was written.
	Written time.Time
	// Author that contributed this part.
	Author string
	// Author that comes next.  Blank if this is the last part.
	NextAuthor string
}

func (s *StoryPart) SetId(id string) {
	s.Id = id
}

func (s StoryPart) GetId() string {
	return s.Id
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
