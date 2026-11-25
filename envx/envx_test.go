package envx

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestGetters(t *testing.T) {
	t.Setenv("GK_S", "  ciao ")
	t.Setenv("GK_EMPTY", "   ")
	t.Setenv("GK_I", "42")
	t.Setenv("GK_BAD", "x")
	t.Setenv("GK_B", "Yes")
	t.Setenv("GK_D", "90")
	t.Setenv("GK_D2", "2m")
	t.Setenv("GK_L", "a, b,,c ")

	if got := String("GK_S", "d"); got != "ciao" {
		t.Errorf("String = %q", got)
	}
	if got := String("GK_EMPTY", "d"); got != "d" {
		t.Errorf("String empty = %q", got)
	}
	if got := First("GK_MISSING", "GK_EMPTY", "GK_S"); got != "ciao" {
		t.Errorf("First = %q", got)
	}
	if Int("GK_I", 1) != 42 || Int("GK_BAD", 1) != 1 || Int64("GK_I", 1) != 42 {
		t.Error("Int/Int64")
	}
	if !Bool("GK_B", false) || Bool("GK_BAD", false) {
		t.Error("Bool")
	}
	if Duration("GK_D", 0) != 90*time.Second || Duration("GK_D2", 0) != 2*time.Minute || Duration("GK_BAD", time.Hour) != time.Hour {
		t.Error("Duration")
	}
	if got := List("GK_L", nil); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("List = %v", got)
	}
	if _, err := Required("GK_MISSING"); err == nil {
		t.Error("Required deve fallire")
	}
}

func TestLoadDotEnv(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	_ = os.WriteFile(p, []byte("# commento\nGK_A=1\nexport GK_B=\"due\"\nGK_KEEP=nuovo\nmalformata\n"), 0o600)
	t.Setenv("GK_KEEP", "vecchio")
	t.Setenv("GK_A", "")
	t.Setenv("GK_B", "")
	if err := LoadDotEnv(p); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("GK_A") != "1" || os.Getenv("GK_B") != "due" || os.Getenv("GK_KEEP") != "vecchio" {
		t.Errorf("valori: %q %q %q", os.Getenv("GK_A"), os.Getenv("GK_B"), os.Getenv("GK_KEEP"))
	}
	if err := LoadDotEnv(filepath.Join(t.TempDir(), "nope")); err != nil {
		t.Errorf("file assente: %v", err)
	}
}
