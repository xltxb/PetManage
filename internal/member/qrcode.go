package member

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

// QRCodeSecret is the HMAC key used to sign member QR code tokens.
// It is set via SetQRCodeSecret. If unset, a fallback value is used.
var qrCodeSecret string

// SetQRCodeSecret configures the QR code signing secret.
func SetQRCodeSecret(secret string) {
	qrCodeSecret = secret
}

func getQRSecret() string {
	if qrCodeSecret == "" {
		return "pet-member-qrcode-default-secret"
	}
	return qrCodeSecret
}

// GenerateQRCodeToken creates a signed token for a member QR code.
// Format: base64(memberID.merchantID.timestamp.signature)
// The signature is HMAC-SHA256(memberID.merchantID.timestamp, secret) truncated to 16 hex chars.
func GenerateQRCodeToken(memberID, merchantID int64) string {
	ts := time.Now().Unix()
	payload := fmt.Sprintf("%d.%d.%d", memberID, merchantID, ts)
	mac := hmac.New(sha256.New, []byte(getQRSecret()))
	mac.Write([]byte(payload))
	sig := fmt.Sprintf("%x", mac.Sum(nil))[:16]
	return base64.RawURLEncoding.EncodeToString([]byte(payload + "." + sig))
}

// VerifyQRCodeToken decodes and verifies a signed QR code token.
// Returns (memberID, merchantID, ok). If the token is invalid or tampered with, returns 0, 0, false.
func VerifyQRCodeToken(token string) (int64, int64, bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, 0, false
	}

	parts := strings.SplitN(string(decoded), ".", 4)
	if len(parts) != 4 {
		return 0, 0, false
	}

	payload := parts[0] + "." + parts[1] + "." + parts[2]
	expectedSig := parts[3]

	mac := hmac.New(sha256.New, []byte(getQRSecret()))
	mac.Write([]byte(payload))
	actualSig := fmt.Sprintf("%x", mac.Sum(nil))[:16]

	if !hmac.Equal([]byte(expectedSig), []byte(actualSig)) {
		return 0, 0, false
	}

	memberID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, false
	}

	merchantID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, false
	}

	return memberID, merchantID, true
}

// RenderQRCodePNG generates a PNG QR code image for the given content.
func RenderQRCodePNG(content string) ([]byte, error) {
	return qrcode.Encode(content, qrcode.Medium, 256)
}
