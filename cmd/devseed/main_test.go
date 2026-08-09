package main

import "testing"

func TestParseSeedRequiresUUIDv7CompactPublicKeyAndCanonicalHandle(t *testing.T) {
	raw := `{"account":{"id":"019c52f0-0000-7000-8000-000000000001","userKey":"TestA123","email":"test@example.test","password":"long-enough-password","handle":"Test_Admin","displayName":"Test Admin"}}`
	declared, err := parseSeed(raw)
	if err != nil {
		t.Fatal(err)
	}
	if declared.Account.UserKey != "TestA123" || declared.Account.Handle != "test_admin" {
		t.Fatalf("account = %#v", declared.Account)
	}
}

func TestParseSeedRejectsWrongIdentifierKindsAndReservedHandle(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		userKey string
		handle  string
	}{
		{name: "uuid v4 is not the entity writer", id: "ac73d232-ce55-487d-bb39-fd336f1a9806", userKey: "TestA123", handle: "test_admin"},
		{name: "uuid is not a public key", id: "019c52f0-0000-7000-8000-000000000001", userKey: "ac73d232-ce55-487d-bb39-fd336f1a9806", handle: "test_admin"},
		{name: "invalid compact key", id: "019c52f0-0000-7000-8000-000000000001", userKey: "0ABCDEFG", handle: "test_admin"},
		{name: "reserved handle", id: "019c52f0-0000-7000-8000-000000000001", userKey: "TestA123", handle: "admin"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := `{"account":{"id":"` + test.id + `","userKey":"` + test.userKey + `","email":"test@example.test","password":"long-enough-password","handle":"` + test.handle + `","displayName":"Test Admin"}}`
			if _, err := parseSeed(raw); err == nil {
				t.Fatal("parseSeed() accepted invalid account identity contract")
			}
		})
	}
}
