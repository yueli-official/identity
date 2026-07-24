package authentication

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

type WebAuthnConfig struct {
	RPID             string
	RPDisplayName    string
	RPOrigins        []string
	RPTopOrigins     []string
	AllowCrossOrigin bool
}

type webAuthnAdapter struct {
	rpID    string
	library *webauthn.WebAuthn
}

func NewWebAuthnVerifier(config WebAuthnConfig) (WebAuthnVerifier, error) {
	if config.RPID == "" || config.RPDisplayName == "" || len(config.RPOrigins) == 0 {
		return nil, errors.New("WebAuthn RP id, display name, and origins are required")
	}
	library, err := webauthn.New(&webauthn.Config{
		RPID: config.RPID, RPDisplayName: config.RPDisplayName,
		RPOrigins: config.RPOrigins, RPTopOrigins: config.RPTopOrigins,
		RPAllowCrossOrigin:    config.AllowCrossOrigin,
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationRequired,
		},
	})
	if err != nil {
		return nil, err
	}
	return &webAuthnAdapter{rpID: config.RPID, library: library}, nil
}

func (adapter *webAuthnAdapter) BeginRegistration(
	user PasskeyUser,
) (CeremonyMaterial, BrowserOptions, error) {
	creation, session, err := adapter.library.BeginRegistration(
		webAuthnUserFromDomain(user),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
	)
	if err != nil {
		return CeremonyMaterial{}, BrowserOptions{}, err
	}
	return encodeCeremony(session, creation)
}

func (adapter *webAuthnAdapter) FinishRegistration(
	user PasskeyUser,
	material CeremonyMaterial,
	response []byte,
) (PasskeyCredential, error) {
	parsed, err := protocol.ParseCredentialCreationResponseBytes(response)
	if err != nil {
		return PasskeyCredential{}, err
	}
	session, err := decodeCeremony(material, parsed.Response.CollectedClientData.Challenge)
	if err != nil {
		return PasskeyCredential{}, err
	}
	credential, err := adapter.library.CreateCredential(webAuthnUserFromDomain(user), session, parsed)
	if err != nil {
		return PasskeyCredential{}, err
	}
	return credentialToDomain(adapter.rpID, user.IdentityID, *credential, true), nil
}

func (adapter *webAuthnAdapter) BeginDiscoverableAuthentication() (
	CeremonyMaterial,
	BrowserOptions,
	error,
) {
	assertion, session, err := adapter.library.BeginDiscoverableLogin(
		webauthn.WithUserVerification(protocol.VerificationRequired),
	)
	if err != nil {
		return CeremonyMaterial{}, BrowserOptions{}, err
	}
	return encodeCeremony(session, assertion)
}

func (adapter *webAuthnAdapter) FinishDiscoverableAuthentication(
	material CeremonyMaterial,
	response []byte,
	resolve func([]byte) (PasskeyUser, error),
) (PasskeyUser, PasskeyCredential, error) {
	parsed, err := protocol.ParseCredentialRequestResponseBytes(response)
	if err != nil {
		return PasskeyUser{}, PasskeyCredential{}, err
	}
	session, err := decodeCeremony(material, parsed.Response.CollectedClientData.Challenge)
	if err != nil {
		return PasskeyUser{}, PasskeyCredential{}, err
	}
	if len(parsed.Response.UserHandle) == 0 {
		return PasskeyUser{}, PasskeyCredential{}, ErrCeremonyInvalid
	}
	user, err := resolve(parsed.Response.UserHandle)
	if err != nil {
		return PasskeyUser{}, PasskeyCredential{}, ErrCeremonyInvalid
	}
	credential, err := adapter.library.ValidateLogin(webAuthnUserFromDomain(user), session, parsed)
	if err != nil {
		return PasskeyUser{}, PasskeyCredential{}, err
	}
	updated := credentialToDomain(adapter.rpID, user.IdentityID, *credential, false)
	for _, stored := range user.Credentials {
		if bytes.Equal(stored.CredentialID, updated.CredentialID) {
			updated.ID = stored.ID
			updated.Version = stored.Version
			updated.UserVerifiedAtRegistration = stored.UserVerifiedAtRegistration
			updated.Status = stored.Status
			updated.Label = stored.Label
			break
		}
	}
	if updated.ID == "" {
		return PasskeyUser{}, PasskeyCredential{}, ErrPasskeyNotFound
	}
	return user, updated, nil
}

