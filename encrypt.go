package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"
)

const encPrefix = "ENC["
const encSuffix = "]"

// SetEncryptionKey sets the key used for encrypting and decrypting
// configuration values. The key can be any length; it is hashed with
// SHA-256 to produce a 32-byte AES-256 key.
//
// Must be called before InitServiceConfig if the config file contains
// encrypted values.
//
// Example:
//
//	config.SetEncryptionKey([]byte(os.Getenv("CONFIG_KEY")))
func SetEncryptionKey(key []byte) {
	mu.Lock()
	defer mu.Unlock()

	h := sha256.Sum256(key)
	globalConfig.encryptionKey = h[:]
}

// EncryptValue encrypts a plaintext string and returns it in the
// format ENC[base64data]. The encryption key must be set first via
// SetEncryptionKey.
//
// Use this to prepare values before storing them in a config file.
//
// Example:
//
//	encrypted, err := config.EncryptValue("my-secret-password")
//	// encrypted = "ENC[base64...]"
func EncryptValue(plaintext string) (string, error) {
	mu.RLock()
	key := globalConfig.encryptionKey
	mu.RUnlock()

	if len(key) == 0 {
		return "", fmt.Errorf("encryption key not set: call SetEncryptionKey first")
	}

	ciphertext, err := encryptAESGCM(key, []byte(plaintext))
	if err != nil {
		return "", err
	}

	return encPrefix + base64.StdEncoding.EncodeToString(ciphertext) + encSuffix, nil
}

// DecryptValue decrypts a value in the format ENC[base64data] and
// returns the plaintext string. If the value is not encrypted (no
// ENC[...] wrapper), it is returned unchanged.
//
// Example:
//
//	plain, err := config.DecryptValue("ENC[base64...]")
func DecryptValue(value string) (string, error) {
	mu.RLock()
	key := globalConfig.encryptionKey
	mu.RUnlock()

	return decryptIfEncrypted(key, value)
}

// IsEncryptedValue reports whether s is in the ENC[...] format.
func IsEncryptedValue(s string) bool {
	return strings.HasPrefix(s, encPrefix) && strings.HasSuffix(s, encSuffix)
}

// decryptIfEncrypted decrypts a value if it is in ENC[...] format,
// otherwise returns it unchanged.
func decryptIfEncrypted(key []byte, value string) (string, error) {
	if !IsEncryptedValue(value) {
		return value, nil
	}

	if len(key) == 0 {
		return "", fmt.Errorf("encryption key not set but encrypted value found")
	}

	encoded := value[len(encPrefix) : len(value)-len(encSuffix)]
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decoding encrypted value: %w", err)
	}

	plaintext, err := decryptAESGCM(key, ciphertext)
	if err != nil {
		return "", fmt.Errorf("decrypting value: %w", err)
	}

	return string(plaintext), nil
}

// decryptConfigFields walks the config struct and its Service field,
// decrypting any string fields that contain ENC[...] values.
func decryptConfigFields(c *Config) error {
	// Decrypt base config string fields
	fields := []struct {
		ptr  *string
		name string
	}{
		{&c.AppID, "appID"},
		{&c.AppSecret, "appSecret"},
		{&c.Environment, "environment"},
		{&c.Logger.LogLevel, "logger.logLevel"},
	}

	for _, f := range fields {
		decrypted, err := decryptIfEncrypted(c.encryptionKey, *f.ptr)
		if err != nil {
			return fmt.Errorf("decrypting %s: %w", f.name, err)
		}
		*f.ptr = decrypted
	}

	// Decrypt service config fields using reflection
	if err := decryptStructFields(c.encryptionKey, c.Service); err != nil {
		return fmt.Errorf("decrypting service config: %w", err)
	}

	return nil
}

// decryptStructFields uses reflection to find and decrypt any string
// fields in a struct that contain ENC[...] values.
func decryptStructFields(key []byte, v any) error {
	if v == nil {
		return nil
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return nil
	}

	for i := range rv.NumField() {
		field := rv.Field(i)
		if field.Kind() == reflect.String && field.CanSet() {
			val := field.String()
			if IsEncryptedValue(val) {
				decrypted, err := decryptIfEncrypted(key, val)
				if err != nil {
					return fmt.Errorf("field %s: %w", rv.Type().Field(i).Name, err)
				}
				field.SetString(decrypted)
			}
		}
	}

	return nil
}

func encryptAESGCM(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decryptAESGCM(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
