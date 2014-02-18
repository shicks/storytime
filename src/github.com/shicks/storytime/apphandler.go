package storytime

import (
	"errors"
	"fmt"
	"net/http"

	"appengine"
)

type appError struct {
	Error   error
	Message string
	Code    int
}

func err(text string) *appError {
	return &appError{errors.New(text), text, http.StatusBadRequest}
}

type appHandler func(http.ResponseWriter, *http.Request) *appError

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if e := recover(); e != nil {
			c := appengine.NewContext(r)
			switch e := e.(type) {
			case appError:
				c.Errorf("%v", e.Error)
				http.Error(w, e.Message, e.Code)
			default:
				c.Errorf("%v", e)
				http.Error(w, fmt.Sprintf("%v", e), http.StatusInternalServerError)
			}
		}
	}()
	if e := fn(w, r); e != nil {
		c := appengine.NewContext(r)
		c.Errorf("%v", e.Error)
		http.Error(w, e.Message, e.Code)
	}
}
