package dto

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserInfo struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

type AuthResponse struct {
	Token string   `json:"token"`
	User  UserInfo `json:"user"`
}
