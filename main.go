package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
)

func signalContext() context.Context {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	_ = stop
	return ctx
}

//go:embed all:web/dist
var webFS embed.FS

func main() {
	ctx := signalContext()

	pool, err := connectDB(ctx, getenv("DATABASE_URL", "postgres://chat:chat@localhost:5432/chatdb"))
	if err != nil {
		log.Fatal("postgres:", err)
	}
	defer pool.Close()

	if err := runMigrations(ctx, pool); err != nil {
		log.Fatal("migrations:", err)
	}

	store := &Store{pool: pool}
	if err := store.ensureLobby(ctx); err != nil {
		log.Fatal("lobby seed:", err)
	}

	obj, err := connectMinIO(ctx, loadMinIO())
	if err != nil {
		log.Fatal("minio:", err)
	}

	s := &Server{
		cfg:   loadConfig(),
		store: store,
		obj:   obj,
		hub:   newHub(store),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/register", s.handleRegister)
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/me", s.handleMe)
	mux.HandleFunc("PATCH /api/me", s.handleMe)
	mux.HandleFunc("POST /api/me/profile-picture", s.handleUploadAvatar)
	mux.HandleFunc("POST /api/upload", s.handleUpload)
	mux.HandleFunc("GET /api/users", s.handleUserSearch)
	mux.HandleFunc("GET /api/conversations", s.handleConversations)
	mux.HandleFunc("POST /api/conversations", s.handleConversations)
	mux.HandleFunc("GET /api/conversations/{id}/messages", s.handleConversationDetail)
	mux.HandleFunc("GET /ws", s.handleWebSocket)
	mux.HandleFunc("GET /files/", s.handleFile)
	mux.Handle("/", spaHandler())

	addr := ":" + s.cfg.Addr
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, logRequests(mux)); err != nil {
		log.Fatal(err)
	}
}

// spaHandler serves the embedded frontend, falling back to index.html for
// unknown paths so client routing works.
func spaHandler() http.Handler {
	sub, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if _, err := fs.Stat(sub, p); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}

func logRequests(next http.Handler) http.Handler {
	if os.Getenv("LOG_REQUESTS") == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}