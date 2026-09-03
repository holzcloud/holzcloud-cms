package activity

import "testing"

func TestSanitizeNilReturnsEmptyMap(t *testing.T) {
	got := sanitize(nil)
	if got == nil {
		t.Fatal("sanitize(nil) returned nil; want non-nil empty map")
	}
	if len(got) != 0 {
		t.Errorf("sanitize(nil) len = %d; want 0", len(got))
	}
}

func TestSanitizeAllowedKeysPassThrough(t *testing.T) {
	in := map[string]any{"email": "x@y.z", "id": int64(5), "title": "Hello"}
	got := sanitize(in)
	if len(got) != 3 {
		t.Errorf("len = %d; want 3; got %v", len(got), got)
	}
	for k, v := range in {
		if got[k] != v {
			t.Errorf("got[%q] = %v; want %v", k, got[k], v)
		}
	}
}

func TestSanitizeStripsBannedKeys(t *testing.T) {
	banned := []string{
		"password", "new_password", "old_password",
		"confirm_password", "password_confirm",
		"token", "csrf", "csrf_token", "_csrf",
	}
	for _, key := range banned {
		t.Run(key, func(t *testing.T) {
			in := map[string]any{key: "SECRET", "email": "x@y.z"}
			got := sanitize(in)
			if _, present := got[key]; present {
				t.Errorf("key %q leaked through sanitizer: got %v", key, got)
			}
			if got["email"] != "x@y.z" {
				t.Errorf("email key was incorrectly stripped: got %v", got)
			}
		})
	}
}

func TestSanitizeCaseInsensitive(t *testing.T) {
	cases := []map[string]any{
		{"Password": "x"},
		{"PASSWORD": "x"},
		{"Token": "x"},
		{"CSRF": "x"},
		{"New_Password": "x"},
		{"Csrf_Token": "x"},
	}
	for _, in := range cases {
		got := sanitize(in)
		if len(got) != 0 {
			t.Errorf("sanitize(%v) = %v; want empty map (case-insensitive deny)", in, got)
		}
	}
}

func TestSanitizeDoesNotMutateInput(t *testing.T) {
	in := map[string]any{"password": "secret", "email": "x@y.z"}
	_ = sanitize(in)
	if _, present := in["password"]; !present {
		t.Fatal("sanitize mutated the input map (password was removed)")
	}
}

func TestSanitizeMultipleBannedKeys(t *testing.T) {
	in := map[string]any{"password": "a", "token": "b", "csrf": "c", "email": "x@y.z"}
	got := sanitize(in)
	if len(got) != 1 {
		t.Fatalf("len = %d; want 1; got %v", len(got), got)
	}
	if got["email"] != "x@y.z" {
		t.Errorf("email key missing or wrong: got %v", got)
	}
}
