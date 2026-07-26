// Package password owns password normalization, policy evaluation, hashing,
// legacy verification, and progressive hash upgrades.
package password

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/text/unicode/norm"
)

const (
	ReasonTooShort  = "too_short"
	ReasonTooLong   = "too_long"
	ReasonBlocklist = "blocklisted"
	ReasonContext   = "context_specific"
)

type PolicyError struct {
	Reason string
	Limit  int
}

func (e *PolicyError) Error() string {
	switch e.Reason {
	case ReasonTooShort:
		return fmt.Sprintf("password must be at least %d characters", e.Limit)
	case ReasonTooLong:
		return fmt.Sprintf("password must be at most %d characters", e.Limit)
	case ReasonBlocklist:
		return "password appears in the common or compromised password blocklist"
	case ReasonContext:
		return "password must not match account-specific information"
	default:
		return "password does not satisfy policy"
	}
}

type Blocklist interface {
	Contains(context.Context, string) (bool, error)
}

type Context struct {
	Email       string
	DisplayName string
}

type Policy struct {
	MinLength     int
	MaxLength     int
	Normalization string
	Blocklist     bool
}

type Config struct {
	MinLength   int
	MaxLength   int
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
	Blocklist   Blocklist
}

func DefaultConfig() Config {
	return Config{
		MinLength: 8, MaxLength: 128,
		MemoryKiB: 19 * 1024, Iterations: 2, Parallelism: 1,
		SaltLength: 16, KeyLength: 32,
		Blocklist: BuiltinBlocklist(),
	}
}

type Manager struct {
	config Config
}

func New(config Config) *Manager {
	defaults := DefaultConfig()
	if config.MinLength <= 0 {
		config.MinLength = defaults.MinLength
	}
	if config.MaxLength < config.MinLength {
		config.MaxLength = defaults.MaxLength
	}
	if config.MemoryKiB == 0 {
		config.MemoryKiB = defaults.MemoryKiB
	}
	if config.Iterations == 0 {
		config.Iterations = defaults.Iterations
	}
	if config.Parallelism == 0 {
		config.Parallelism = defaults.Parallelism
	}
	if config.SaltLength == 0 {
		config.SaltLength = defaults.SaltLength
	}
	if config.KeyLength == 0 {
		config.KeyLength = defaults.KeyLength
	}
	if config.Blocklist == nil {
		config.Blocklist = defaults.Blocklist
	}
	return &Manager{config: config}
}

func (manager *Manager) Policy() Policy {
	return Policy{
		MinLength: manager.config.MinLength, MaxLength: manager.config.MaxLength,
		Normalization: "NFC", Blocklist: manager.config.Blocklist != nil,
	}
}

func Normalize(value string) string {
	return norm.NFC.String(value)
}

func (manager *Manager) Validate(
	ctx context.Context,
	plain string,
	account Context,
) (string, error) {
	normalized := Normalize(plain)
	length := utf8.RuneCountInString(normalized)
	if length < manager.config.MinLength {
		return "", &PolicyError{Reason: ReasonTooShort, Limit: manager.config.MinLength}
	}
	if length > manager.config.MaxLength {
		return "", &PolicyError{Reason: ReasonTooLong, Limit: manager.config.MaxLength}
	}
	if matchesContext(normalized, account) {
		return "", &PolicyError{Reason: ReasonContext}
	}
	blocked, err := manager.config.Blocklist.Contains(ctx, normalized)
	if err != nil {
		return "", fmt.Errorf("password blocklist: %w", err)
	}
	if blocked {
		return "", &PolicyError{Reason: ReasonBlocklist}
	}
	return normalized, nil
}

func (manager *Manager) Hash(normalized string) (string, error) {
	normalized = Normalize(normalized)
	salt := make([]byte, manager.config.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	digest := argon2.IDKey(
		[]byte(normalized), salt, manager.config.Iterations,
		manager.config.MemoryKiB, manager.config.Parallelism,
		manager.config.KeyLength,
	)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, manager.config.MemoryKiB, manager.config.Iterations,
		manager.config.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(digest),
	), nil
}

func (manager *Manager) Verify(encoded, plain string) bool {
	if strings.HasPrefix(encoded, "$2") {
		return bcrypt.CompareHashAndPassword([]byte(encoded), []byte(plain)) == nil
	}
	parameters, salt, expected, err := parseArgon2id(encoded)
	if err != nil {
		return false
	}
	actual := argon2.IDKey(
		[]byte(Normalize(plain)), salt, parameters.iterations,
		parameters.memoryKiB, parameters.parallelism, uint32(len(expected)),
	)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func (manager *Manager) NeedsRehash(encoded string) bool {
	if strings.HasPrefix(encoded, "$2") {
		return true
	}
	parameters, salt, digest, err := parseArgon2id(encoded)
	if err != nil {
		return false
	}
	return parameters.memoryKiB != manager.config.MemoryKiB ||
		parameters.iterations != manager.config.Iterations ||
		parameters.parallelism != manager.config.Parallelism ||
		uint32(len(salt)) != manager.config.SaltLength ||
		uint32(len(digest)) != manager.config.KeyLength
}

type argonParameters struct {
	memoryKiB   uint32
	iterations  uint32
	parallelism uint8
}

func parseArgon2id(encoded string) (argonParameters, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return argonParameters{}, nil, nil, errors.New("unsupported password hash")
	}
	var parameters argonParameters
	if _, err := fmt.Sscanf(
		parts[3], "m=%d,t=%d,p=%d",
		&parameters.memoryKiB, &parameters.iterations, &parameters.parallelism,
	); err != nil {
		return argonParameters{}, nil, nil, err
	}
	if parameters.memoryKiB == 0 || parameters.iterations == 0 ||
		parameters.parallelism == 0 || parameters.memoryKiB > 256*1024 ||
		parameters.iterations > 10 || parameters.parallelism > 16 {
		return argonParameters{}, nil, nil, errors.New("invalid argon2 parameters")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 || len(salt) > 64 {
		return argonParameters{}, nil, nil, errors.New("invalid argon2 salt")
	}
	digest, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(digest) < 16 || len(digest) > 64 {
		return argonParameters{}, nil, nil, errors.New("invalid argon2 digest")
	}
	return parameters, salt, digest, nil
}

func matchesContext(plain string, account Context) bool {
	candidate := strings.ToLower(strings.TrimSpace(plain))
	values := []string{
		strings.ToLower(strings.TrimSpace(account.Email)),
		strings.ToLower(strings.TrimSpace(account.DisplayName)),
	}
	if at := strings.IndexByte(values[0], '@'); at > 0 {
		values = append(values, values[0][:at])
	}
	for _, value := range values {
		if utf8.RuneCountInString(value) >= 4 && candidate == value {
			return true
		}
	}
	return false
}

func ParseReason(err error) string {
	var policyError *PolicyError
	if errors.As(err, &policyError) {
		return policyError.Reason
	}
	return ""
}
