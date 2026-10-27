package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Server struct {
	cfg   Config
	store *Store
	obj   *ObjectStore
	hub   *Hub
}

var (
	usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)
	statusMax  = 200
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !usernameRe.MatchString(req.Username) {
		writeErr(w, http.StatusBadRequest, "username must be 3-32 chars: letters, numbers, _ and -")
		return
	}
	if len(req.Password) < 8 {
		writeErr(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	hash, err := hashPassword(req.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "server error")
		return
	}
	u, err := s.store.CreateUser(r.Context(), req.Username, hash)
	if err == errUsernameTaken {
		writeErr(w, http.StatusConflict, "username already taken")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "server error")
		return
	}
	if err := s.store.ensureLobbyMembership(r.Context(), u.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "server error")
		return
	}
	s.issueTokens(w, r, u)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request")
		return
	}
	u, err := s.store.GetUserByUsername(r.Context(), strings.TrimSpace(req.Username))
	if err != nil || !checkPassword(u.PasswordHash, req.Password) {
		writeErr(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	s.issueTokens(w, r, u)
}

func (s *Server) issueTokens(w http.ResponseWriter, r *http.Request, u *User) {
	token, err := s.newToken(u.ID, u.Username)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "server error")
		return
	}
	s.setCookie(w, r, token)
	u.Online = s.hub.isOnline(u.ID)
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  s.publicUser(u),
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	clearCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	userID, err := s.authenticate(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	switch r.Method {
	case http.MethodGet:
		u, err := s.store.GetUserByID(r.Context(), userID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "server error")
			return
		}
		u.Online = s.hub.isOnline(u.ID)
		writeJSON(w, http.StatusOK, map[string]any{"user": s.publicUser(u)})
	case http.MethodPatch:
		var req struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request")
			return
		}
		if len(req.Status) > statusMax {
			writeErr(w, http.StatusBadRequest, "status too long")
			return
		}
		if err := s.store.SetProfile(r.Context(), userID, req.Status, ""); err != nil {
			writeErr(w, http.StatusInternalServerError, "server error")
			return
		}
		u, err := s.store.GetUserByID(r.Context(), userID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": s.publicUser(u)})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID, err := s.authenticate(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	key, err := s.uploadOne(r, "avatars")
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.SetProfile(r.Context(), userID, "", key); err != nil {
		writeErr(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"key": key, "url": "/files/" + key})
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	userID, err := s.authenticate(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	_ = userID
	key, err := s.uploadOne(r, "uploads")
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"key": key, "url": "/files/" + key})
}

const maxUpload = 25 << 20 // 25 MB

func (s *Server) uploadOne(r *http.Request, prefix string) (string, error) {
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		return "", errors.New("upload too large (max 25 MB)")
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		return "", errors.New("missing file field")
	}
	defer f.Close()

	contentType := hdr.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	key := prefix + "/" + randomID() + "-" + sanitizeName(hdr.Filename)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	if err := s.obj.Put(ctx, key, f, hdr.Size, contentType); err != nil {
		return "", errors.New("upload failed")
	}
	return key, nil
}

func (s *Server) handleUserSearch(w http.ResponseWriter, r *http.Request) {
	userID, err := s.authenticate(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	users, err := s.store.SearchUsers(r.Context(), q, userID, 10)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "server error")
		return
	}
	out := make([]map[string]any, 0, len(users))
	for i := range users {
		users[i].Online = s.hub.isOnline(users[i].ID)
		out = append(out, s.publicUser(&users[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleConversations(w http.ResponseWriter, r *http.Request) {
	userID, err := s.authenticate(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.listConversations(w, r, userID)
	case http.MethodPost:
		s.createConversation(w, r, userID)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) listConversations(w http.ResponseWriter, r *http.Request, userID int64) {
	convs, err := s.store.listConversations(r.Context(), userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "server error")
		return
	}
	out := make([]map[string]any, 0, len(convs))
	for i := range convs {
		if lm, err := s.store.lastMessage(r.Context(), convs[i].ID); err == nil {
			convs[i].LastMessage = lm
		}
		out = append(out, s.conversationView(r.Context(), &convs[i], userID))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createConversation(w http.ResponseWriter, r *http.Request, userID int64) {
	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request")
		return
	}
	if strings.EqualFold(req.Username, "") {
		writeErr(w, http.StatusBadRequest, "username required")
		return
	}
	other, err := s.store.GetUserByUsername(r.Context(), strings.TrimSpace(req.Username))
	if err != nil {
		writeErr(w, http.StatusNotFound, "user not found")
		return
	}
	if other.ID == userID {
		writeErr(w, http.StatusBadRequest, "you are the lobby, not a user")
		return
	}
	conv, err := s.store.ensureDM(r.Context(), userID, other.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, s.conversationView(r.Context(), conv, userID))
}

func (s *Server) conversationView(ctx context.Context, c *Conversation, viewerID int64) map[string]any {
	view := map[string]any{
		"id":         c.ID,
		"kind":       c.Kind,
		"name":       c.Name,
		"members":    []any{},
		"last_message": nil,
	}
	if c.LastMessage != nil {
		view["last_message"] = c.LastMessage
	}
	if c.Kind == "lobby" {
		ids, _ := s.store.conversationMembers(ctx, c.ID)
		online := 0
		for _, id := range ids {
			if s.hub.isOnline(id) {
				online++
			}
		}
		view["member_count"] = len(ids)
		view["online_count"] = online
		return view
	}
	ids, _ := s.store.conversationMembers(ctx, c.ID)
	var others []map[string]any
	for _, id := range ids {
		if id == viewerID {
			continue
		}
		u, err := s.store.GetUserByID(ctx, id)
		if err != nil {
			continue
		}
		u.Online = s.hub.isOnline(id)
		others = append(others, s.publicUser(u))
	}
	view["members"] = others
	return view
}

func (s *Server) handleConversationDetail(w http.ResponseWriter, r *http.Request) {
	convID, err := parseInt64(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad conversation id")
		return
	}
	userID, err := s.authenticate(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !s.isMember(userID, convID) {
		writeErr(w, http.StatusForbidden, "not a member")
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := parseInt64(v); err == nil && n > 0 && n <= 200 {
			limit = int(n)
		}
	}
	before := int64(0)
	if v := r.URL.Query().Get("before"); v != "" {
		before, _ = parseInt64(v)
	}
	msgs, err := s.store.messagesBefore(r.Context(), convID, before, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, msgs)
}

func (s *Server) isMember(userID, convID int64) bool {
	ids, err := s.store.conversationMembers(context.Background(), convID)
	if err != nil {
		return false
	}
	for _, id := range ids {
		if id == userID {
			return true
		}
	}
	return false
}

func (s *Server) publicUser(u *User) map[string]any {
	return map[string]any{
		"id":          u.ID,
		"username":    u.Username,
		"profile_pic": u.ProfilePic,
		"status":      u.Status,
		"online":      u.Online,
		"last_online": u.LastOnline.Format(time.RFC3339),
		"created_at":  u.CreatedAt.Format(time.RFC3339),
	}
}

