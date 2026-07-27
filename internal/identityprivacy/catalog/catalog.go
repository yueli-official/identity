// Package catalog defines the deployment-local privacy vocabulary coordinated
// by Identity. Storage and execution remain owned by each participating service.
package catalog

import (
	"time"

	"github.com/yueli-official/foundation/go/privacy"
)

const (
	IdentityOwner     privacy.OwnerKey = "identity"
	BlogOwner         privacy.OwnerKey = "blog"
	NotificationOwner privacy.OwnerKey = "notification"

	UserSubject       privacy.SubjectKind = "user"
	SubscriberSubject privacy.SubjectKind = "subscriber"

	AccountContact   privacy.DataCategoryKey = "account.contact"
	AccountProfile   privacy.DataCategoryKey = "account.profile"
	Credential       privacy.DataCategoryKey = "account.credential"
	SecurityEvidence privacy.DataCategoryKey = "security.evidence"
	PublicContent    privacy.DataCategoryKey = "content.public"
	CommentContact   privacy.DataCategoryKey = "comment.contact"
	NetworkSecurity  privacy.DataCategoryKey = "network.security"
	PublicAuthorship privacy.DataCategoryKey = "content.authorship"
	PrivateReaction  privacy.DataCategoryKey = "content.reaction"
	MarketingContact privacy.DataCategoryKey = "marketing.contact"
	DeliveryEvidence privacy.DataCategoryKey = "delivery.evidence"

	IdentityAccountDataset  privacy.DatasetKey = "identity.account"
	IdentityAuditDataset    privacy.DatasetKey = "identity.security_audit"
	BlogNewsletterDataset   privacy.DatasetKey = "blog.newsletter"
	BlogCommentsDataset     privacy.DatasetKey = "blog.comments"
	BlogAuthorshipDataset   privacy.DatasetKey = "blog.authorship"
	BlogReactionsDataset    privacy.DatasetKey = "blog.reactions"
	NotificationPrefs       privacy.DatasetKey = "notification.preferences"
	NotificationSuppression privacy.DatasetKey = "notification.suppressions"
	NotificationHistory     privacy.DatasetKey = "notification.delivery_history"

	NotificationDeliveryRetention privacy.RetentionRuleKey = "notification.delivery_review"
)

func SubjectKinds() []privacy.SubjectKindDefinition {
	return []privacy.SubjectKindDefinition{
		{Key: UserSubject, Description: "Identity user identifier", MaxRefBytes: 128},
		{Key: SubscriberSubject, Description: "Normalized newsletter address", MaxRefBytes: 320},
	}
}

func Categories() []privacy.DataCategoryDefinition {
	return []privacy.DataCategoryDefinition{
		{Key: AccountContact, Description: "Account contact identifiers"},
		{Key: AccountProfile, Description: "Public and private account profile"},
		{Key: Credential, Description: "Authentication credentials", Sensitive: true},
		{Key: SecurityEvidence, Description: "Minimum security and audit evidence", Sensitive: true},
		{Key: PublicContent, Description: "User-published content"},
		{Key: CommentContact, Description: "Non-public comment contact data"},
		{Key: NetworkSecurity, Description: "Network and client security metadata", Sensitive: true},
		{Key: PublicAuthorship, Description: "Public authorship attribution"},
		{Key: PrivateReaction, Description: "Private reactions and bookmarks"},
		{Key: MarketingContact, Description: "Marketing subscription contact"},
		{Key: DeliveryEvidence, Description: "Message delivery and suppression evidence", Sensitive: true},
	}
}

func Owners(configuredOwners ...privacy.OwnerDefinition) []privacy.OwnerDefinition {
	result := []privacy.OwnerDefinition{Identity()}
	return append(result, configuredOwners...)
}

