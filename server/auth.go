package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// Token = base64url(uid:role:refID:expUnix) + "." + HMAC-SHA256.
// Zero-dependency JWT: cukup utk stateless auth multi-worker di Railway.
func SignToken(secret string, uid int64, role string, refID int64, ttl time.Duration) string {
	payload := fmt.Sprintf("%d:%s:%d:%d", uid, role, refID, time.Now().Add(ttl).Unix())
	enc := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return enc + "." + hmacHex(secret, enc)
}

type Claims struct {
	UID   int64
	Role  string
	RefID int64
}

func VerifyToken(secret, tok string) (*Claims, error) {
	parts := strings.SplitN(tok, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("token rusak")
	}
	if !hmac.Equal([]byte(hmacHex(secret, parts[0])), []byte(parts[1])) {
		return nil, fmt.Errorf("signature tidak valid")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	f := strings.Split(string(raw), ":")
	if len(f) != 4 {
		return nil, fmt.Errorf("payload rusak")
	}
	exp, err := strconv.ParseInt(f[3], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return nil, fmt.Errorf("token kedaluwarsa")
	}
	uid, _ := strconv.ParseInt(f[0], 10, 64)
	ref, _ := strconv.ParseInt(f[2], 10, 64)
	return &Claims{UID: uid, Role: f[1], RefID: ref}, nil
}

func hmacHex(secret, msg string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b) // crypto/rand: aman utk nama file unik
	return hex.EncodeToString(b)[:n]
}
