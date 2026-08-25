package controller

import (
	"context"
	"net/http"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/yueli-official/foundation/go/identifier"
	"github.com/yueli-official/foundation/go/privacy"

	v1 "github.com/yueli-official/identity/api/v1"
	"github.com/yueli-official/identity/internal/authentication"
	"github.com/yueli-official/identity/internal/logic"
	"github.com/yueli-official/identity/stepup"
)

const sessionCookie = "id_session"

// Controller handles HTTP requests for the identity service.
type Controller struct {
	svc          *logic.Service
	secureCookie bool
	sessionTTL   time.Duration
	privacy      PrivacyService
	authn        *authentication.Module
	adminStepUp  *stepup.Verifier
}

type PrivacyService interface {
	OpenErasure(context.Context, string, string, string, privacy.IdempotencyKey, string, time.Time, privacy.VerificationEvidence) (privacy.RightsRequestView, error)
	GetByToken(context.Context, string, privacy.RightsRequestID) (privacy.RightsRequestView, error)
}

// New builds the controller. secureCookie should be true in production (HTTPS);
// tests/local HTTP use false so the session cookie round-trips over plain HTTP.
func New(svc *logic.Service, secureCookie bool, sessionTTL ...time.Duration) *Controller {
	ttl := logic.DefaultConfig().SessionIdleTTL
	if len(sessionTTL) > 0 {
		ttl = sessionTTL[0]
	}
	return &Controller{svc: svc, secureCookie: secureCookie, sessionTTL: ttl}
}

// NewPrivacyAware builds the runtime controller with the coordinator seam.
// Keeping dependency wiring in a constructor prevents GoFrame from treating a
// public setter method as an HTTP action during reflection-based binding.
func NewPrivacyAware(
	svc *logic.Service,
	authn *authentication.Module,
	secureCookie bool,
	privacyService PrivacyService,
	sessionTTL time.Duration,
	adminStepUp *stepup.Verifier,
) *Controller {
	controller := New(svc, secureCookie, sessionTTL)
	controller.privacy = privacyService
	controller.authn = authn
	controller.adminStepUp = adminStepUp
	return controller
}

func (c *Controller) Register(ctx context.Context, req *v1.RegisterReq) (*v1.RegisterRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	attemptID := req.AbuseAttemptID
	if attemptID == "" {
		attemptID = identifier.MustNew().String()
	}
	id, err := c.svc.Register(ctx, logic.RegisterInput{
		Email: req.Email, Password: req.Password, DisplayName: req.DisplayName,
		AttemptID: attemptID, IP: r.GetClientIp(), Proof: req.ChallengeProof,
	})
	if err != nil {
		return nil, err
	}
	return &v1.RegisterRes{UserKey: id.UserKey, Email: id.Email}, nil
}

func (c *Controller) Login(ctx context.Context, req *v1.LoginReq) (*v1.LoginRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	attemptID := req.AbuseAttemptID
	if attemptID == "" {
		attemptID = identifier.MustNew().String()
	}
	out, err := c.svc.Login(ctx, logic.LoginInput{
		Email: req.Email, Password: req.Password,
		UserAgent: r.UserAgent(), IP: r.GetClientIp(),
		AttemptID: attemptID, Proof: req.ChallengeProof,
	})
	if err != nil {
		return nil, err
	}
	if out.MFARequired {
		return &v1.LoginRes{
			UserKey: out.Identity.UserKey, Email: out.Identity.Email, MFARequired: true,
			MFATransaction: out.MFATransaction,
			MFAExpiresAt:   out.MFAExpiresAt.Format(time.RFC3339),
			MFAMethods:     out.MFAMethods,
		}, nil
	}
	c.setSessionCookie(r, out.SessionID)
	return &v1.LoginRes{
		UserKey: out.Identity.UserKey, Email: out.Identity.Email, MFARequired: false,
	}, nil
}

func (c *Controller) Logout(ctx context.Context, _ *v1.LogoutReq) (*v1.LogoutRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	sid := r.Cookie.Get(sessionCookie, "").String()
	if sid != "" {
		if err := c.svc.Logout(ctx, sid); err != nil {
			return nil, err
		}
	}
	r.Cookie.Remove(sessionCookie)
	return &v1.LogoutRes{}, nil
}

func (c *Controller) Reauthenticate(ctx context.Context, req *v1.ReauthenticateReq) (*v1.ReauthenticateRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	sid := r.Cookie.Get(sessionCookie, "").String()
	if err := c.svc.Reauthenticate(ctx, sid, req.Password); err != nil {
		return nil, err
	}
	session, _, err := c.svc.AuthenticatedSession(ctx, sid)
	if err != nil {
		return nil, err
	}
	return &v1.ReauthenticateRes{
		AuthenticatedAt: session.Authentication.AuthenticatedAt.Format(time.RFC3339),
	}, nil
}

// EmailVerifyRequest sends a verify-email link to the logged-in caller. The
// caller is resolved from the session cookie (same seam as Me).
func (c *Controller) EmailVerifyRequest(ctx context.Context, _ *v1.EmailVerifyRequestReq) (*v1.EmailVerifyRequestRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	id, err := c.svc.Me(ctx, r.Cookie.Get(sessionCookie, "").String())
	if err != nil {
		return nil, err
	}
	if err := c.svc.RequestEmailVerification(ctx, id.ID, r.GetClientIp()); err != nil {
		return nil, err
	}
	return &v1.EmailVerifyRequestRes{}, nil
}

// EmailVerify consumes an email-verification token.
func (c *Controller) EmailVerify(ctx context.Context, req *v1.EmailVerifyReq) (*v1.EmailVerifyRes, error) {
	if err := c.svc.VerifyEmail(ctx, req.Token); err != nil {
		return nil, err
	}
	return &v1.EmailVerifyRes{}, nil
}

// PasswordForgot requests a password-reset email. Always returns success to
// avoid account enumeration (the service swallows unknown-email cases).
func (c *Controller) PasswordForgot(ctx context.Context, req *v1.PasswordForgotReq) (*v1.PasswordForgotRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	if err := c.svc.RequestPasswordReset(ctx, req.Email, r.GetClientIp()); err != nil {
		return nil, err
	}
	return &v1.PasswordForgotRes{}, nil
}

// PasswordReset sets a new password via a single-use reset token.
func (c *Controller) PasswordReset(ctx context.Context, req *v1.PasswordResetReq) (*v1.PasswordResetRes, error) {
	if err := c.svc.ResetPassword(ctx, req.Token, req.Password); err != nil {
		return nil, err
	}
	return &v1.PasswordResetRes{}, nil
}

func (c *Controller) PasswordPolicy(
	context.Context,
	*v1.PasswordPolicyReq,
) (*v1.PasswordPolicyRes, error) {
	policy := c.svc.PasswordPolicy()
	return &v1.PasswordPolicyRes{
		MinLength: policy.MinLength, MaxLength: policy.MaxLength,
		Normalization: policy.Normalization, Blocklist: policy.Blocklist,
	}, nil
}

func (c *Controller) setSessionCookie(r *ghttp.Request, sid string) {
	cookie := &http.Cookie{
		Name: sessionCookie, Value: sid, Path: "/",
		HttpOnly: true, Secure: c.secureCookie, SameSite: http.SameSiteLaxMode,
	}
	if c.sessionTTL > 0 {
		cookie.MaxAge = int(c.sessionTTL.Seconds())
		cookie.Expires = time.Now().Add(c.sessionTTL)
	}
	r.Cookie.SetHttpCookie(cookie)
}
