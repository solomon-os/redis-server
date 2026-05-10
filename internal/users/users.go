package users

import (
	"crypto/sha256"
	"fmt"
	"slices"
)

type User struct {
	flags     []string
	passwords []string
	name      string
}

var Users map[string]*User

func init() {
	Users = make(map[string]*User)
	user := newDefaultUser()
	Users[user.name] = user
}

func newDefaultUser() *User {
	return &User{
		flags: []string{"nopass"},
		name:  "default",
	}
}

func (u *User) Flags() []string {
	return slices.Clone(u.flags)
}

func (u *User) Passwords() []string {
	return slices.Clone(u.passwords)
}

func (u *User) AddPassword(password string) {
	u.passwords = append(u.passwords, fmt.Sprintf("%x", sha256.Sum256([]byte(password))))
	idx := slices.Index(u.flags, "nopass")
	if idx >= 0 {
		u.flags = slices.Delete(u.flags, idx, idx+1)
	}
}

func (u *User) Name() string {
	return u.name
}
