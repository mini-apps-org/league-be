package auth

// UserResponse is the API `User` shape: camelCase, no identity subject or
// timestamps.
type UserResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
}

type CurrentUserResponse struct {
	User        UserResponse `json:"user"`
	Roles       []string     `json:"roles" example:"CAPTAIN"`
	Permissions []string     `json:"permissions" example:"match.create,match.update"`
}

func newUserResponse(user *User) UserResponse {
	return UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		AvatarURL: user.AvatarURL,
	}
}
