// Package oidc checks an OpenID Connect ID token without asking anybody.
//
// The usual way to sign somebody in through an identity provider is the
// authorization code flow: the browser comes back with a code, and the server
// trades it for a token at the provider's token endpoint and fetches the
// provider's signing keys from its JWKS address. Both are requests this program
// would make to another server while it runs, and that is the one rule this
// project does not break (docs/security.md, "No runtime dependencies on third
// parties"; internal/web/forwardauth.go says the same about the JWKS).
//
// So this package takes the other road the specification offers. The provider
// is asked for an ID token directly (response_type=id_token) and to hand it
// over by having the browser post it back (response_mode=form_post). The token
// is signed; the key that checks the signature is the one thing that has to be
// known in advance, and the operator puts it on disk once — a JWKS document or
// a PEM public key, downloaded from the provider by hand — or, for a provider
// that signs with the client secret (Authentik without a signing key does),
// names that secret in the environment.
//
// Nothing here opens a socket. The price is written down where an operator
// reads it: when the provider rotates its signing key, the file has to be
// replaced by hand, and sign-ins are refused until it is.
package oidc

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// Errors a token is refused with. They are codes for the server log and never
// sentences on a screen: somebody refused here is shown the login form with a
// general message, and the reason goes to the operator.
var (
	ErrMalformed = errors.New("oidc: the token is not a well-formed JWT")
	ErrAlgorithm = errors.New("oidc: the token is signed with an algorithm this installation does not accept")
	ErrNoKey     = errors.New("oidc: no configured key matches the token")
	ErrSignature = errors.New("oidc: the signature does not verify")
	ErrIssuer    = errors.New("oidc: the token was issued by somebody else")
	ErrAudience  = errors.New("oidc: the token was issued for another client")
	ErrExpired   = errors.New("oidc: the token has expired")
	ErrNotYet    = errors.New("oidc: the token was issued in the future")
	ErrNonce     = errors.New("oidc: the token does not answer this sign-in")
)

// skew is how far the two clocks may disagree. A minute is what the common
// libraries allow, and a server whose clock is further off than that has a
// problem a sign-in cannot fix.
const skew = time.Minute

// maxTokenBytes bounds what is decoded at all. An ID token with a few groups is
// a couple of kilobytes; a form field of megabytes is not a token.
const maxTokenBytes = 64 << 10

// Key is one key a token may be signed with.
type Key struct {
	// ID is the JWKS "kid". Empty for a key read from a PEM file, which then
	// matches a token whatever it names.
	ID string
	// Public is an *rsa.PublicKey or an *ecdsa.PublicKey on P-256.
	Public crypto.PublicKey
}

// Verifier checks tokens for one client at one issuer.
type Verifier struct {
	Issuer   string
	ClientID string
	// Keys are the public keys of the provider. When there are any, only RS256
	// and ES256 are accepted.
	Keys []Key
	// Secret is the client secret, for a provider that signs with it (HS256).
	// It is used only when Keys is empty. Accepting HS256 beside public keys is
	// the textbook confusion: whoever knows the public key — everybody — could
	// then sign a token with it as an HMAC secret.
	Secret []byte
	// Now is the clock, replaceable in tests.
	Now func() time.Time
}

// Claims is what a verified token says about the person.
type Claims struct {
	Subject string
	// Username is the claim the verifier was told to read the username from.
	Username string
	Email    string
	// EmailVerified is false when the provider said so explicitly, and true
	// when it said true or said nothing.
	EmailVerified bool
	Name          string
	Groups        []string
}

// Options names which claims carry what. The defaults are what Authentik,
// Keycloak and most providers send.
type Options struct {
	// UsernameClaim defaults to preferred_username.
	UsernameClaim string
	// GroupsClaim defaults to groups.
	GroupsClaim string
}

