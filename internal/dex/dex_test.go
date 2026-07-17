package dex

import "testing"

func TestSpeciesForDeterministic(t *testing.T) {
	id := "session-abc-123"
	first := SpeciesFor(id)
	for i := 0; i < 100; i++ {
		if got := SpeciesFor(id); got.Name != first.Name {
			t.Fatalf("SpeciesFor not deterministic: %q vs %q", got.Name, first.Name)
		}
	}
}

func TestSpeciesForDistribution(t *testing.T) {
	if Count() == 0 {
		t.Fatal("no species loaded")
	}
	counts := map[string]int{}
	const n = 6000
	for i := 0; i < n; i++ {
		sp := SpeciesFor(randID(i))
		counts[sp.Name]++
	}
	// every species should get a non-trivial share
	expected := n / Count()
	for _, sp := range All() {
		c := counts[sp.Name]
		if c < expected/3 {
			t.Errorf("species %q underrepresented: %d (expected ~%d)", sp.Name, c, expected)
		}
	}
}

func TestFramesParsed(t *testing.T) {
	for _, sp := range All() {
		if sp.Name == "" {
			t.Error("species with empty name")
		}
		if sp.Idle.H == 0 || sp.Idle.W == 0 {
			t.Errorf("%s: empty idle frame", sp.Name)
		}
		if len(sp.WalkA.Lines) == 0 || len(sp.WalkB.Lines) == 0 {
			t.Errorf("%s: missing walk frames", sp.Name)
		}
	}
}

func TestMirrorSwaps(t *testing.T) {
	f := Frame{Lines: []string{"(/>"}, W: 3, H: 1}
	m := f.Mirror()
	if got := m.Lines[0]; got != "<\\)" {
		t.Errorf("mirror = %q, want %q", got, "<\\)")
	}
}

func randID(i int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	x := uint32(i)*2654435761 + 1
	for j := range b {
		b[j] = letters[x%uint32(len(letters))]
		x = x*1103515245 + 12345
	}
	return string(b)
}
