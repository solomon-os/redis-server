package users

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"slices"
	"sync"
	"sync/atomic"
)

type snapshot struct {
	flags     []string
	passwords [][32]byte
}

type Details struct {
	Flags     []string
	Passwords [][32]byte
}

type User struct {
	name    string
	state   atomic.Pointer[snapshot]
	writeMu sync.Mutex
}

var (
	mu       sync.RWMutex
	registry = map[string]*User{}
)

func init() {
	Set(newDefaultUser())
}

func Get(name string) (*User, bool) {
	mu.RLock()
	defer mu.RUnlock()
	u, ok := registry[name]
	return u, ok
}

func Set(u *User) {
	mu.Lock()
	defer mu.Unlock()
	registry[u.Name()] = u
}

func newDefaultUser() *User {
	u := &User{name: "default"}
	u.state.Store(&snapshot{flags: []string{"nopass"}})
	return u
}

func New(name, password string) *User {
	mu.Lock()
	defer mu.Unlock()

	u, ok := registry[name]
	if !ok {
		u = &User{name: name}
		u.state.Store(&snapshot{flags: []string{"nopass"}})
	}

	u.AddPassword(password)

	return u
}

func HashPassword(password string) [32]byte {
	return sha256.Sum256([]byte(password))
}

func (u *User) Flags() []string {
	return slices.Clone(u.state.Load().flags)
}

func (u *User) Passwords() []string {
	pwds := u.state.Load().passwords
	out := make([]string, len(pwds))
	for i := range pwds {
		out[i] = hex.EncodeToString(pwds[i][:])
	}
	return out
}

func (u *User) AddPassword(password string) {
	u.writeMu.Lock()
	defer u.writeMu.Unlock()

	cur := u.state.Load()
	next := &snapshot{
		flags:     slices.Clone(cur.flags),
		passwords: append(slices.Clone(cur.passwords), HashPassword(password)),
	}
	if idx := slices.Index(next.flags, "nopass"); idx >= 0 {
		next.flags = slices.Delete(next.flags, idx, idx+1)
	}
	u.state.Store(next)
}

func (u *User) CheckHashedPassword(hashedPassword [32]byte) bool {
	passwords := u.state.Load().passwords
	var match int
	for i := range passwords {
		match |= subtle.ConstantTimeCompare(passwords[i][:], hashedPassword[:])
	}
	return match == 1
}

func (u *User) CheckPassword(password string) bool {
	hashedPassword := HashPassword(password)
	return u.CheckHashedPassword(hashedPassword)
}

func (u *User) PasswordRequired() bool {
	return !slices.Contains(u.state.Load().flags, "nopass")
}

func (u *User) Name() string {
	return u.name
}
