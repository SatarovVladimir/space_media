package routes

import "net/http"

func New(
	testHandler http.Handler,
	dbTestHandler http.Handler,
	registerHandler http.Handler,
	loginHandler http.Handler,
	ordersHandler http.Handler,
	socialHandler http.Handler,
	authenticate func(http.Handler) http.Handler,
	authenticateOptional func(http.Handler) http.Handler,
) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/test", testHandler)
	mux.Handle("/dbtest", dbTestHandler)
	mux.Handle("POST /auth/register", registerHandler)
	mux.Handle("POST /auth/login", loginHandler)
	mux.Handle("/orders", authenticate(ordersHandler))
	mux.Handle("GET /articles", authenticateOptional(socialHandler))
	mux.Handle("POST /articles", authenticate(socialHandler))
	mux.Handle("GET /articles/", authenticateOptional(socialHandler))
	mux.Handle("POST /articles/", authenticate(socialHandler))
	mux.Handle("GET /profiles/", authenticateOptional(socialHandler))
	mux.Handle("/profile/", authenticate(socialHandler))
	mux.Handle("/follows", authenticate(socialHandler))
	mux.Handle("/follows/", authenticate(socialHandler))
	mux.Handle("/publication-plan", authenticate(socialHandler))
	return mux
}
