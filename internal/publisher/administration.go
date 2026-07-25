package publisher

import (
	"context"
	"sync"
)

type TrustState struct {
	mu      sync.RWMutex
	current VerifiedTrustManifest
}

func NewTrustState(initial VerifiedTrustManifest) *TrustState {
	return &TrustState{current: initial}
}

func (state *TrustState) Current() VerifiedTrustManifest {
	if state == nil {
		return VerifiedTrustManifest{}
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	return cloneVerifiedTrustManifest(state.current)
}

func (state *TrustState) install(next VerifiedTrustManifest) error {
	state.mu.Lock()
	defer state.mu.Unlock()
	if next.Manifest.ManifestVersion <= state.current.Manifest.ManifestVersion ||
		next.Manifest.Issuer != state.current.Manifest.Issuer ||
		next.Manifest.RootKeyID != state.current.Manifest.RootKeyID {
		return ErrInvalidTrustManifest
	}
	state.current = cloneVerifiedTrustManifest(next)
	return nil
}

type KeyAdministration struct {
	mu           sync.Mutex
	manager      ManagedKeyProvider
	trust        *TrustState
	roots        []TrustRoot
	manifestPath string
}

func NewKeyAdministration(
	manager ManagedKeyProvider,
	trust *TrustState,
	roots []TrustRoot,
	manifestPath string,
) (*KeyAdministration, error) {
	if manager == nil || trust == nil || len(roots) == 0 ||
		trust.Current().Manifest.ManifestVersion == 0 || manifestPath == "" {
		return nil, ErrInvalidTrustManifest
	}
	return &KeyAdministration{
		manager: manager, trust: trust, roots: roots, manifestPath: manifestPath,
	}, nil
}

func (admin *KeyAdministration) PrepareRotation(
	ctx context.Context,
) (VerificationKey, []VerificationKey, error) {
	admin.mu.Lock()
	defer admin.mu.Unlock()
	key, err := admin.manager.PrepareRotation(ctx)
	if err != nil {
		return VerificationKey{}, nil, err
	}
	return key, admin.manager.VerificationKeys(), nil
}

func (admin *KeyAdministration) ApplyTrustManifest(
	ctx context.Context,
	raw []byte,
) (VerifiedTrustManifest, error) {
	admin.mu.Lock()
	defer admin.mu.Unlock()
	var manifest TrustManifest
	if err := decodeStrict(raw, &manifest); err != nil {
		return VerifiedTrustManifest{}, invalidTrust(err)
	}
	verified, err := VerifyTrustManifest(ctx, manifest, admin.roots)
	if err != nil {
		return VerifiedTrustManifest{}, err
	}
	current := admin.trust.Current()
	if verified.Manifest.Issuer != current.Manifest.Issuer ||
		verified.Manifest.ManifestVersion <= current.Manifest.ManifestVersion {
		return VerifiedTrustManifest{}, ErrInvalidTrustManifest
	}
	if err := admin.manager.ApplyTrustManifest(ctx, verified); err != nil {
		return VerifiedTrustManifest{}, err
	}
	if err := WriteTrustManifest(admin.manifestPath, verified.Manifest); err != nil {
		return VerifiedTrustManifest{}, err
	}
	if err := admin.trust.install(verified); err != nil {
		return VerifiedTrustManifest{}, err
	}
	return verified, nil
}

func cloneVerifiedTrustManifest(value VerifiedTrustManifest) VerifiedTrustManifest {
	clone := value
	clone.Manifest.Keys = make([]VerificationKey, len(value.Manifest.Keys))
	for index := range value.Manifest.Keys {
		clone.Manifest.Keys[index] = cloneVerificationKey(value.Manifest.Keys[index])
	}
	return clone
}
