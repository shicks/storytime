package storytime

import (
	"errors"
	"fmt"
	"net/http"
	"path"

	"appengine"
)

// Attempts to match a path with a given pattern, of the form
// "/foo/:bar/*", where :bar and * are wildcards.  Colon-prefixed
// wildcards will match anything
func matchPath(arg, pattern string) *map[string]string {
	arg = path.Clean(arg)
	components := strings.Split(pattern, "/")

}

type Action struct {
	RequestPath string
	Handler     interface{}
}

type Module func(Binder)

type Injector interface {
	Inject(Key) interface{}
}

type Key string

func KeyFromInstance(interface{}) Key {
	return ""
}

type Binder interface {
	Bind(Key) BindingBuilder
	BindProvider(interface{})
}

type BindingBuilder interface {
	ToInstance(interface{})
	ToKey(Key)
}

// static const fields in modules to facilitate injecting?

func AppEngineModule(b Binder) {
	b.BindProvider(func(r *http.Request) appengine.Context {
		return appengine.NewContext(r)
	})
}

func GetContext(Injector i) appengine.Context {
	return i.Inject("appengine.Context")
}

type Response interface {
	WriteResponse(http.ResponseWriter)
}

const serveFoo = Action{
	"/bar/:baz/*",
	func(c appengine.Context) {

	},
}
