package preset

import "testing"

func TestApply(t *testing.T) {
	t.Parallel()

	a, err := Apply("autobrr")
	if err != nil {
		t.Fatal(err)
	}

	if a.OwnerRepo != "autobrr/autobrr" || len(a.ExtraFiles) != 1 {
		t.Fatalf("%+v", a)
	}

	u, err := Apply("unpackerr")
	if err != nil {
		t.Fatal(err)
	}

	if u.OwnerRepo != "Unpackerr/unpackerr" {
		t.Fatalf("%+v", u)
	}

	c, err := Apply("captain-updater")
	if err != nil {
		t.Fatal(err)
	}

	if c.Name != "Captain Updater" {
		t.Fatalf("%+v", c)
	}

	if _, err := Apply("nope"); err == nil {
		t.Fatal("expected error")
	}

	if n := len(All()); n != 3 {
		t.Fatalf("presets %d", n)
	}
}