// Verify checks raw and returns what it says. nonce is the value this server
// sent with the sign-in the token answers; a token without it, or with another,
// is refused, because a token captured from one sign-in must not complete
// another.
func (v *Verifier) Verify(raw, nonce string, opts Options) (*Claims, error) {
	if len(raw) > maxTokenBytes {
		return nil, ErrMalformed
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, ErrMalformed
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrMalformed
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrMalformed
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrMalformed
	}
	var header struct {
		Alg  string   `json:"alg"`
		Kid  string   `json:"kid"`
		Crit []string `json:"crit"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, ErrMalformed
	}
	// A header naming extensions the reader must understand is a header this
	// reader does not understand (RFC 7515, 4.1.11).
	if len(header.Crit) > 0 {
		return nil, ErrMalformed
	}

	// The signature before a single claim is believed.
	signed := []byte(parts[0] + "." + parts[1])
	if err := v.checkSignature(header.Alg, header.Kid, signed, sig); err != nil {
		return nil, err
	}

	var c map[string]any
	dec := json.NewDecoder(strings.NewReader(string(payload)))
	dec.UseNumber()
	if err := dec.Decode(&c); err != nil {
		return nil, ErrMalformed
	}

	if iss, _ := c["iss"].(string); iss == "" || iss != v.Issuer {
		return nil, ErrIssuer
	}
	if !audienceMatches(c["aud"], c["azp"], v.ClientID) {
		return nil, ErrAudience
	}
	now := time.Now()
	if v.Now != nil {
		now = v.Now()
	}
	exp, ok := numericDate(c["exp"])
	if !ok || !now.Before(exp.Add(skew)) {
		return nil, ErrExpired
	}
	if iat, ok := numericDate(c["iat"]); ok && iat.After(now.Add(skew)) {
		return nil, ErrNotYet
	}
	if nbf, ok := numericDate(c["nbf"]); ok && nbf.After(now.Add(skew)) {
		return nil, ErrNotYet
	}
	got, _ := c["nonce"].(string)
	if nonce == "" || subtle.ConstantTimeCompare([]byte(got), []byte(nonce)) != 1 {
		return nil, ErrNonce
	}

	usernameClaim := opts.UsernameClaim
	if usernameClaim == "" {
		usernameClaim = "preferred_username"
	}
	groupsClaim := opts.GroupsClaim
	if groupsClaim == "" {
		groupsClaim = "groups"
	}
	out := &Claims{EmailVerified: true}
	out.Subject, _ = c["sub"].(string)
	out.Username, _ = c[usernameClaim].(string)
	out.Email, _ = c["email"].(string)
	out.Name, _ = c["name"].(string)
	if verified, present := c["email_verified"]; present {
		b, _ := verified.(bool)
		out.EmailVerified = b
	}
	out.Groups = stringList(c[groupsClaim])
	return out, nil
}

func (v *Verifier) checkSignature(alg, kid string, signed, sig []byte) error {
	switch alg {
	case "RS256", "ES256":
		if len(v.Keys) == 0 {
			return ErrAlgorithm
		}
		digest := sha256.Sum256(signed)
		matched := false
		for _, k := range v.Keys {
			if k.ID != "" && kid != "" && k.ID != kid {
				continue
			}
			switch pub := k.Public.(type) {
			case *rsa.PublicKey:
				if alg != "RS256" {
					continue
				}
				matched = true
				if rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig) == nil {
					return nil
				}
			case *ecdsa.PublicKey:
				if alg != "ES256" || pub.Curve != elliptic.P256() || len(sig) != 64 {
					continue
				}
				matched = true
				r := new(big.Int).SetBytes(sig[:32])
				s := new(big.Int).SetBytes(sig[32:])
				if ecdsa.Verify(pub, digest[:], r, s) {
					return nil
				}
			}
		}
		if !matched {
			return ErrNoKey
		}
		return ErrSignature
	case "HS256":
		if len(v.Keys) > 0 || len(v.Secret) == 0 {
			return ErrAlgorithm
		}
		mac := hmac.New(sha256.New, v.Secret)
		mac.Write(signed)
		if !hmac.Equal(mac.Sum(nil), sig) {
			return ErrSignature
		}
		return nil
	default:
		// "none" above all, and everything else nobody asked for.
		return ErrAlgorithm
	}
}

// audienceMatches follows OpenID Connect Core 3.1.3.7: the client has to be
// among the audiences, and where there is more than one, the authorised party
// has to be the client.
func audienceMatches(aud, azp any, clientID string) bool {
	if clientID == "" {
		return false
	}
	list := stringList(aud)
	if s, ok := aud.(string); ok {
		list = []string{s}
	}
	found := false
	for _, a := range list {
		if a == clientID {
			found = true
		}
	}
	if !found {
		return false
	}
	if len(list) > 1 {
		p, _ := azp.(string)
		return p == clientID
	}
	if p, ok := azp.(string); ok && p != "" && p != clientID {
		return false
	}
	return true
}

func numericDate(v any) (time.Time, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return time.Time{}, false
	}
	f, err := n.Float64()
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(int64(f), 0), true
}

// stringList reads a claim that is a list of strings. A single string is not a
// list here: a groups claim of "admins" is a provider's bug, and reading it as
// one group would be guessing.
func stringList(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

// ParseKeys reads the keys an operator put on disk: a JWKS document, as the
// provider publishes it, or one or more PEM blocks — PUBLIC KEY or CERTIFICATE.
//
// A key this package cannot use is an error rather than something skipped: a
// file that silently yields no key is a sign-in that fails in a year, when
// somebody has forgotten why.
func ParseKeys(data []byte) ([]Key, error) {
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "{") {
		return parseJWKS([]byte(trimmed))
	}
	var keys []Key
	rest := []byte(trimmed)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		var pub any
		switch block.Type {
		case "PUBLIC KEY":
			k, err := x509.ParsePKIXPublicKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("oidc: a PUBLIC KEY block: %w", err)
			}
			pub = k
		case "CERTIFICATE":
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("oidc: a CERTIFICATE block: %w", err)
			}
			pub = cert.PublicKey
		default:
			return nil, fmt.Errorf("oidc: a PEM block of type %q is not a public key", block.Type)
		}
		k, err := usable(pub)
		if err != nil {
			return nil, err
		}
		keys = append(keys, Key{Public: k})
	}
	if len(keys) == 0 {
		return nil, errors.New("oidc: the file holds neither a JWKS document nor a PEM public key")
	}
	return keys, nil
}

func parseJWKS(data []byte) ([]Key, error) {
	var doc struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			Use string `json:"use"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
			Crv string `json:"crv"`
			X   string `json:"x"`
			Y   string `json:"y"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("oidc: the JWKS document: %w", err)
	}
	var keys []Key
	for _, k := range doc.Keys {
		// A key for encryption is not a key for checking signatures.
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		switch k.Kty {
		case "RSA":
			n, err1 := base64.RawURLEncoding.DecodeString(k.N)
			e, err2 := base64.RawURLEncoding.DecodeString(k.E)
			if err1 != nil || err2 != nil || len(n) == 0 || len(e) == 0 || len(e) > 4 {
				return nil, fmt.Errorf("oidc: the RSA key %q is malformed", k.Kid)
			}
			pub := &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
			if _, err := usable(pub); err != nil {
				return nil, err
			}
			keys = append(keys, Key{ID: k.Kid, Public: pub})
		case "EC":
			if k.Crv != "P-256" {
				return nil, fmt.Errorf("oidc: the EC key %q is on %s; only P-256 (ES256) is supported", k.Kid, k.Crv)
			}
			x, err1 := base64.RawURLEncoding.DecodeString(k.X)
			y, err2 := base64.RawURLEncoding.DecodeString(k.Y)
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("oidc: the EC key %q is malformed", k.Kid)
			}
			pub := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}
			if !pub.Curve.IsOnCurve(pub.X, pub.Y) {
				return nil, fmt.Errorf("oidc: the EC key %q is not a point on P-256", k.Kid)
			}
			keys = append(keys, Key{ID: k.Kid, Public: pub})
		default:
			return nil, fmt.Errorf("oidc: a key of type %q is not supported", k.Kty)
		}
	}
	if len(keys) == 0 {
		return nil, errors.New("oidc: the JWKS document holds no signing key")
	}
	return keys, nil
}

// usable refuses keys too weak to rely on and curves this package does not
// check signatures on.
func usable(pub any) (crypto.PublicKey, error) {
	switch k := pub.(type) {
	case *rsa.PublicKey:
		if k.N.BitLen() < 2048 {
			return nil, fmt.Errorf("oidc: an RSA key of %d bits is too short; at least 2048 are required", k.N.BitLen())
		}
		return k, nil
	case *ecdsa.PublicKey:
		if k.Curve != elliptic.P256() {
			return nil, errors.New("oidc: only EC keys on P-256 (ES256) are supported")
		}
		return k, nil
	default:
		return nil, fmt.Errorf("oidc: a key of type %T is not supported", pub)
	}
}
