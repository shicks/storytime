package storytime

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"

	"appengine"
	"appengine/user"
)

type appError struct {
	Error   error
	Message string
	Code    int
}

func err(text string) *appError {
	return &appError{errors.New(text), text, http.StatusBadRequest}
}

type request struct {
	req     *http.Request
	reqCtx  *appengine.Context
	reqUser *user.User
}

func (r request) ctx() appengine.Context {
	if r.reqCtx == nil {
		c := appengine.NewContext(r.req)
		r.reqCtx = &c
	}
	return *r.reqCtx
}

func (r request) user() (*user.User, string) {
	if r.reqUser == nil {
		r.reqUser = user.Current(r.ctx())
		if r.reqUser == nil {
			url, err := user.LoginURL(r.ctx(), r.req.URL.String())
			if err != nil {
				panic(err)
			}
			return nil, url
		}
	}
	return r.reqUser, ""
}

func (r request) userRequired() *user.User {
	u, url := r.user()
	if u == nil {
		panic(redirect(url))
	}
	return u
}

// Pattern is a string like "/story/:storyId/:partId"
func (r request) matchPath(pattern string) *map[string]string {
	pattern = path.Clean(pattern)
	url := path.Clean(r.req.URL.String())
	patternSplit := strings.Split(pattern, "/")
	urlSplit := strings.Split(url, "/")
	if len(urlSplit) < len(patternSplit) || (len(urlSplit) > len(patternSplit) && !strings.HasSuffix(url, "/*")) {
		return nil
	}
	result := make(map[string]string)
	for i := 0; i < len(patternSplit); i++ {
		if patternSplit[i] == "*" {
			if i != len(patternSplit)-1 {
				panic(errors.New("Bad pattern"))
			}
			result["*"] = strings.Join(urlSplit[i:], "/")
		} else if strings.HasPrefix(patternSplit[i], ":") {
			result[patternSplit[i][1:]] = urlSplit[i]
		} else if patternSplit[i] != urlSplit[i] {
			return nil
		}
	}
	return &result
}

type response interface {
	Write(http.ResponseWriter)
}

// Response that can be returned (or thrown) to redirect
type redirectResponse struct {
	url  string
	code int
}

func redirect(url string) redirectResponse {
	return redirectResponse{url, http.StatusFound}
}

func (r redirectResponse) Write(w http.ResponseWriter) {
	w.Header().Set("Location", r.url)
	w.WriteHeader(r.code)
}

// Response that returns an error to the user
type errorResponse struct {
	code    int
	message string
}

func (r errorResponse) Write(w http.ResponseWriter) {
	http.Error(w, r.message, r.code)
}

var notFound = errorResponse{404, "Not Found"}

type appHandler func(request) response

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if e := recover(); e != nil {
			// We can use panic to prematurely exit a function
			if resp, ok := e.(response); ok {
				resp.Write(w)
				return
			}
			// More traditional recovery involves some logging
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
	resp := fn(request{req: r})
	resp.Write(w)
}
