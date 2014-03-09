package storytime

import (
	"appengine"
	"appengine/datastore"
	"appengine/memcache"
)

// Conditionally adds a name to the name store (and cache).
// Does nothing if a name is already set for thie email.
func putNameForEmailIfAbsent(c appengine.Context, name, email string) {
	existing := getNameFromEmail(c, email)
	if existing == nil {
		putNameForEmail(c, name, email)
	}
}

// Stores a name for the given email.
func putNameForEmail(c appengine.Context, name, email string) {
	key := datastore.NewKey(c, "UserInfo", email, 0, nil)
	if _, err := datastore.Put(c, key, &UserInfo{email, name}); err != nil {
		return // best effort
	}
	cacheNameForEmail(c, name, email)
}

// Retrieves a name from the store (or cache).  Returns nil if no
// name is set.
func getNameFromEmail(c appengine.Context, email string) *string {
	result, err := memcache.Get(c, "nameforemail:"+email)
	var name string
	if err != nil && err != memcache.ErrCacheMiss {
		panic(err) // who knows what this could be...
	}
	if err == nil {
		name = string(result.Value)
		if name != "" {
			return &name
		}
		return nil
	}
	// Cache miss: go to datastore
	info := new(UserInfo)
	err = datastore.Get(c, datastore.NewKey(c, "UserInfo", email, 0, nil), info)
	if err != nil && err != datastore.ErrNoSuchEntity {
		panic(err) // again, something weird.
	}
	if result != nil {
		name = info.Name
	}
	cacheNameForEmail(c, name, email)
	if name != "" {
		return &name
	}
	return nil
}

func cacheNameForEmail(c appengine.Context, name, email string) {
	memcache.Set(c, &memcache.Item{
		Key:   "nameforemail:" + email,
		Value: []byte(name),
	})
}

func nameFunc(c appengine.Context) func(string) string {
	return func(email string) string {
		name := getNameFromEmail(c, email)
		if name != nil {
			return *name
		}
		return email
	}
}

type UserInfo struct {
	// The user's email address
	Email string
	// The user's preferred name
	Name string
}
