package dataset

import "testing"

func TestIsNewerRelease(t *testing.T) {
	newer, err := isNewerRelease("v0.4.1", "0.4.0")
	if err != nil || !newer {
		t.Fatalf("newer=%v err=%v", newer, err)
	}
	for _, tag := range []string{"v0.4.0", "v0.3.9"} {
		newer, err := isNewerRelease(tag, "0.4.0")
		if err != nil || newer {
			t.Fatalf("tag=%s newer=%v err=%v", tag, newer, err)
		}
	}
	if _, err := isNewerRelease("v0.4", "0.4.0"); err == nil {
		t.Fatal("expected invalid release tag")
	}
}
