package v1

import (
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

type PasskeyLoginBeginReq struct {
	g.Meta `path:"/api/v1/auth/passkeys/login/begin" method:"post" tags:"passkeys" summary:"Begin discoverable passkey login"`
}

type PasskeyLoginBeginRes struct {
	CeremonyID string          `json:"ceremonyId"`
	ExpiresAt  string          `json:"expiresAt"`
	Options    json.RawMessage `json:"options"`
}

type PasskeyLoginFinishReq struct {
	g.Meta     `path:"/api/v1/auth/passkeys/login/finish" method:"post" tags:"passkeys" summary:"Finish discoverable passkey login"`
	CeremonyID string          `json:"ceremonyId" v:"required"`
	Response   json.RawMessage `json:"response" v:"required"`
}

type PasskeyLoginFinishRes struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type PasskeyRegistrationBeginReq struct {
	g.Meta `path:"/api/v1/account/passkeys/registration/begin" method:"post" tags:"passkeys" summary:"Begin passkey registration"`
}

type PasskeyRegistrationBeginRes struct {
	CeremonyID string          `json:"ceremonyId"`
	ExpiresAt  string          `json:"expiresAt"`
	Options    json.RawMessage `json:"options"`
}

type PasskeyRegistrationFinishReq struct {
	g.Meta     `path:"/api/v1/account/passkeys/registration/finish" method:"post" tags:"passkeys" summary:"Finish passkey registration"`
	CeremonyID string          `json:"ceremonyId" v:"required"`
	Label      string          `json:"label" v:"length:0,100"`
	Response   json.RawMessage `json:"response" v:"required"`
}

type PasskeyRegistrationFinishRes struct {
	Passkey PasskeyEntry `json:"passkey"`
}

type ListPasskeysReq struct {
	g.Meta `path:"/api/v1/account/passkeys" method:"get" tags:"passkeys" summary:"List passkeys"`
}

type ListPasskeysRes struct {
	Entries []PasskeyEntry `json:"entries"`
}

type RenamePasskeyReq struct {
	g.Meta `path:"/api/v1/account/passkeys/{id}" method:"patch" tags:"passkeys" summary:"Rename a passkey"`
	ID     string `json:"id" in:"path" v:"required"`
	Label  string `json:"label" v:"length:0,100"`
}

type RenamePasskeyRes struct {
	Passkey PasskeyEntry `json:"passkey"`
}

type RevokePasskeyReq struct {
	g.Meta `path:"/api/v1/account/passkeys/{id}" method:"delete" tags:"passkeys" summary:"Remove a passkey"`
	ID     string `json:"id" in:"path" v:"required"`
}

type RevokePasskeyRes struct{}

type PasskeyEntry struct {
	ID             string   `json:"id"`
	Label          string   `json:"label"`
	Status         string   `json:"status"`
	Transports     []string `json:"transports"`
	Attachment     string   `json:"attachment"`
	BackupEligible bool     `json:"backupEligible"`
	BackupState    bool     `json:"backupState"`
	CreatedAt      string   `json:"createdAt"`
	LastUsedAt     string   `json:"lastUsedAt,omitempty"`
}
