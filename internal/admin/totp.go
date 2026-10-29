package admin

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	totpStep       = 30
	totpDigits     = 6
	totpSecretSize = 20
)

// NewTOTPSecret returns a random base32 secret (no padding) for enrollment.
func NewTOTPSecret() (string, error) {
	b := make([]byte, totpSecretSize)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

// DecodeTOTPSecret accepts a base32 secret with optional spaces/dashes.
func DecodeTOTPSecret(s string) ([]byte, error) {
	s = strings.ToUpper(strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(s)))
	b, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
	if err != nil || len(b) < 10 {
		return nil, ErrInvalid
	}
	return b, nil
}

// VerifyTOTP checks a six-digit code at now, allowing one step of clock
// skew. It returns the accepted step, which must be strictly newer than the
// caller's stored last step to prevent replay.
func VerifyTOTP(secret []byte, code string, now time.Time, lastStep int64) (int64, bool) {
	code = strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(code))
	if len(code) != totpDigits {
		return 0, false
	}
	step := now.Unix() / totpStep
	for _, candidate := range []int64{step - 1, step, step + 1} {
		if candidate <= lastStep {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(hotp(secret, uint64(candidate))), []byte(code)) == 1 {
			return candidate, true
		}
	}
	return 0, false
}

// CodeAt returns the six-digit code for secret at now. Provisioning tools
// and tests use it to confirm enrollment; the secret is never logged.
func CodeAt(secret []byte, now time.Time) string {
	return hotp(secret, uint64(now.Unix()/totpStep))
}

func hotp(secret []byte, counter uint64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write(buf[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := (uint32(sum[offset])&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])
	return fmt.Sprintf("%06d", value%1_000_000)
}

// SealMFA encrypts a TOTP secret under the operator MFA key, bound to the
// staff id. The key comes from SELA_ADMIN_MFA_KEY and is never stored.
func sealMFA(key []byte, id AdminID, secret []byte) ([]byte, error) {
	gcm, err := mfaGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, secret, mfaAAD(id)), nil
}

func openMFA(key []byte, id AdminID, sealed []byte) ([]byte, error) {
	gcm, err := mfaGCM(key)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize()+gcm.Overhead() {
		return nil, ErrInvalid
	}
	nonce, ct := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, mfaAAD(id))
}

func mfaGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("admin: MFA key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func mfaAAD(id AdminID) []byte { return []byte("sela.admin-mfa.v1:" + id.String()) }
