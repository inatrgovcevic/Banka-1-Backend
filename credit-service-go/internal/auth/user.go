package auth

type User struct {
	ID       int64
	Email    string
	Username string
	Role     string
}

func (u User) HasRole(role string) bool {
	return u.Role == role
}

func (u User) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if u.Role == role {
			return true
		}
	}
	return false
}