func Identity() privacy.OwnerDefinition {
	return privacy.OwnerDefinition{
		Ref:                 privacy.OwnerRef{Key: IdentityOwner, Revision: 1},
		SubjectKinds:        []privacy.SubjectKind{UserSubject},
		FinalizeAfterOwners: true,
		Datasets: []privacy.DatasetDefinition{
			{
				Key: IdentityAccountDataset,
				Categories: []privacy.DataCategoryKey{
					AccountContact, AccountProfile, Credential,
				},
				Operations: []privacy.RightsOperation{privacy.RightErasure},
			},
			{
				Key:        IdentityAuditDataset,
				Categories: []privacy.DataCategoryKey{SecurityEvidence},
				Operations: []privacy.RightsOperation{privacy.RightErasure},
			},
		},
	}
}

func Blog() privacy.OwnerDefinition {
	return BlogFor(BlogOwner)
}

func BlogFor(ownerKey privacy.OwnerKey) privacy.OwnerDefinition {
	return privacy.OwnerDefinition{
		Ref:          privacy.OwnerRef{Key: ownerKey, Revision: 1},
		SubjectKinds: []privacy.SubjectKind{UserSubject, SubscriberSubject},
		Datasets: []privacy.DatasetDefinition{
			{
				Key:        BlogNewsletterDataset,
				Categories: []privacy.DataCategoryKey{MarketingContact},
				Operations: []privacy.RightsOperation{privacy.RightErasure},
			},
			{
				Key:        BlogCommentsDataset,
				Categories: []privacy.DataCategoryKey{PublicContent, CommentContact, NetworkSecurity},
				Operations: []privacy.RightsOperation{privacy.RightErasure},
			},
			{
				Key:        BlogAuthorshipDataset,
				Categories: []privacy.DataCategoryKey{PublicAuthorship, PublicContent},
				Operations: []privacy.RightsOperation{privacy.RightErasure},
			},
			{
				Key:        BlogReactionsDataset,
				Categories: []privacy.DataCategoryKey{PrivateReaction},
				Operations: []privacy.RightsOperation{privacy.RightErasure},
			},
		},
	}
}

func Notification() privacy.OwnerDefinition {
	return privacy.OwnerDefinition{
		Ref:          privacy.OwnerRef{Key: NotificationOwner, Revision: 1},
		SubjectKinds: []privacy.SubjectKind{UserSubject, SubscriberSubject},
		Datasets: []privacy.DatasetDefinition{
			{
				Key:        NotificationPrefs,
				Categories: []privacy.DataCategoryKey{MarketingContact},
				Operations: []privacy.RightsOperation{privacy.RightErasure},
			},
			{
				Key:        NotificationSuppression,
				Categories: []privacy.DataCategoryKey{MarketingContact, DeliveryEvidence},
				Operations: []privacy.RightsOperation{privacy.RightErasure},
			},
			{
				Key:        NotificationHistory,
				Categories: []privacy.DataCategoryKey{DeliveryEvidence, AccountContact},
				Operations: []privacy.RightsOperation{privacy.RightErasure},
				RetentionRules: []privacy.RetentionRuleRef{{
					Key: NotificationDeliveryRetention, Revision: 1,
				}},
			},
		},
	}
}

func RetentionRules() []privacy.RetentionRuleDefinition {
	return []privacy.RetentionRuleDefinition{{
		Ref:        privacy.RetentionRuleRef{Key: NotificationDeliveryRetention, Revision: 1},
		Categories: []privacy.DataCategoryKey{DeliveryEvidence, AccountContact},
		Trigger:    "notification_created", ReviewAfter: privacy.CalendarPeriod{Days: 90},
		DefaultReviewOutcome: privacy.ReviewOwnerDecision,
	}}
}

func RightsPolicies() []privacy.RightsPolicy {
	operations := []privacy.RightsOperation{privacy.RightErasure}
	result := make([]privacy.RightsPolicy, 0, len(operations))
	for _, operation := range operations {
		result = append(result, privacy.RightsPolicy{
			Operation: operation, RespondWithin: privacy.CalendarPeriod{Months: 1},
			VerificationMaxAge: 24 * time.Hour,
		})
	}
	return result
}
