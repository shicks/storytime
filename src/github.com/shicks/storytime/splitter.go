package storytime

// TODO(sdh): consider splitting (get it?) this out into a separate package

import (
	"strings"
)

type Splitter struct {
	delim func(string) (int, int)
	emit  func(chan<- string, string)
}

func SplitterOn(delim string) Splitter {
	return Splitter{
		delim: func(s string) (int, int) {
			return strings.Index(s, delim), len(delim)
		},
		emit: func(c chan<- string, s string) {
			c <- s
		},
	}
}

func SplitterOnAny(chars string) Splitter {
	return Splitter{
		delim: func(s string) (int, int) {
			return strings.IndexAny(s, chars), 1
		},
		emit: func(c chan<- string, s string) {
			c <- s
		},
	}
}

func (s Splitter) Split(str string) <-chan string {
	c := make(chan string)
	go func() {
		i, l := s.delim(str)
		for i >= 0 {
			s.emit(c, str[:i])
			str = str[i+l:]
			i, l = s.delim(str)
		}
		s.emit(c, str)
		close(c)
	}()
	return c
}

func (s Splitter) SplitToList(str string) []string {
	result := make([]string, 0)
	c := s.Split(str)
	for s := range c {
		result = append(result, s)
	}
	return result
}

func (s Splitter) TrimResults() Splitter {
	return Splitter{
		delim: s.delim,
		emit: func(c chan<- string, str string) {
			s.emit(c, strings.Trim(str, " "))
		},
	}
}

func (s Splitter) OmitEmpty() Splitter {
	return Splitter{
		delim: s.delim,
		emit: func(c chan<- string, str string) {
			if str != "" {
				s.emit(c, str)
			}
		},
	}
}
