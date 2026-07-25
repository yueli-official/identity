// Command identity is the user-center IdP backend: an OIDC / OAuth2
// authorization server layered on the identity core.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/net/goai"
	"github.com/gogf/gf/v2/os/gctx"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	_ "github.com/gogf/gf/contrib/nosql/redis/v2"

	foundationabuse "github.com/yueli-official/foundation/go/abuse"
	"github.com/yueli-official/foundation/go/abuse/turnstile"
	foundationauth "github.com/yueli-official/foundation/go/auth"
	"github.com/yueli-official/foundation/go/privacy"
	privacyadapter "github.com/yueli-official/foundation/go/privacy/httpadapter"
	"github.com/yueli-official/foundation/go/work"
	workpostgres "github.com/yueli-official/foundation/go/work/postgres"
	"platform/gokit/authhttp"
	"platform/gokit/capability"
	"platform/gokit/ghttpx"
	"platform/gokit/healthcheck"
	"platform/gokit/notificationclient"
	"platform/gokit/observability"
	"platform/gokit/openapiexport"
	"platform/gokit/privacycatalog"
	"platform/gokit/privacyhttp"
	"platform/gokit/stepup"
	"platform/services/identity/internal/assetclient"
	"platform/services/identity/internal/authentication"
	"platform/services/identity/internal/cache"
	"platform/services/identity/internal/controller"
	"platform/services/identity/internal/dao"
	"platform/services/identity/internal/githubbinding"
	"platform/services/identity/internal/guest"
	"platform/services/identity/internal/identityabuse"
	"platform/services/identity/internal/identitycap"
	"platform/services/identity/internal/identitymaintenance"
	"platform/services/identity/internal/identityprivacy"
	"platform/services/identity/internal/identitysecurity"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/mailer"
	"platform/services/identity/internal/oauthlogin"
	"platform/services/identity/internal/oidc"
	"platform/services/identity/internal/publisher"
	"platform/services/identity/internal/repo"
)

type runtimeRepositories struct {
	store          repo.Store
	guestStore     repo.GuestSessionStore
	clients        repo.ClientRepo
	signingKeys    repo.SigningKeyRepo
	audit          repo.AuditRepo
	oidcBackend    oidc.Backend
	passkeys       authentication.PasskeyStore
	mfa            authentication.MFAStore
	sessionCache   authentication.SessionCache
	publisherStore publisher.Store
	githubStore    githubbinding.Store
}

type privacyOwnerConfig struct {
	Key               string `json:"key"`
	Kind              string `json:"kind"`
	URL               string `json:"url"`
	ClientID          string `json:"clientId"`
	ClientSecret      string `json:"clientSecret"`
	AllowInsecureHTTP bool   `json:"allowInsecureHttp"`
}

type publisherConsumerConfig struct {
	Audience      string            `json:"audience"`
	Instance      string            `json:"instance"`
	Enabled       bool              `json:"enabled"`
	ArtifactKinds map[string]string `json:"artifactKinds"`
}

func newRuntimeRepositories(openAPIExport bool) runtimeRepositories {
	if openAPIExport {
		memory := repo.NewMemory()
		return runtimeRepositories{
			store:          memory,
			guestStore:     memory,
			clients:        memory,
			signingKeys:    memory,
			audit:          memory,
			oidcBackend:    oidc.NewMemBackend(),
			publisherStore: publisher.NewMemoryStore(),
			githubStore:    githubbinding.NewMemoryStore(),
		}
	}

	postgres := dao.NewPG(g.DB())
	redis := cache.NewRedis(g.Redis())
	return runtimeRepositories{
		store:          repo.NewComposite(postgres, repo.NewRecoveringSessionStore(redis, postgres), redis),
		guestStore:     postgres,
		clients:        postgres,
		signingKeys:    postgres,
		audit:          postgres,
		oidcBackend:    oidc.NewPGBackend(g.DB()),
		passkeys:       postgres,
		mfa:            postgres,
		sessionCache:   redis,
		publisherStore: postgres,
		githubStore:    postgres,
	}
}

