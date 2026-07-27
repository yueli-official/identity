package controller

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	foundationauth "github.com/yueli-official/foundation/go/auth"

	v1 "github.com/yueli-official/identity/api/v1"
	"github.com/yueli-official/identity/internal/githubbinding"
	"github.com/yueli-official/identity/internal/iderr"
	"github.com/yueli-official/identity/internal/logic"
	"github.com/yueli-official/identity/internal/publisher"
)

const githubSubmissionAuthorizeScope = "publisher:github-submission:authorize"

type GitHubSubmissionController struct {
	module  *githubbinding.Module
	trust   *publisher.TrustState
	issuer  string
	service *logic.Service
}

func NewGitHubSubmission(
	module *githubbinding.Module,
	trust *publisher.TrustState,
	issuer string,
	service *logic.Service,
) *GitHubSubmissionController {
	return &GitHubSubmissionController{
		module: module, trust: trust, issuer: issuer, service: service,
	}
}

func (controller *GitHubSubmissionController) AuthorizeGitHubSubmission(
	ctx context.Context,
	req *v1.AuthorizeGitHubSubmissionReq,
) (*v1.AuthorizeGitHubSubmissionRes, error) {
	principal, ok := foundationauth.FromContext(ctx)
	if !ok || principal == nil || !principal.HasScope(githubSubmissionAuthorizeScope) {
		return nil, iderr.Forbidden()
	}
	if controller.module == nil || controller.trust == nil {
		return nil, iderr.GitHubBindingUnavailable()
	}
	manifest := githubbinding.NewSubmissionManifest(
		githubbinding.PublisherAttestationDocument{
			AttestationID:    req.Attestation.AttestationID,
			Issuer:           req.Attestation.Issuer,
			PublisherSubject: req.Attestation.PublisherSubject,
			StatementDigest:  req.Attestation.StatementDigest,
			KeyID:            req.Attestation.KeyID, Envelope: req.Attestation.Envelope,
			IssuedAt: req.Attestation.IssuedAt,
		},
		githubbinding.GitHubProvenance{
			ProviderAccountID:  req.Provenance.ProviderAccountID,
			RepositoryID:       req.Provenance.RepositoryID,
			RepositoryNodeID:   req.Provenance.RepositoryNodeID,
			RepositoryFullName: req.Provenance.RepositoryFullName,
			PullRequestNumber:  req.Provenance.PullRequestNumber,
			HeadCommitSHA:      req.Provenance.HeadCommitSHA,
		},
		time.Now(),
	)
	currentTrust := controller.trust.Current()
	authorized, err := controller.module.AuthorizeSubmission(
		ctx, manifest, githubbinding.SubmissionPolicy{
			Issuer: controller.issuer, Audience: req.Audience,
			ConsumerInstance: req.ConsumerInstance, Namespace: req.Namespace,
			Artifact: publisher.Artifact{
				Kind: req.Artifact.Kind, Identity: req.Artifact.Identity,
				Version: req.Artifact.Version, Name: req.Artifact.Name,
				URI: req.Artifact.URI, MediaType: req.Artifact.MediaType,
				SHA256: req.Artifact.SHA256,
			},
			Keys: currentTrust.Manifest.Keys,
		},
	)
	if err != nil {
		controller.service.RecordGitHubSubmissionDenied(
			ctx, principal.ClientID, githubSubmissionAuditReason(err),
			req.Provenance.ProviderAccountID, req.Provenance.RepositoryID,
			req.Provenance.PullRequestNumber,
		)
		return nil, mapGitHubSubmissionError(err)
	}
	manifestJSON, err := json.Marshal(authorized.Manifest)
	if err != nil {
		return nil, iderr.GitHubSubmissionInvalid()
	}
	controller.service.RecordGitHubSubmissionAuthorized(
		ctx, principal.ClientID, authorized.PublisherSubject,
		authorized.BindingID, authorized.ManifestDigest,
		req.Provenance.RepositoryID, req.Provenance.PullRequestNumber,
		req.Provenance.HeadCommitSHA,
	)
	return &v1.AuthorizeGitHubSubmissionRes{
		Authorized: true, PublisherSubject: authorized.PublisherSubject,
		BindingID:         authorized.BindingID,
		BindingVerifiedAt: authorized.BindingVerifiedAt.Format(time.RFC3339Nano),
		ManifestDigest:    authorized.ManifestDigest, Manifest: manifestJSON,
	}, nil
}

func githubSubmissionAuditReason(err error) string {
	switch {
	case errors.Is(err, githubbinding.ErrBindingInactive):
		return "binding_inactive"
	case errors.Is(err, githubbinding.ErrSubjectMismatch):
		return "publisher_subject_mismatch"
	case errors.Is(err, githubbinding.ErrInvalidSubmission):
		return "manifest_invalid"
	default:
		return "authorization_unavailable"
	}
}

func mapGitHubSubmissionError(err error) error {
	switch {
	case errors.Is(err, githubbinding.ErrBindingInactive),
		errors.Is(err, githubbinding.ErrSubjectMismatch):
		return iderr.GitHubSubmissionUnauthorized()
	case errors.Is(err, githubbinding.ErrInvalidSubmission):
		return iderr.GitHubSubmissionInvalid()
	default:
		return iderr.GitHubBindingUnavailable()
	}
}
