package users

import "slices"

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

func (u *User) Name() string {
	return u.name
}