func encodeCeremony(session *webauthn.SessionData, options any) (
	CeremonyMaterial,
	BrowserOptions,
	error,
) {
	challenge := session.Challenge
	stored := *session
	stored.Challenge = ""
	state, err := json.Marshal(stored)
	if err != nil {
		return CeremonyMaterial{}, BrowserOptions{}, err
	}
	wire, err := json.Marshal(options)
	if err != nil {
		return CeremonyMaterial{}, BrowserOptions{}, err
	}
	return CeremonyMaterial{
		ChallengeDigest: challengeDigest(challenge),
		LibraryState:    state,
	}, BrowserOptions{JSON: wire}, nil
}

func decodeCeremony(material CeremonyMaterial, presentedChallenge string) (webauthn.SessionData, error) {
	got := challengeDigest(presentedChallenge)
	if len(got) != len(material.ChallengeDigest) ||
		subtle.ConstantTimeCompare(got, material.ChallengeDigest) != 1 {
		return webauthn.SessionData{}, ErrCeremonyInvalid
	}
	var session webauthn.SessionData
	if err := json.Unmarshal(material.LibraryState, &session); err != nil {
		return webauthn.SessionData{}, ErrCeremonyInvalid
	}
	session.Challenge = presentedChallenge
	return session, nil
}

type webAuthnUser struct {
	handle      []byte
	name        string
	displayName string
	credentials []webauthn.Credential
}

func (user webAuthnUser) WebAuthnID() []byte                         { return user.handle }
func (user webAuthnUser) WebAuthnName() string                       { return user.name }
func (user webAuthnUser) WebAuthnDisplayName() string                { return user.displayName }
func (user webAuthnUser) WebAuthnCredentials() []webauthn.Credential { return user.credentials }

func webAuthnUserFromDomain(user PasskeyUser) webAuthnUser {
	credentials := make([]webauthn.Credential, 0, len(user.Credentials))
	for _, credential := range user.Credentials {
		if credential.Status != "active" {
			continue
		}
		credentials = append(credentials, credentialToLibrary(credential))
	}
	return webAuthnUser{
		handle: user.UserHandle, name: user.Name, displayName: user.DisplayName,
		credentials: credentials,
	}
}

func credentialToDomain(
	rpID, identityID string,
	value webauthn.Credential,
	registration bool,
) PasskeyCredential {
	transports := make([]string, len(value.Transport))
	for i, transport := range value.Transport {
		transports[i] = string(transport)
	}
	return PasskeyCredential{
		IdentityID: identityID, RPID: rpID,
		CredentialID: value.ID, PublicKey: value.PublicKey,
		PublicKeyAlgorithm: value.Attestation.PublicKeyAlgorithm,
		Transports:         transports, Attachment: string(value.Authenticator.Attachment),
		AttestationType: value.AttestationType, AttestationFormat: value.AttestationFormat,
		AAGUID: value.Authenticator.AAGUID, SignCount: value.Authenticator.SignCount,
		CloneWarning: value.Authenticator.CloneWarning, Flags: byte(value.Flags.ProtocolValue()),
		UserVerified:               value.Flags.UserVerified,
		UserVerifiedAtRegistration: registration && value.Flags.UserVerified,
		BackupEligible:             value.Flags.BackupEligible, BackupState: value.Flags.BackupState,
		AttestationClientDataJSON:    value.Attestation.ClientDataJSON,
		AttestationClientDataHash:    value.Attestation.ClientDataHash,
		AttestationAuthenticatorData: value.Attestation.AuthenticatorData,
		AttestationObject:            value.Attestation.Object,
	}
}

func credentialToLibrary(value PasskeyCredential) webauthn.Credential {
	transports := make([]protocol.AuthenticatorTransport, len(value.Transports))
	for i, transport := range value.Transports {
		transports[i] = protocol.AuthenticatorTransport(transport)
	}
	return webauthn.Credential{
		ID: value.CredentialID, PublicKey: value.PublicKey,
		AttestationType: value.AttestationType, AttestationFormat: value.AttestationFormat,
		Transport: transports,
		Flags:     webauthn.NewCredentialFlags(protocol.AuthenticatorFlags(value.Flags)),
		Authenticator: webauthn.Authenticator{
			AAGUID: value.AAGUID, SignCount: value.SignCount,
			CloneWarning: value.CloneWarning,
			Attachment:   protocol.AuthenticatorAttachment(value.Attachment),
		},
		Attestation: webauthn.CredentialAttestation{
			ClientDataJSON:     value.AttestationClientDataJSON,
			ClientDataHash:     value.AttestationClientDataHash,
			AuthenticatorData:  value.AttestationAuthenticatorData,
			PublicKeyAlgorithm: value.PublicKeyAlgorithm,
			Object:             value.AttestationObject,
		},
	}
}

var _ WebAuthnVerifier = (*webAuthnAdapter)(nil)
