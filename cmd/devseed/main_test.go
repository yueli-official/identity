package main

import "testing"

func TestParseSeedRequiresPublicUserKeyAndCanonicalHandle(t *testing.T) {
	raw := `{"account":{"id":"ac73d232-ce55-487d-bb39-fd336f1a9806","userKey":"usr_AAAAAAAAAAAAAAAAAAAAAA","email":"test@example.test","password":"long-enough-password","handle":"Test_Admin","displayName":"Test Admin"}}`
	declared, err := parseSeed(raw)
	if err != nil {
		t.Fatal(err)
	}
	if declared.Account.UserKey != "usr_AAAAAAAAAAAAAAAAAAAAAA" || declared.Account.Handle != "test_admin" {
		t.Fatalf("account = %#v", declared.Account)
	}
}

func TestParseSeedRejectsStorageIDAndReservedHandle(t *testing.T) {
	tests := []struct {
		name    string
		userKey string
		handle  string
	}{
		{name: "uuid is not a public key", userKey: "ac73d232-ce55-487d-bb39-fd336f1a9806", handle: "test_admin"},
		{name: "reserved handle", userKey: "usr_AAAAAAAAAAAAAAAAAAAAAA", handle: "admin"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := `{"account":{"id":"ac73d232-ce55-487d-bb39-fd336f1a9806","userKey":"` + test.userKey + `","email":"test@example.test","password":"long-enough-password","handle":"` + test.handle + `","displayName":"Test Admin"}}`
			if _, err := parseSeed(raw); err == nil {
				t.Fatal("parseSeed() accepted invalid account identity contract")
			}
		})
	}
}
