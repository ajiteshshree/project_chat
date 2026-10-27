package main

import (
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func fmtInt64(v int64) string {
	return strconv.FormatInt(v, 10)
}

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

var messageTypes = map[string]bool{
	"text":  true,
	"image": true,
	"video": true,
}

func randomID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

var badNameRe = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func sanitizeName(name string) string {
	name = filepath.Base(name)
	return strings.TrimLeft(badNameRe.ReplaceAllString(name, "_"), "._")
}