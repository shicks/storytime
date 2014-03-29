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

// Returns a snippet of the last piece of a story written by the given author.
func (s Story) InProgressSnippet(author string) string {
	for i := len(s.Parts) - 1; i >= 0; i-- {
		part := s.Parts[i]
		if part.Author == author {
			var prefix string
			if i > 0 {
				prefix = "... "
			}
			return "\"" + strings.Trim(prefix+part.Hidden+" "+part.Visible+" ...", " \r\n") + "\""
		}
	}
	return "(You have not written any parts yet)"
}

// Returns an InProgressStory for the given user.
func (s Story) InProgress(author string) InProgressStory {
	inProgress := InProgressStory{
		Id:          s.Id,
		Created:     s.Created,
		Creator:     s.Creator,
		NextAuthor:  s.NextAuthor,
		Modified:    s.Modified,
		LastWritten: s.InProgressSnippet(author),
		Authors:     s.Authors,
		Words:       s.Words,
		WordsLeft:   s.WordsLeft(),
	}
	if len(s.Parts) > 0 {
		inProgress.LastAuthor = s.Parts[len(s.Parts)-1].Author
	}
	return inProgress
}

// Returns the full text of a story.
func (s Story) FullText() string {
	pieces := make([]string, 0)
	for _, p := range s.Parts {
		pieces = append(append(pieces, p.Hidden), p.Visible)
	}
	return strings.Join(pieces, " ")
}

// Returns the last part of the story, or nil.
func (s Story) LastPart() *StoryPart {
	if len(s.Parts) == 0 {
		return nil
	}
	return &s.Parts[len(s.Parts)-1]
}

// Returns the unix time in seconds this story was last modified.
func (s Story) LastModified() int64 {
	return s.Modified.Unix()
}

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

// This kind is used to quickly access all current stories for a given author.
type StoryAuthor struct {
	// Name of the author.
	Author string
	// Id of the story.
	StoryId string
}

// Summarizes an in-progress story for a given author.
type InProgressStory struct {
	// The ID of this story.
	Id string
	// The time the story was created.
	Created time.Time
	// Email address that created the story.
	Creator string
	// Email address of the next author.
	NextAuthor string
	// Email address of the last author.
	LastAuthor string
	// Timestamp this story was last modified.
	Modified time.Time
	// Last chunk written by the current user.
	LastWritten string
	// Email addresses of each author.
	Authors []string
	// Total number of words in the story.  Once the story
	// reaches this length (or longer), it will be closed.
	Words int
	// Words remaining in the story.
	WordsLeft int
}

// Rewrites the authors with real names if available.
func (s *InProgressStory) RewriteAuthors(rewriter func(string) string) {
	s.Creator = rewriter(s.Creator)
	s.NextAuthor = rewriter(s.NextAuthor)
	s.LastAuthor = rewriter(s.LastAuthor)
	for i, author := range s.Authors {
		s.Authors[i] = rewriter(author)
	}
}
