package storytime

import (
	"strings"
	"time"
)

type hasId interface {
	SetId(string)
	GetId() string
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
	NextId string
	// Email address of the next author.
	NextAuthor string
	// Timestamp this story was last modified.
	Modified time.Time
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

// Rewrites the authors with real names if available.
func (s *Story) RewriteAuthors(rewriter func(string) string) {
	s.Creator = rewriter(s.Creator)
	s.NextAuthor = rewriter(s.NextAuthor)
	for i, part := range s.Parts {
		part.Author = rewriter(part.Author)
		s.Parts[i] = part
	}
}

// Returns the total number of words in this story, so far.
func (s Story) WordCount() int {
	var count int
	splitter := SplitterOnAny("\n\r ").TrimResults().OmitEmpty()
	for _, p := range s.Parts {
		count += len(splitter.SplitToList(p.Hidden))
		count += len(splitter.SplitToList(p.Visible))
	}
	return count
}

// Returns the number of words left before the story is complete.
func (s Story) WordsLeft() int {
	left := s.Words - s.WordCount()
	if left < 0 {
		return 0
	}
	return left
}

// Returns a 24-word snippet for displaying on the completed stories page.
func (s Story) Snippet() string {
	words := SplitterOnAny("\n\r ").TrimResults().OmitEmpty().SplitToList(s.FullText())
	if len(words) > 24 {
		words = append(words[:24], "...")
	}
	return strings.Join(words, " ")
}

// Returns the full text of a story.
func (s Story) FullText() string {
	var text string
	for _, p := range s.Parts {
		text += (p.Hidden + " " + p.Visible)
	}
	return text
}

// Returns the last part of the story, or nil.
func (s Story) LastPart() *StoryPart {
	if len(s.Parts) == 0 {
		return nil
	}
	return &s.Parts[len(s.Parts)-1]
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
	// Text that the next writer does not get to see.
	Hidden string
	// Text that the next writer does get to see.
	Visible string
	// The time that this part was written.
	Written time.Time
	// Author that contributed this part.
	Author string
}

func (s *StoryPart) SetId(id string) {
	s.Id = id
}

func (s StoryPart) GetId() string {
	return s.Id
}
