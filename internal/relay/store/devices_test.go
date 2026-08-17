package store

import (
	"crypto/ed25519"
	"crypto/rand"
	"path/filepath"
	"testing"
	"time"
)

func TestListDevices(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	enroll := func(email string, at time.Time) Device {
		tok, err := st.CreateInvite(email, time.Hour, at)
		if err != nil {
			t.Fatalf("CreateInvite: %v", err)
		}
		pub, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("keygen: %v", err)
		}
		_, d, err := st.Enroll(tok, pub, "lap-"+email, at)
		if err != nil {
			t.Fatalf("Enroll: %v", err)
		}
		return d
	}

	now := time.Now().UTC()
	d1 := enroll("a@x.com", now)                  // older
	d2 := enroll("b@x.com", now.Add(time.Second)) // newer
	if err := st.RevokeDevice(d1.ID, now.Add(2*time.Second)); err != nil {
		t.Fatalf("RevokeDevice: %v", err)
	}

	list, err := st.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	// newest first (created_at DESC).
	if list[0].DeviceID != d2.ID || list[1].DeviceID != d1.ID {
		t.Errorf("order = %s,%s, want %s,%s", list[0].DeviceID, list[1].DeviceID, d2.ID, d1.ID)
	}
	byID := map[string]DeviceListing{list[0].DeviceID: list[0], list[1].DeviceID: list[1]}
	if byID[d1.ID].RevokedAt.IsZero() {
		t.Error("d1 should be revoked")
	}
	if !byID[d2.ID].RevokedAt.IsZero() {
		t.Error("d2 should be active")
	}
	if byID[d2.ID].Email != "b@x.com" {
		t.Errorf("email = %q, want b@x.com", byID[d2.ID].Email)
	}
}

func TestSetPersonNameOnlyWhenEmpty(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()
	now := time.Now().UTC()
	tok, err := st.CreateInvite("c@x.com", time.Hour, now)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	p, _, err := st.Enroll(tok, pub, "dev", now)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	if err := st.SetPersonName(p.ID, "Bob"); err != nil {
		t.Fatalf("SetPersonName: %v", err)
	}
	// Re-setting is a no-op guard: an existing name is never silently overwritten.
	if err := st.SetPersonName(p.ID, "Robert"); err != nil {
		t.Fatalf("SetPersonName (2): %v", err)
	}
	got, err := st.PersonByID(p.ID)
	if err != nil {
		t.Fatalf("PersonByID: %v", err)
	}
	if got.Label != "Bob" {
		t.Errorf("label = %q, want Bob (not overwritten)", got.Label)
	}
}
