package main

import "time"

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	ProfilePic   string    `json:"profile_pic"`
	Status       string    `json:"status"`
	Online       bool      `json:"online"`
	LastOnline   time.Time `json:"last_online"`
	CreatedAt    time.Time `json:"created_at"`
}

type Conversation struct {
	ID          int64      `json:"id"`
	Kind        string     `json:"kind"`
	Name        string     `json:"name"`
	Members     []User     `json:"members,omitempty"`
	LastMessage *Message   `json:"last_message,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Message struct {
	ID             int64     `json:"id"`
	ConversationID int64     `json:"conversation_id"`
	SenderID       int64     `json:"sender_id"`
	SenderUsername string    `json:"sender_username"`
	SenderPic      string    `json:"sender_pic"`
	Type           string    `json:"type"`
	Content        string    `json:"content"`
	FileURL        string    `json:"file_url"`
	CreatedAt      time.Time `json:"created_at"`
}