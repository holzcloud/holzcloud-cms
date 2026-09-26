package oidc

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

var (
	testRSA, _ = rsa.GenerateKey(rand.Reader, 2048)
	testEC, _  = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	testNow    = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
)

const (
	testIssuer = "https://auth.example.org/application/o/holzcloud/"
	testClient = "holzcloud-client"
	testNonce  = "n-0S6_WzA2Mj"
)

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func claims(over map[string]any) map[string]any {
	c := map[string]any{
		"iss":                testIssuer,
		"aud":                testClient,
		"sub":                "abc123",
		"exp":                testNow.Add(5 * time.Minute).Unix(),
		"iat":                testNow.Unix(),
		"nonce":              testNonce,
		"preferred_username": "anna",
		"email":              "anna@example.com",
		"name":               "Anna Ashcroft",
		"groups":             []string{"holzcloud-admins", "staff"},
	}
	for k, v := range over {
		if v == nil {
			delete(c, k)
			continue
		}
		c[k] = v
	}
	return c
}

func sign(t *testing.T, alg, kid string, c map[string]any, key any) string {
	t.Helper()
	h := map[string]any{"alg": alg, "typ": "JWT"}
	if kid != "" {
		h["kid"] = kid
	}
	hj, _ := json.Marshal(h)
	cj, _ := json.Marshal(c)
	signed := b64(hj) + "." + b64(cj)
	digest := sha256.Sum256([]byte(signed))
	var sig []byte
	switch alg {
	case "RS256":
		s, err := rsa.SignPKCS1v15(rand.Reader, key.(*rsa.PrivateKey), crypto.SHA256, digest[:])
		if err != nil {
			t.Fatal(err)
		}
		sig = s
	case "ES256":
		r, s, err := ecdsa.Sign(rand.Reader, key.(*ecdsa.PrivateKey), digest[:])
		if err != nil {
			t.Fatal(err)
		}
		sig = make([]byte, 64)
		r.FillBytes(sig[:32])
		s.FillBytes(sig[32:])
	case "HS256":
		mac := hmac.New(sha256.New, key.([]byte))
		mac.Write([]byte(signed))
		sig = mac.Sum(nil)
	case "none":
	}
	return signed + "." + b64(sig)
}

func rsaVerifier() *Verifier {
	return &Verifier{Issuer: testIssuer, ClientID: testClient,
		Keys: []Key{{ID: "k1", Public: &testRSA.PublicKey}, {ID: "k2", Public: &testEC.PublicKey}},
		Now:  func() time.Time { return testNow }}
}

func TestAValidTokenIsRead(t *testing.T) {
	for _, tc := range []struct {
		alg, kid string
		key      any
	}{{"RS256", "k1", testRSA}, {"ES256", "k2", testEC}, {"RS256", "", testRSA}} {
		got, err := rsaVerifier().Verify(sign(t, tc.alg, tc.kid, claims(nil), tc.key), testNonce, Options{})
		if err != nil {
			t.Fatalf("%s: %v", tc.alg, err)
		}
		if got.Username != "anna" || got.Email != "anna@example.com" || got.Name != "Anna Ashcroft" ||
			len(got.Groups) != 2 || got.Groups[0] != "holzcloud-admins" {
			t.Errorf("%s: %+v", tc.alg, got)
		}
	}
}