func main() {
	ctx := gctx.New()
	var err error
	openAPIExport := os.Getenv(openapiexport.OutputEnv) != ""
	if !openAPIExport {
		shutdown, err := observability.StartFromEnvironment(ctx, "identity-api")
		if err != nil {
			panic(err)
		}
		defer observability.ShutdownWithTimeout(shutdown)
	}

	// ── Config ──────────────────────────────────────────────────────────────
	issuer := g.Cfg().MustGet(ctx, "oidc.issuer").String()
	loginURL := g.Cfg().MustGet(ctx, "account.loginUrl").String()
	globalSecret := g.Cfg().MustGet(ctx, "oidc.globalSecret").String()
	if len(globalSecret) < 32 {
		panic(fmt.Sprintf("oidc.globalSecret must be >= 32 bytes (got %d); set GF_OIDC_GLOBALSECRET", len(globalSecret)))
	}
	secureCookie := g.Cfg().MustGet(ctx, "cookie.secure", true).Bool()
	accountBaseURL := g.Cfg().MustGet(ctx, "account.baseUrl", "http://localhost:3000").String()
	assetBaseURL := g.Cfg().MustGet(ctx, "asset.baseUrl", "http://localhost:8082").String()
	assetAudience := g.Cfg().MustGet(ctx, "asset.audience").String()
	commerceBaseURL := g.Cfg().MustGet(ctx, "commerce.baseUrl").String()
	commerceAudience := g.Cfg().MustGet(ctx, "commerce.audience").String()
	notificationBaseURL := g.Cfg().MustGet(ctx, "notification.baseUrl").String()
	notificationAudience := g.Cfg().MustGet(ctx, "notification.audience").String()

	// ── Business auth ───────────────────────────────────────────────────────
	cfg := logic.DefaultConfig()
	cfg.AccountBaseURL = accountBaseURL // base for verify-email / reset links
	if sessionIdleTTL := g.Cfg().MustGet(ctx, "auth.sessionIdleTtl").Duration(); sessionIdleTTL > 0 {
		cfg.SessionIdleTTL = sessionIdleTTL
	}
	repositories := newRuntimeRepositories(openAPIExport)
	var authenticationModule *authentication.Module
	if !openAPIExport {
		webAuthnVerifier, verifierErr := authentication.NewWebAuthnVerifier(authentication.WebAuthnConfig{
			RPID:             g.Cfg().MustGet(ctx, "webauthn.rpId").String(),
			RPDisplayName:    g.Cfg().MustGet(ctx, "webauthn.displayName").String(),
			RPOrigins:        g.Cfg().MustGet(ctx, "webauthn.origins").Strings(),
			RPTopOrigins:     g.Cfg().MustGet(ctx, "webauthn.topOrigins").Strings(),
			AllowCrossOrigin: g.Cfg().MustGet(ctx, "webauthn.allowCrossOrigin", false).Bool(),
		})
		if verifierErr != nil {
			panic(fmt.Sprintf("identity WebAuthn verifier: %v", verifierErr))
		}
		authenticationModule, err = authentication.NewModule(
			repositories.passkeys,
			repositories.sessionCache,
			webAuthnVerifier,
			authentication.ModuleConfig{
				SessionTTL:     cfg.SessionIdleTTL,
				CeremonyTTL:    g.Cfg().MustGet(ctx, "webauthn.ceremonyTtl", "5m").Duration(),
				TransactionTTL: g.Cfg().MustGet(ctx, "mfa.transactionTtl", "5m").Duration(),
				RecoveryTTL:    g.Cfg().MustGet(ctx, "mfa.recoverySessionTtl", "15m").Duration(),
			},
		)
		if err != nil {
			panic(fmt.Sprintf("identity authentication module: %v", err))
		}
		secretBox, secretErr := authentication.NewSecretBox([]byte(globalSecret))
		if secretErr != nil {
			panic(fmt.Sprintf("identity TOTP secret encryption: %v", secretErr))
		}
		recoveryCodes, recoveryErr := authentication.NewRecoveryCodeCodec([]byte(globalSecret))
		if recoveryErr != nil {
			panic(fmt.Sprintf("identity recovery-code codec: %v", recoveryErr))
		}
		totpVerifier, totpErr := authentication.NewTOTPVerifier(
			g.Cfg().MustGet(ctx, "mfa.totp.issuer", "月离账户").String(),
		)
		if totpErr != nil {
			panic(fmt.Sprintf("identity TOTP verifier: %v", totpErr))
		}
		if err := authenticationModule.ConfigureMFA(
			repositories.mfa, totpVerifier, secretBox, recoveryCodes,
		); err != nil {
			panic(fmt.Sprintf("identity MFA module: %v", err))
		}
	}
	var master *sql.DB
	if !openAPIExport {
		master, err = g.DB().Master()
		if err != nil {
			panic(fmt.Sprintf("identity database: %v", err))
		}
	}
	if !openAPIExport && g.Cfg().MustGet(ctx, "maintenance.securityRetention.enabled", true).Bool() {
		interval := g.Cfg().MustGet(ctx, "maintenance.securityRetention.interval", "1h").Duration()
		retention := g.Cfg().MustGet(ctx, "maintenance.securityRetention.retention", "24h").Duration()
		batchSize := g.Cfg().MustGet(ctx, "maintenance.securityRetention.batchSize", 500).Int()
		if interval <= 0 {
			panic("maintenance.securityRetention.interval must be positive")
		}
		cleaner := identitymaintenance.SecurityRetention{
			DB: master, Retention: retention, BatchSize: batchSize,
		}
		maintenanceContext, stopMaintenance := context.WithCancel(ctx)
		defer stopMaintenance()
		go func() {
			run := func() {
				result, cleanupErr := cleaner.RunOnce(maintenanceContext)
				if cleanupErr != nil {
					if !errors.Is(cleanupErr, context.Canceled) {
						g.Log().Warningf(maintenanceContext, "identity security retention cleanup: %v", cleanupErr)
					}
					return
				}
				if result.Ceremonies+result.Transactions+result.PendingTOTP+result.ProofUses+result.BindingAttempts > 0 {
					g.Log().Infof(
						maintenanceContext,
						"identity security retention cleanup: ceremonies=%d transactions=%d pending_totp=%d proof_uses=%d github_binding_attempts=%d",
						result.Ceremonies, result.Transactions, result.PendingTOTP, result.ProofUses, result.BindingAttempts,
					)
				}
			}
			run()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-maintenanceContext.Done():
					return
				case <-ticker.C:
					run()
				}
			}
		}()
	}

	// PAT HMAC secret: warn at startup when falling back to the insecure dev key.
	cfg.PATHMACSecret = g.Cfg().MustGet(ctx, "pat.hmacSecret").String()
	if cfg.PATHMACSecret == "" {
		g.Log().Warning(ctx, "pat.hmacSecret not set; using insecure dev fallback key")
	}
	if maxPerUser := g.Cfg().MustGet(ctx, "pat.maxPerUser", 0).Int(); maxPerUser > 0 {
		cfg.PATMaxPerUser = maxPerUser
	}

	svc := logic.New(repositories.store, cfg)
	if authenticationModule != nil {
		svc.SetSecondFactorGate(authenticationModule)
	}
	var (
		challengeDefinition *foundationabuse.ChallengeDefinition
		challengeVerifiers  map[foundationabuse.ChallengeKind]foundationabuse.ChallengeVerifier
	)
	turnstileSecret := g.Cfg().MustGet(ctx, "abuse.turnstile.secret").String()
	if turnstileSecret != "" && !openAPIExport {
		hostnames := g.Cfg().MustGet(ctx, "abuse.turnstile.hostnames").Strings()
		if len(hostnames) == 0 {
			panic("abuse.turnstile.hostnames is required when Turnstile is enabled")
		}
		verifier, err := turnstile.New(turnstile.Options{
			Secret:   turnstileSecret,
			Endpoint: g.Cfg().MustGet(ctx, "abuse.turnstile.endpoint").String(),
		})
		if err != nil {
			panic(fmt.Sprintf("identity abuse Turnstile verifier: %v", err))
		}
		challengeDefinition = &foundationabuse.ChallengeDefinition{
			Kind: "turnstile", ExpectedAction: "identity-login",
			AllowedHosts: hostnames,
		}
		challengeVerifiers = map[foundationabuse.ChallengeKind]foundationabuse.ChallengeVerifier{
			"turnstile": verifier,
		}
	}
	abuseCatalog := foundationabuse.MustCompile(identityabuse.Definition(identityabuse.Policy{
		LoginAccountCapacity: int64(cfg.LoginMaxFails),
		LoginNetworkCapacity: int64(cfg.IPMaxFails),
		LoginWindow:          cfg.LoginFailWindow,
		Challenge:            challengeDefinition,
	}))
	var (
		abuseModule foundationabuse.Module
		abuseErr    error
	)
	if openAPIExport {
		abuseModule, abuseErr = foundationabuse.NewMemory(abuseCatalog, foundationabuse.MemoryOptions{
			Secret:    []byte("identity-openapi-abuse-memory-secret"),
			Verifiers: challengeVerifiers,
		})
	} else {
		abuseModule, abuseErr = foundationabuse.NewPostgres(ctx, abuseCatalog, foundationabuse.PostgresOptions{
			DB: master, InstanceKey: "identity", Verifiers: challengeVerifiers,
		})
	}
	if abuseErr != nil {
		panic(fmt.Sprintf("identity abuse module: %v", abuseErr))
	}
	svc.SetAbuseModule(abuseModule)
	var privacyOwner privacy.OwnerHost
	var privacyService controller.PrivacyService
	if !openAPIExport {
		workCatalog, err := work.Compile(identityprivacy.WorkDefinition())
		if err != nil {
			panic(fmt.Sprintf("identity privacy work catalog: %v", err))
		}
		workAdapter, err := workpostgres.New(ctx, workCatalog, workpostgres.Options{
			DB: master, InstanceKey: "identity:privacy",
		})
		if err != nil {
			panic(fmt.Sprintf("identity privacy work: %v", err))
		}
		var ownerConfigs []privacyOwnerConfig
		if err := g.Cfg().MustGet(ctx, "privacy.owners").Scan(&ownerConfigs); err != nil {
			panic(fmt.Sprintf("identity privacy owners: %v", err))
		}
		remoteOwners := map[privacy.OwnerKey]privacy.OwnerHost{}
		seenOwners := map[privacy.OwnerKey]struct{}{}
		var configuredOwners []privacy.OwnerDefinition
		for _, configured := range ownerConfigs {
			owner := privacy.OwnerKey(configured.Key)
			if _, exists := seenOwners[owner]; exists {
				panic(fmt.Sprintf("identity privacy owner %q is duplicated", owner))
			}
			seenOwners[owner] = struct{}{}
			switch configured.Kind {
			case "blog":
				configuredOwners = append(configuredOwners, privacycatalog.BlogFor(owner))
			case "notification":
				if owner != privacycatalog.NotificationOwner {
					panic(fmt.Sprintf("identity privacy notification owner key must be %q", privacycatalog.NotificationOwner))
				}
				configuredOwners = append(configuredOwners, privacycatalog.Notification())
			default:
				panic(fmt.Sprintf("identity privacy owner %q has unsupported kind %q", owner, configured.Kind))
			}
			if configured.URL == "" {
				continue
			}
			tokenSource := &privacyhttp.ClientCredentialsTokenSource{
				TokenURL: g.Cfg().MustGet(ctx, "privacy.tokenUrl").String(),
				ClientID: configured.ClientID, ClientSecret: configured.ClientSecret,
				Scope: "privacy:owner",
			}
			client, err := privacyadapter.NewClient(privacyadapter.ClientOptions{
				Endpoint: configured.URL, TokenSource: tokenSource.Token,
				AllowInsecureHTTP: configured.AllowInsecureHTTP,
			})
			if err != nil {
				panic(fmt.Sprintf("identity privacy owner %s: %v", owner, err))
			}
			remoteOwners[owner] = client
		}
		identityPrivacy, err := identityprivacy.NewPostgres(ctx, identityprivacy.Options{
			DB: master, InstanceKey: "identity", Remote: remoteOwners,
			Owners: configuredOwners, Work: workAdapter,
		})
		if err != nil {
			panic(fmt.Sprintf("identity privacy: %v", err))
		}
		privacyService = identityPrivacy
		privacyOwner = identityPrivacy.OwnerHost()
		workerID := "identity-privacy-worker"
		if hostname, hostnameErr := os.Hostname(); hostnameErr == nil && hostname != "" {
			workerID += ":" + hostname
		}
		runner, err := work.NewRunner(
			workCatalog, workAdapter,
			map[work.Kind]work.Handler{identityprivacy.DriveKind: identityPrivacy.WorkHandler()},
			work.RunnerOptions{
				WorkerID: workerID, PollInterval: time.Second,
				OnError: func(err error) {
					g.Log().Warningf(ctx, "identity privacy work runner error: %v", err)
				},
			},
		)
		if err != nil {
			panic(fmt.Sprintf("identity privacy runner: %v", err))
		}
		workerContext, stopWorker := context.WithCancel(ctx)
		defer stopWorker()
		go func() {
			if err := runner.Run(workerContext); err != nil && !errors.Is(err, context.Canceled) {
				g.Log().Errorf(ctx, "identity privacy work runner stopped: %v", err)
			}
		}()
	}
	// OIDC and step-up proofs share the same rotating RS256 trust root.
	mgr, err := oidc.NewManager(ctx, repositories.signingKeys)
	if err != nil {
		panic(fmt.Sprintf("oidc.NewManager: %v", err))
	}
	identityAudience := g.Cfg().MustGet(ctx, "oidc.audience", "identity-api").String()
	identityVerifier, err := foundationauth.NewVerifier(foundationauth.Config{
		Keys: mgr, Issuer: issuer, Audiences: []string{identityAudience},
	})
	if err != nil {
		panic(fmt.Sprintf("identity capability token verifier: %v", err))
	}
	var adminStepUpVerifier *stepup.Verifier
	if !openAPIExport {
		adminStepUpVerifier, err = stepup.New(stepup.Config{
			Keys: mgr, Issuer: issuer, Audience: identityAudience,
			Replay: stepup.PostgreSQLReplayStore{DB: master},
		})
		if err != nil {
			panic(fmt.Sprintf("identity admin step-up verifier: %v", err))
		}
	}
	authCtl := controller.NewPrivacyAware(
		svc, authenticationModule, secureCookie, privacyService,
		cfg.SessionIdleTTL, adminStepUpVerifier,
	)

	var publisherKeys publisher.KeyProvider
	var publisherManager publisher.ManagedKeyProvider
	var publisherTrust *publisher.TrustState
	var publisherRoots []publisher.TrustRoot
	if openAPIExport {
		publisherKeys, err = publisher.NewLocalKeyProvider()
		if err == nil {
			root, rootErr := publisher.NewOfflineRoot()
			if rootErr != nil {
				err = rootErr
			} else {
				manifest, signErr := publisher.SignTrustManifest(ctx, publisher.TrustManifest{
					Schema: publisher.TrustManifestSchema, ManifestVersion: 1,
					Issuer: issuer, IssuedAt: time.Now().UTC(),
					PolicyVersion: "publisher-attestation/v1",
					Keys:          publisherKeys.VerificationKeys(),
				}, root)
				if signErr != nil {
					err = signErr
				} else {
					verified, verifyErr := publisher.VerifyTrustManifest(
						ctx, manifest, []publisher.TrustRoot{root.TrustRoot()},
					)
					if verifyErr != nil {
						err = verifyErr
					} else {
						publisherTrust = publisher.NewTrustState(verified)
						publisherRoots = []publisher.TrustRoot{root.TrustRoot()}
					}
				}
			}
		}
	} else {
		switch mode := g.Cfg().MustGet(ctx, "publisher.mode").String(); mode {
		case "local-file":
			ring, ringErr := publisher.LoadOrCreateLocalKeyRing(
				g.Cfg().MustGet(ctx, "publisher.keyRingFile").String(),
			)
			publisherKeys, publisherManager, err = ring, ring, ringErr
		case "secret-pem":
			publisherKeys, err = publisher.NewSecretPEMKeyProvider(
				g.Cfg().MustGet(ctx, "publisher.privateKeyPem").String(),
			)
		default:
			err = fmt.Errorf(
				"publisher key provider mode %q is not configured; local-file must be explicit",
				mode,
			)
		}
	}
	if err != nil {
		panic(fmt.Sprintf("identity publisher key provider: %v", err))
	}
	if !openAPIExport {
		activeKeyID, keyErr := publisherKeys.KeyID()
		if keyErr != nil && !errors.Is(keyErr, publisher.ErrSigningUnavailable) {
			panic(fmt.Sprintf("identity publisher active key: %v", keyErr))
		}
		root, rootErr := publisher.ReadTrustRoot(
			g.Cfg().MustGet(ctx, "publisher.trustRootFile").String(),
		)
		if rootErr != nil {
			panic(fmt.Sprintf("identity publisher trust root: %v", rootErr))
		}
		verified, trustErr := publisher.ReadTrustManifest(
			g.Cfg().MustGet(ctx, "publisher.trustManifestFile").String(),
			g.Cfg().MustGet(ctx, "publisher.trustRootFile").String(),
			issuer,
			activeKeyID,
			g.Cfg().MustGet(ctx, "publisher.trustManifestMinVersion", 1).Uint64(),
		)
		if trustErr != nil {
			panic(fmt.Sprintf("identity publisher trust manifest: %v", trustErr))
		}
		if publisherManager != nil {
			if applyErr := publisherManager.ApplyTrustManifest(ctx, verified); applyErr != nil {
				panic(fmt.Sprintf("identity publisher key ring reconciliation: %v", applyErr))
			}
		}
		publisherTrust = publisher.NewTrustState(verified)
		publisherRoots = []publisher.TrustRoot{root}
	}
	var publisherConsumerConfigs []publisherConsumerConfig
	if err := g.Cfg().MustGet(ctx, "publisher.consumers").Scan(&publisherConsumerConfigs); err != nil {
		panic(fmt.Sprintf("identity publisher consumers: %v", err))
	}
	publisherConsumers := make([]publisher.Consumer, 0, len(publisherConsumerConfigs))
	for _, configured := range publisherConsumerConfigs {
		artifactKinds := make(map[string]publisher.ArtifactPolicy, len(configured.ArtifactKinds))
		for kind, mediaType := range configured.ArtifactKinds {
			artifactKinds[kind] = publisher.ArtifactPolicy{MediaType: mediaType}
		}
		publisherConsumers = append(publisherConsumers, publisher.Consumer{
			Audience: configured.Audience, Instance: configured.Instance,
			Disabled: !configured.Enabled, ArtifactKinds: artifactKinds,
		})
	}
	publisherModule, err := publisher.New(publisher.Config{
		Issuer: issuer, Consumers: publisherConsumers,
		Store: repositories.publisherStore, Signer: publisherKeys,
	})
	if err != nil {
		panic(fmt.Sprintf("identity publisher module: %v", err))
	}
	publisherCtl := controller.NewPublisher(svc, publisherModule, publisherKeys, publisherTrust)
	var publisherKeyAdmin *publisher.KeyAdministration
	if publisherManager != nil {
		publisherKeyAdmin, err = publisher.NewKeyAdministration(
			publisherManager, publisherTrust, publisherRoots,
			g.Cfg().MustGet(ctx, "publisher.trustManifestFile").String(),
		)
		if err != nil {
			panic(fmt.Sprintf("identity publisher key administration: %v", err))
		}
	}
	publisherAdminCtl := controller.NewPublisherAdmin(authCtl, svc, publisherKeyAdmin)

	// ── GitHub publisher identity binding ───────────────────────────────────
	// This is intentionally separate from credentials_oauth. The user token is
	// used only for authenticated GET /user, revoked best-effort, and never
	// persisted. A configured provider must also configure revocation webhooks.
	githubClientID := g.Cfg().MustGet(ctx, "githubBinding.clientId").String()
	githubClientSecret := g.Cfg().MustGet(ctx, "githubBinding.clientSecret").String()
	githubRedirectURL := g.Cfg().MustGet(ctx, "githubBinding.redirectUrl").String()
	githubWebhookSecret := g.Cfg().MustGet(ctx, "githubBinding.webhookSecret").String()
	var (
		githubProvider *githubbinding.GitHubApp
		githubModule   *githubbinding.Module
	)
	githubConfigured := githubClientID != "" || githubClientSecret != "" ||
		githubRedirectURL != "" || githubWebhookSecret != ""
	if githubConfigured {
		if githubClientID == "" || githubClientSecret == "" ||
			githubRedirectURL == "" || githubWebhookSecret == "" {
			panic("githubBinding clientId, clientSecret, redirectUrl, and webhookSecret must be configured together")
		}
		githubProvider, err = githubbinding.NewGitHubApp(
			githubClientID, githubClientSecret, githubRedirectURL,
		)
		if err != nil {
			panic(fmt.Sprintf("identity GitHub binding provider: %v", err))
		}
		githubModule, err = githubbinding.New(githubbinding.Config{
			Store: repositories.githubStore, Provider: githubProvider,
			CipherSecret: []byte(globalSecret),
			AttemptTTL: g.Cfg().MustGet(
				ctx, "githubBinding.attemptTtl", "10m",
			).Duration(),
		})
		if err != nil {
			panic(fmt.Sprintf("identity GitHub binding module: %v", err))
		}
	}
	githubCtl := controller.NewGitHubBinding(
		svc, githubModule, []byte(githubWebhookSecret), "/",
	)
	githubSubmissionCtl := controller.NewGitHubSubmission(
		githubModule, publisherTrust, issuer, svc,
	)
	var githubChecker identitycap.HealthChecker
	if githubProvider != nil {
		githubChecker = githubProvider
	}

	// ── Bootstrap admin (RBAC) ───────────────────────────────────────────────
	// If rbac.bootstrapAdminEmail is set and that identity already exists, grant
	// it the admin role. Best-effort + idempotent (GrantRole is a no-op on a
	// repeat grant); silent if the identity hasn't registered yet.
	if bootstrapEmail := g.Cfg().MustGet(ctx, "rbac.bootstrapAdminEmail").String(); !openAPIExport && bootstrapEmail != "" {
		id, err := svc.GetByEmail(ctx, bootstrapEmail)
		switch {
		case errors.Is(err, repo.ErrIdentityMissing):
			g.Log().Infof(ctx, "bootstrap admin: identity %q not found yet, skipping", bootstrapEmail)
		case err != nil:
			g.Log().Warningf(ctx, "bootstrap admin: lookup of %q failed: %v", bootstrapEmail, err)
		default:
			if err := svc.GrantRole(ctx, id.ID, logic.AdminRole); err != nil {
				g.Log().Warningf(ctx, "bootstrap admin: grant to %q failed: %v", bootstrapEmail, err)
			} else {
				g.Log().Infof(ctx, "bootstrap admin: granted %q to %q", logic.AdminRole, bootstrapEmail)
			}
		}
	}

	// ── Mailer ──────────────────────────────────────────────────────────────
	// Production delivery goes through Notification. Local development keeps a
	// log-only fallback when client credentials have not been provisioned.
	devMailer := mailer.NewDev()
	var mlr mailer.Mailer = devMailer
	var securityNotifier mailer.SecurityNotifier = devMailer
	var notificationHealth identitycap.HealthChecker = devMailer
	notificationClientConfig := notificationclient.Config{
		BaseURL:      notificationBaseURL,
		TokenURL:     g.Cfg().MustGet(ctx, "notification.tokenUrl").String(),
		ClientID:     g.Cfg().MustGet(ctx, "notification.clientId").String(),
		ClientSecret: g.Cfg().MustGet(ctx, "notification.clientSecret").String(),
		Scope:        g.Cfg().MustGet(ctx, "notification.scope", "notification:send").String(),
	}
	notificationConfigured := notificationClientConfig.BaseURL != "" && notificationClientConfig.TokenURL != "" &&
		notificationClientConfig.ClientID != "" && notificationClientConfig.ClientSecret != ""
	if notificationConfigured {
		client, err := notificationclient.New(notificationClientConfig)
		if err != nil {
			panic(fmt.Sprintf("notification client: %v", err))
		}
		notificationMailer := mailer.NewNotification(client)
		mlr = notificationMailer
		securityNotifier = notificationMailer
		notificationHealth = client
	} else {
		g.Log().Warning(ctx, "identity notification OAuth config is incomplete; using dev-log mailer")
	}
	svc.SetMailer(mlr)
	if authenticationModule != nil {
		authenticationModule.SetSecurityEventSink(identitysecurity.New(
			repositories.audit, repositories.store, securityNotifier, accountBaseURL,
		))
	}

	// ── OIDC / OAuth2 ───────────────────────────────────────────────────────
	stepUpCtl := controller.NewStepUp(svc, authenticationModule, mgr, issuer)
	// Durable PG-backed OIDC store: persists OAuth requests and refresh tokens
	// across restarts. Access tokens remain stateless JWTs.
	oidcStore := oidc.NewStore(repositories.oidcBackend, repositories.clients)
	// Wire passive logout: logic.Service calls RevokeRefreshBySession on logout,
	// which revokes all refresh tokens belonging to that identity session.
	svc.SetRefreshRevoker(oidcStore)
	refreshTTL := g.Cfg().MustGet(ctx, "oidc.refreshTtl", "720h").Duration()
	provider := oidc.NewProvider(oidcStore, oidc.Config{
		Issuer:       issuer,
		GlobalSecret: []byte(globalSecret),
		AccessTTL:    10 * time.Minute,
		IDTTL:        10 * time.Minute,
		RefreshTTL:   refreshTTL,
	}, mgr.KeyGetter)
	oidcCtl := controller.NewOIDC(
		provider, mgr, svc, repositories.clients, issuer, loginURL,
		secureCookie, []byte(globalSecret),
	)
	guestCtl := controller.NewGuest(guest.New(repositories.guestStore, repositories.clients, mgr, guest.Config{
		Issuer:         issuer,
		MaxSessionTTL:  g.Cfg().MustGet(ctx, "guest.maxSessionTtl", "720h").Duration(),
		AccessTokenTTL: g.Cfg().MustGet(ctx, "guest.accessTokenTtl", "10m").Duration(),
	}))

	// ── Google OAuth login ──────────────────────────────────────────────────
	// Provider stays nil when credentials are unconfigured; the controller then
	// redirects the start/callback endpoints to login with ?error=oauth_unavailable.
	googleClientID := g.Cfg().MustGet(ctx, "oidc.google.clientId").String()
	googleClientSecret := g.Cfg().MustGet(ctx, "oidc.google.clientSecret").String()
	googleRedirectURL := g.Cfg().MustGet(ctx, "oidc.google.redirectUrl").String()
	var googleProvider oauthlogin.Provider
	if googleClientID != "" && googleClientSecret != "" {
		googleProvider = oauthlogin.NewGoogle(googleClientID, googleClientSecret, googleRedirectURL)
	}
	oauthCtl := controller.NewOAuth(svc, googleProvider, secureCookie, []byte(globalSecret), loginURL, cfg.SessionIdleTTL)

	coreKeys := []string{"identity.oidc", "identity.pat", "identity.profile", "identity.publisher-attestation", "identity.user-admin"}
	mailKeys := []string{"identity.reset-password", "identity.verify-email"}
	var googleChecker identitycap.HealthChecker
	if googleProvider != nil {
		googleChecker, _ = googleProvider.(identitycap.HealthChecker)
	}
	capabilityRegistry, err := identitycap.New(
		identitycap.Registration{
			Key: "identity-jwks", Adapter: "builtin", Mode: "in-memory", Registered: true, Enabled: true,
			CapabilityKeys: []string{"identity.jwks"}, Operations: []string{"get"},
			Checker: identitycap.HealthCheckFunc(func(context.Context) error { return nil }), InitialHealth: capability.HealthHealthy,
		},
		identitycap.Registration{
			Key: "identity-core", Adapter: "builtin", Mode: "first-party", Registered: true, Enabled: true,
			CapabilityKeys: coreKeys, Operations: []string{"admin", "authenticate", "authorize", "issue", "profile", "verify"},
			RequiredConfig: []capability.ConfigField{identitycap.Field("issuer", issuer, false), identitycap.Field("global_secret", globalSecret, true)},
			Checker: identitycap.HealthCheckFunc(func(ctx context.Context) error {
				return errors.Join(healthcheck.Database(ctx), healthcheck.Redis(ctx))
			}),
		},
		identitycap.Registration{
			Key: "dev-mail", Adapter: "dev", Mode: "development", Registered: true, Enabled: !notificationConfigured,
			CapabilityKeys: mailKeys, Operations: []string{"send"}, Checker: devMailer, InitialHealth: capability.HealthHealthy,
		},
		identitycap.Registration{
			Key: "notification-service", Adapter: "notification-http", Mode: "platform", Registered: notificationConfigured, Enabled: notificationConfigured,
			CapabilityKeys: mailKeys, Operations: []string{"enqueue"}, Checker: notificationHealth,
			RequiredConfig: []capability.ConfigField{
				identitycap.Field("base_url", notificationClientConfig.BaseURL, false),
				identitycap.Field("token_url", notificationClientConfig.TokenURL, false),
				identitycap.Field("client_id", notificationClientConfig.ClientID, false),
				identitycap.Field("client_secret", notificationClientConfig.ClientSecret, true),
			},
		},
		identitycap.Registration{
			Key: "google", Adapter: "google-oauth", Mode: "external", Registered: googleProvider != nil, Enabled: googleProvider != nil,
			CapabilityKeys: []string{"identity.external-login"}, Operations: []string{"authorize", "callback", "link", "unlink"}, Checker: googleChecker,
			RequiredConfig: []capability.ConfigField{
				identitycap.Field("client_id", googleClientID, false), identitycap.Field("client_secret", googleClientSecret, true),
				identitycap.Field("redirect_url", googleRedirectURL, false),
			},
		},
		identitycap.Registration{
			Key: "github-binding", Adapter: "github-app-user-authorization", Mode: "external",
			Registered: githubModule != nil, Enabled: githubModule != nil,
			CapabilityKeys: []string{"identity.github-binding"},
			Operations:     []string{"authorize", "authorize_submission", "bind", "list", "unlink", "revoke"},
			Checker:        githubChecker,
			RequiredConfig: []capability.ConfigField{
				identitycap.Field("client_id", githubClientID, false),
				identitycap.Field("client_secret", githubClientSecret, true),
				identitycap.Field("redirect_url", githubRedirectURL, false),
				identitycap.Field("webhook_secret", githubWebhookSecret, true),
			},
		},
	)
	if err != nil {
		panic(fmt.Sprintf("identity capability registry: %v", err))
	}
	capabilityCtl := controller.NewCapability(authCtl, capabilityRegistry, repositories.audit, identitycap.ServiceMetadata())

	// Avatar/cover upload proxy: the IdP drives the asset upload server-side on
	// behalf of the cookie-authenticated caller (mints a short-lived user token).
	avatarCtl := controller.NewAvatar(svc, mgr, assetclient.New(assetBaseURL), issuer, assetAudience)
	assetAdminProxy := controller.NewAssetAdminProxy(authCtl, mgr, issuer, assetBaseURL, assetAudience)
	capabilityTargets := map[string]controller.CapabilityProxyTarget{
		"asset": {BaseURL: assetBaseURL, Audience: assetAudience},
	}
	if commerceBaseURL != "" || commerceAudience != "" {
		capabilityTargets["commerce"] = controller.CapabilityProxyTarget{BaseURL: commerceBaseURL, Audience: commerceAudience}
	}
	if notificationBaseURL != "" || notificationAudience != "" {
		capabilityTargets["notification"] = controller.CapabilityProxyTarget{BaseURL: notificationBaseURL, Audience: notificationAudience}
	}
	platformCapabilityProxy, err := controller.NewPlatformCapabilityProxy(authCtl, mgr, issuer, capabilityTargets)
	if err != nil {
		panic(fmt.Sprintf("platform capability proxy: %v", err))
	}

	// ── Routing ─────────────────────────────────────────────────────────────
	s := g.Server()
	s.GetOpenApi().Components.SecuritySchemes = goai.SecuritySchemes{
		"AdminAuth": {Value: &goai.SecurityScheme{Type: "http", Scheme: "bearer", BearerFormat: "JWT", Description: "Admin session cookie or an Identity access token with platform capability scope."}},
		"MachineAuth": {Value: &goai.SecurityScheme{Type: "http", Scheme: "bearer", BearerFormat: "JWT", Description: "Identity-issued service access token with the endpoint scope."}},
		"UserAuth":  {Value: &goai.SecurityScheme{Type: "http", Scheme: "bearer", BearerFormat: "JWT", Description: "Identity user access token."}},
	}
	s.Use(ghttpx.TraceRouteMiddleware)

	// Actor middleware runs globally (before all handlers) so that every
	// request — business API, OIDC endpoints, and OAuth login callbacks —
	// has IP / User-Agent / X-Request-Id available via actor.From(ctx).
	s.Use(controller.ActorMiddleware)
	rateLimiter := ghttpx.MustRateLimiterFromEnvironment()
	apiMiddleware := ghttpx.NewMiddleware(rateLimiter, ghttpx.ForwardedClientIPKey)
	rawRateLimitMiddleware := ghttpx.NewRawRateLimitMiddleware(rateLimiter, ghttpx.ForwardedClientIPKey)

	// Business API: raw-success/Problem middleware applied to this group only.
	s.Group("/", func(grp *ghttp.RouterGroup) {
		grp.Middleware(apiMiddleware, authhttp.Optional(identityVerifier))
		grp.GET("/healthz", controller.Healthz)
		grp.GET("/readyz", healthcheck.Handler(map[string]healthcheck.Check{
			"database": healthcheck.Database,
			"redis":    healthcheck.Redis,
		}))
		grp.Bind(authCtl)
		grp.Bind(stepUpCtl)
		grp.Bind(avatarCtl)
		grp.Bind(capabilityCtl)
		grp.Bind(guestCtl)
		grp.Bind(publisherCtl)
		grp.Bind(publisherAdminCtl)
		grp.Bind(githubCtl)
		grp.Bind(githubSubmissionCtl)
		if privacyOwner != nil {
			grp.POST("/api/internal/privacy/owner", privacyhttp.OwnerHandler(privacyOwner, "privacy:owner"))
		}
		grp.ALL("/api/v1/admin/assets-proxy/*", assetAdminProxy.Forward)
		grp.ALL("/api/v1/admin/platform-proxy/*", platformCapabilityProxy.Forward)
	})

	// OIDC standard endpoints: raw RFC responses — NO envelope middleware.
	s.Group("/", func(grp *ghttp.RouterGroup) {
		grp.Middleware(rawRateLimitMiddleware)
		grp.GET("/.well-known/openid-configuration", oidcCtl.Discovery)
		grp.GET("/oauth2/jwks.json", oidcCtl.JWKS)
		grp.GET("/oauth2/authorize", oidcCtl.Authorize)
		grp.POST("/oauth2/token", oidcCtl.Token)
		grp.ALL("/oauth2/userinfo", oidcCtl.Userinfo)
		grp.POST("/oauth2/revoke", oidcCtl.Revoke)         // RFC 7009 token revocation
		grp.ALL("/oauth2/end_session", oidcCtl.EndSession) // OIDC RP-initiated logout (MVP)
	})

	// External OAuth callbacks and GitHub webhook: raw protocol responses.
	s.Group("/", func(grp *ghttp.RouterGroup) {
		grp.Middleware(rawRateLimitMiddleware)
		grp.GET("/api/v1/auth/oauth/google/start", oauthCtl.GoogleStart)
		grp.GET("/api/v1/auth/oauth/google/callback", oauthCtl.GoogleCallback)
		grp.GET("/api/v1/account/github-bindings/callback", githubCtl.GitHubCallback)
		grp.POST("/api/v1/webhooks/github", githubCtl.GitHubWebhook)
	})

	if handled, err := openapiexport.ExportIfRequested(s); handled {
		if err != nil {
			panic(err)
		}
		return
	}

	g.Log().Info(ctx, "identity-service starting")
	s.Run()
}
