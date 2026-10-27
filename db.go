package main

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations.sql
var migrationsFS embed.FS

func connectDB(ctx context.Context, url string) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error
	for i := 0; i < 10; i++ {
		pool, err = pgxpool.New(ctx, url)
		if err == nil {
			err = pool.Ping(ctx)
		}
		if err == nil {
			return pool, nil
		}
		fmt.Println("waiting for postgres:", err)
		time.Sleep(3 * time.Second)
	}
	return nil, err
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	sql, err := migrationsFS.ReadFile("migrations.sql")
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, string(sql))
	return err
}

func (s *Store) ensureLobby(ctx context.Context) error {
	id, err := s.insertConversationIfMissing(ctx, "lobby", "Lobby")
	if err != nil {
		return err
	}
	s.lobbyID = id
	return nil
}

// ensureLobbyMembership makes sure the given user belongs to the lobby.
func (s *Store) ensureLobbyMembership(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO conversation_members (conversation_id, user_id)
		 VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		s.lobbyID, userID)
	return err
}