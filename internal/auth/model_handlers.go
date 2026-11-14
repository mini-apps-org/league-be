package auth

// UserResponse is the API `User` shape: camelCase, no identity subject or
// timestamps.
type UserResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
}

// MeResponse is /api/v1/me minus `settings` and `apiKey`: neither table exists
// until M2. Roles and Permissions are always arrays, never null, so the SPA can
// call includes() on them without a guard.
type MeResponse struct {
	User        UserResponse `json:"user"`
	Roles       []string     `json:"roles" example:"CAPTAIN"`
	Permissions []string     `json:"permissions" example:"match.create,match.update"`
}

func newUserResponse(u *User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		AvatarURL: u.AvatarURL,
	}
}