func TestTheSecretSignsOnlyWhereNoPublicKeyIsConfigured(t *testing.T) {
	secret := []byte("a-client-secret-of-reasonable-length")
	v := &Verifier{Issuer: testIssuer, ClientID: testClient, Secret: secret, Now: func() time.Time { return testNow }}
	if _, err := v.Verify(sign(t, "HS256", "", claims(nil), secret), testNonce, Options{}); err != nil {
		t.Fatalf("HS256 with the secret: %v", err)
	}
	if _, err := v.Verify(sign(t, "HS256", "", claims(nil), []byte("another secret")), testNonce, Options{}); !errors.Is(err, ErrSignature) {
		t.Errorf("a wrong secret: %v", err)
	}

	// The confusion: a public key used as an HMAC secret. With public keys
	// configured, HS256 is refused whatever it was signed with.
	pubDER, _ := x509.MarshalPKIXPublicKey(&testRSA.PublicKey)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	v = rsaVerifier()
	v.Secret = secret
	for _, key := range [][]byte{pubPEM, secret} {
		if _, err := v.Verify(sign(t, "HS256", "", claims(nil), key), testNonce, Options{}); !errors.Is(err, ErrAlgorithm) {
			t.Errorf("HS256 beside public keys: %v", err)
		}
	}
}

func TestTheNoneAlgorithmIsRefused(t *testing.T) {
	if _, err := rsaVerifier().Verify(sign(t, "none", "", claims(nil), nil), testNonce, Options{}); !errors.Is(err, ErrAlgorithm) {
		t.Errorf("alg none: %v", err)
	}
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	if _, err := rsaVerifier().Verify(sign(t, "RS256", "k1", claims(nil), other), testNonce, Options{}); !errors.Is(err, ErrSignature) {
		t.Errorf("signed by another key: %v", err)
	}
	if _, err := rsaVerifier().Verify(sign(t, "RS256", "k9", claims(nil), testRSA), testNonce, Options{}); !errors.Is(err, ErrNoKey) {
		t.Errorf("an unknown kid: %v", err)
	}
}

func TestEveryClaimThatBindsTheTokenIsChecked(t *testing.T) {
	for name, tc := range map[string]struct {
		over  map[string]any
		nonce string
		want  error
	}{
		"issuer":               {map[string]any{"iss": "https://evil.example/"}, testNonce, ErrIssuer},
		"no issuer":            {map[string]any{"iss": nil}, testNonce, ErrIssuer},
		"audience":             {map[string]any{"aud": "another-client"}, testNonce, ErrAudience},
		"audience list":        {map[string]any{"aud": []string{"a", testClient}}, testNonce, ErrAudience},
		"azp elsewhere":        {map[string]any{"azp": "another-client"}, testNonce, ErrAudience},
		"expired":              {map[string]any{"exp": testNow.Add(-2 * time.Minute).Unix()}, testNonce, ErrExpired},
		"no expiry":            {map[string]any{"exp": nil}, testNonce, ErrExpired},
		"issued in the future": {map[string]any{"iat": testNow.Add(10 * time.Minute).Unix()}, testNonce, ErrNotYet},
		"not before":           {map[string]any{"nbf": testNow.Add(10 * time.Minute).Unix()}, testNonce, ErrNotYet},
		"another nonce":        {nil, "somebody else's sign-in", ErrNonce},
		"no nonce":             {map[string]any{"nonce": nil}, testNonce, ErrNonce},
		"empty expected nonce": {map[string]any{"nonce": ""}, "", ErrNonce},
	} {
		_, err := rsaVerifier().Verify(sign(t, "RS256", "k1", claims(tc.over), testRSA), tc.nonce, Options{})
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", name, err, tc.want)
		}
	}
	// Several audiences are fine when the authorised party is this client.
	c := claims(map[string]any{"aud": []string{"a", testClient}, "azp": testClient})
	if _, err := rsaVerifier().Verify(sign(t, "RS256", "k1", c, testRSA), testNonce, Options{}); err != nil {
		t.Errorf("several audiences with azp: %v", err)
	}
	// A minute of disagreement between the clocks is tolerated.
	c = claims(map[string]any{"exp": testNow.Add(-30 * time.Second).Unix()})
	if _, err := rsaVerifier().Verify(sign(t, "RS256", "k1", c, testRSA), testNonce, Options{}); err != nil {
		t.Errorf("within the skew: %v", err)
	}
}

