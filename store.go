package main

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool    *pgxpool.Pool
	lobbyID int64
}

var errUsernameTaken = errors.New("username taken")

func (s *Store) CreateUser(ctx context.Context, username, passwordHash string) (*User, error) {
	u := &User{Username: username}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (username, password_hash)
		 VALUES ($1, $2)
		 RETURNING id, profile_pic, status, last_online, created_at`,
		username, passwordHash,
	).Scan(&u.ID, &u.ProfilePic, &u.Status, &u.LastOnline, &u.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return nil, errUsernameTaken
		}
		return nil, err
	}
	return u, nil
}

const userCols = `id, username, profile_pic, status, last_online, created_at`

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	u := &User{Username: username}
	err := s.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, profile_pic, status, last_online, created_at FROM users WHERE username = $1`, username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.ProfilePic, &u.Status, &u.LastOnline, &u.CreatedAt)
	return u, err
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (*User, error) {
	u := &User{ID: id}
	err := s.pool.QueryRow(ctx,
		`SELECT `+userCols+` FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Username, &u.ProfilePic, &u.Status, &u.LastOnline, &u.CreatedAt)
	return u, err
}

func (s *Store) SetProfile(ctx context.Context, id int64, status, profilePic string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET status = $2, profile_pic = COALESCE(NULLIF($3, ''), profile_pic) WHERE id = $1`,
		id, status, profilePic)
	return err
}

func (s *Store) SetLastOnline(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET last_online = now() WHERE id = $1`, id)
	return err
}

func (s *Store) SearchUsers(ctx context.Context, q string, selfID int64, limit int) ([]User, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+userCols+` FROM users
		 WHERE username ILIKE '%' || $1 || '%' AND id <> $2
		 ORDER BY username LIMIT $3`,
		q, selfID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.ProfilePic, &u.Status, &u.LastOnline, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) ensureDM(ctx context.Context, a, b int64) (*Conversation, error) {
	// Find existing DM between the two users.
	var id int64
	err := s.pool.QueryRow(ctx, `
		SELECT c.id FROM conversations c
		JOIN conversation_members m1 ON m1.conversation_id = c.id AND m1.user_id = $1
		JOIN conversation_members m2 ON m2.conversation_id = c.id AND m2.user_id = $2
		WHERE c.kind = 'dm'`,
		a, b).Scan(&id)
	if err == nil {
		return &Conversation{ID: id, Kind: "dm"}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	// Create it.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := tx.QueryRow(ctx,
		`INSERT INTO conversations (kind) VALUES ('dm') RETURNING id`).Scan(&id); err != nil {
		return nil, err
	}
	for _, uid := range []int64{a, b} {
		if _, err := tx.Exec(ctx,
			`INSERT INTO conversation_members (conversation_id, user_id) VALUES ($1, $2)`,
			id, uid); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &Conversation{ID: id, Kind: "dm"}, nil
}

func (s *Store) insertConversationIfMissing(ctx context.Context, kind, name string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO conversations (kind, name) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING RETURNING id`, kind, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	// Already exists — fetch it.
	row := s.pool.QueryRow(ctx,
		`SELECT id FROM conversations WHERE kind = $1 LIMIT 1`, kind)
	err = row.Scan(&id)
	return id, err
}

// listConversations returns all conversations the user belongs to, newest first.
func (s *Store) listConversations(ctx context.Context, userID int64) ([]Conversation, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.kind, c.name, c.created_at
		FROM conversations c
		JOIN conversation_members cm ON cm.conversation_id = c.id
		WHERE cm.user_id = $1
		ORDER BY (c.kind = 'lobby') DESC, c.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.Kind, &c.Name, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) conversationMembers(ctx context.Context, convID int64) ([]int64, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT user_id FROM conversation_members WHERE conversation_id = $1`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) lastMessage(ctx context.Context, convID int64) (*Message, error) {
	m := &Message{}
	err := s.pool.QueryRow(ctx, `
		SELECT m.id, m.conversation_id, m.sender_id, m.type, m.content, m.file_url, m.created_at,
		       COALESCE(u.username, ''), COALESCE(u.profile_pic, '')
		FROM messages m
		LEFT JOIN users u ON u.id = m.sender_id
		WHERE m.conversation_id = $1
		ORDER BY m.id DESC LIMIT 1`, convID).
		Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Type, &m.Content, &m.FileURL, &m.CreatedAt,
			&m.SenderUsername, &m.SenderPic)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Store) messagesBefore(ctx context.Context, convID int64, before int64, limit int) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.conversation_id, m.sender_id, m.type, m.content, m.file_url, m.created_at,
		       COALESCE(u.username, ''), COALESCE(u.profile_pic, '')
		FROM messages m
		LEFT JOIN users u ON u.id = m.sender_id
		WHERE m.conversation_id = $1 AND ($2 = 0 OR m.id < $2)
		ORDER BY m.id DESC LIMIT $3`, convID, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Type, &m.Content, &m.FileURL, &m.CreatedAt,
			&m.SenderUsername, &m.SenderPic); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// newest last
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (s *Store) insertMessage(ctx context.Context, convID, senderID int64, mtype, content, fileURL string) (*Message, error) {
	m := &Message{ConversationID: convID, SenderID: senderID, Type: mtype, Content: content, FileURL: fileURL}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO messages (conversation_id, sender_id, type, content, file_url)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`, convID, senderID, mtype, content, fileURL,
	).Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	u, err := s.GetUserByID(ctx, senderID)
	if err == nil {
		m.SenderUsername = u.Username
		m.SenderPic = u.ProfilePic
	}
	return m, nil
}