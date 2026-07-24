package authentication

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	defaultTOTPPeriod = uint(30)
	defaultTOTPDigits = otp.DigitsSix
)

var totpCodePattern = regexp.MustCompile(`^[0-9]{6}$`)

type TOTPSeed struct {
	Secret    string
	URI       string
	Algorithm string
	Digits    int
	Period    int
}

type TOTPVerifier interface {
	Generate(string) (TOTPSeed, error)
	Verify(string, string, time.Time, *int64) (int64, bool, error)
}

type standardTOTP struct {
	issuer string
}

func NewTOTPVerifier(issuer string) (TOTPVerifier, error) {
	if issuer == "" {
		return nil, errors.New("TOTP issuer is required")
	}
	return &standardTOTP{issuer: issuer}, nil
}

func (verifier *standardTOTP) Generate(accountName string) (TOTPSeed, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer: verifier.issuer, AccountName: accountName,
		Period: defaultTOTPPeriod, SecretSize: 20,
		Digits: defaultTOTPDigits, Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		return TOTPSeed{}, err
	}
	return TOTPSeed{
		Secret: key.Secret(), URI: key.URL(), Algorithm: "SHA1",
		Digits: defaultTOTPDigits.Length(), Period: int(defaultTOTPPeriod),
	}, nil
}

func (verifier *standardTOTP) Verify(
	secret, code string,
	now time.Time,
	lastUsedStep *int64,
) (int64, bool, error) {
	if !totpCodePattern.MatchString(code) {
		return 0, false, nil
	}
	currentStep := now.Unix() / int64(defaultTOTPPeriod)
	for offset := int64(-1); offset <= 1; offset++ {
		step := currentStep + offset
		if step < 0 || lastUsedStep != nil && step <= *lastUsedStep {
			continue
		}
		generated, err := totp.GenerateCodeCustom(
			secret,
			time.Unix(step*int64(defaultTOTPPeriod), 0),
			totp.ValidateOpts{
				Period: defaultTOTPPeriod, Skew: 0,
				Digits: defaultTOTPDigits, Algorithm: otp.AlgorithmSHA1,
			},
		)
		if err != nil {
			return 0, false, fmt.Errorf("generate TOTP verification code: %w", err)
		}
		if subtle.ConstantTimeCompare([]byte(generated), []byte(code)) == 1 {
			return step, true, nil
		}
	}
	return 0, false, nil
}

var _ TOTPVerifier = (*standardTOTP)(nil)