func TestTheClaimNamesCanBeChanged(t *testing.T) {
	c := claims(map[string]any{"nickname": "a.ashcroft", "roles": []string{"editors"}})
	got, err := rsaVerifier().Verify(sign(t, "RS256", "k1", c, testRSA), testNonce,
		Options{UsernameClaim: "nickname", GroupsClaim: "roles"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "a.ashcroft" || len(got.Groups) != 1 || got.Groups[0] != "editors" {
		t.Errorf("%+v", got)
	}
	// A groups claim that is one string is not read as a group.
	c = claims(map[string]any{"groups": "holzcloud-admins"})
	got, err = rsaVerifier().Verify(sign(t, "RS256", "k1", c, testRSA), testNonce, Options{})
	if err != nil || len(got.Groups) != 0 {
		t.Errorf("a string as groups: %v %+v", err, got)
	}
}

func TestMalformedTokensAreRefused(t *testing.T) {
	good := sign(t, "RS256", "k1", claims(nil), testRSA)
	parts := strings.Split(good, ".")
	crit := b64([]byte(`{"alg":"RS256","kid":"k1","crit":["exp"]}`))
	for name, raw := range map[string]string{
		"two parts":    parts[0] + "." + parts[1],
		"bad base64":   parts[0] + ".%%%." + parts[2],
		"not json":     b64([]byte("x")) + "." + parts[1] + "." + parts[2],
		"crit":         crit + "." + parts[1] + "." + parts[2],
		"far too long": strings.Repeat("a", maxTokenBytes+1),
	} {
		if _, err := rsaVerifier().Verify(raw, testNonce, Options{}); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestKeysAreReadFromJWKSAndPEM(t *testing.T) {
	jwks := fmt.Sprintf(`{"keys":[
	  {"kty":"RSA","kid":"r","use":"sig","alg":"RS256","n":%q,"e":%q},
	  {"kty":"EC","kid":"e","crv":"P-256","x":%q,"y":%q},
	  {"kty":"RSA","kid":"enc","use":"enc","n":"AQAB","e":"AQAB"}]}`,
		b64(testRSA.N.Bytes()), b64([]byte{1, 0, 1}),
		b64(testEC.X.FillBytes(make([]byte, 32))), b64(testEC.Y.FillBytes(make([]byte, 32))))
	keys, err := ParseKeys([]byte(jwks))
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0].ID != "r" || keys[1].ID != "e" {
		t.Fatalf("%+v", keys)
	}
	v := &Verifier{Issuer: testIssuer, ClientID: testClient, Keys: keys, Now: func() time.Time { return testNow }}
	if _, err := v.Verify(sign(t, "RS256", "r", claims(nil), testRSA), testNonce, Options{}); err != nil {
		t.Errorf("RS256 through JWKS: %v", err)
	}
	if _, err := v.Verify(sign(t, "ES256", "e", claims(nil), testEC), testNonce, Options{}); err != nil {
		t.Errorf("ES256 through JWKS: %v", err)
	}

	der, _ := x509.MarshalPKIXPublicKey(&testRSA.PublicKey)
	keys, err = ParseKeys(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
	if err != nil || len(keys) != 1 {
		t.Fatalf("PEM: %v %v", keys, err)
	}

	weak, _ := rsa.GenerateKey(rand.Reader, 1024)
	der, _ = x509.MarshalPKIXPublicKey(&weak.PublicKey)
	if _, err := ParseKeys(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})); err == nil {
		t.Error("a 1024-bit key was accepted")
	}
	for name, doc := range map[string]string{
		"empty":       "",
		"private key": "-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n",
		"no keys":     `{"keys":[]}`,
		"P-384":       `{"keys":[{"kty":"EC","crv":"P-384","x":"AA","y":"AA"}]}`,
		"off curve":   `{"keys":[{"kty":"EC","crv":"P-256","x":"AQ","y":"AQ"}]}`,
		"oct":         `{"keys":[{"kty":"oct","k":"c2VjcmV0"}]}`,
	} {
		if _, err := ParseKeys([]byte(doc)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}
