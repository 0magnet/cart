//go:build !wasm

package main

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// ── readFile ─────────────────────────────────────────────────────────────────

// readFile is the only reader of the asset table, and it is indexed with
// constants scattered around the handlers. An index that is off must come back
// empty rather than panic and take the server with it.
func TestReadFileIsBoundsChecked(t *testing.T) {
	files := []FileAsset{
		{Name: "a", Data: []byte("first"), Mu: sync.Mutex{}},
		{Name: "b", Data: []byte("second"), Mu: sync.Mutex{}},
	}
	if got := string(readFile(files, 0)); got != "first" {
		t.Errorf("readFile(0) = %q, want first", got)
	}
	if got := string(readFile(files, 1)); got != "second" {
		t.Errorf("readFile(1) = %q, want second", got)
	}
	for _, i := range []int{-1, 2, 99, -100} {
		if got := readFile(files, i); got != nil {
			t.Errorf("readFile(%d) = %q, want nothing", i, got)
		}
	}
}

func TestReadFileOnAnEmptyTable(t *testing.T) {
	if got := readFile(nil, 0); got != nil {
		t.Errorf("readFile on an empty table = %q", got)
	}
}

// ── shorthand flags ──────────────────────────────────────────────────────────

// Every flag gets a distinct shorthand letter, and once they run out the
// answer is no shorthand rather than a repeat — two flags sharing a letter is
// a startup panic in cobra.
func TestShorthandFlagsAreDistinctAndThenEmpty(t *testing.T) {
	saved := nextShortIndex
	defer func() { nextShortIndex = saved }()
	nextShortIndex = 0

	seen := map[string]bool{}
	for i := 0; i < len(shorthandChars); i++ {
		s := getNextShortFlag()
		if s == "" {
			t.Fatalf("ran out of shorthands after %d, expected %d", i, len(shorthandChars))
		}
		if seen[s] {
			t.Fatalf("shorthand %q was handed out twice", s)
		}
		seen[s] = true
	}
	if got := getNextShortFlag(); got != "" {
		t.Errorf("past the end the shorthand is %q, want empty", got)
	}
}

// h is the help flag, so it must never be handed to anything else.
func TestShorthandNeverHandsOutH(t *testing.T) {
	for _, c := range shorthandChars {
		if c == 'h' {
			t.Error("h is in the shorthand pool; it belongs to --help")
		}
	}
}

// ── ccc ──────────────────────────────────────────────────────────────────────

// ccc turns a pointer to a field into that field's name, which is what makes
// the flags and the environment variables agree without repeating either.
func TestCCCNamesTheFieldAPointerBelongsTo(t *testing.T) {
	v := FlagVars{}
	for _, tc := range []struct {
		ptr         interface{}
		lower, uper string
	}{
		{&v.WebPort, "webport", "WEBPORT"},
		{&v.StripelivePK, "stripelivepk", "STRIPELIVEPK"},
		{&v.Teststripekey, "teststripekey", "TESTSTRIPEKEY"},
	} {
		if got := ccc(tc.ptr, &v, false); got != tc.lower {
			t.Errorf("ccc lower = %q, want %q", got, tc.lower)
		}
		if got := ccc(tc.ptr, &v, true); got != tc.uper {
			t.Errorf("ccc upper = %q, want %q", got, tc.uper)
		}
	}
}

// A pointer that is not one of the struct's fields names nothing, rather than
// naming the wrong one.
func TestCCCOnAPointerThatIsNotAField(t *testing.T) {
	v := FlagVars{}
	other := 0
	if got := ccc(&other, &v, false); got != "" {
		t.Errorf("ccc on a foreign pointer = %q, want empty", got)
	}
}

func TestCCCAcceptsAStructByValue(t *testing.T) {
	v := FlagVars{}
	// Passed by value the fields are not addressable, so nothing matches —
	// which is a miss rather than a panic.
	if got := ccc(&v.WebPort, v, false); got != "" {
		t.Errorf("ccc on a non-addressable struct = %q, want empty", got)
	}
}

func TestCCCPanicsOnSomethingThatIsNotAStruct(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("ccc accepted something that is not a struct")
		}
	}()
	n := 0
	ccc(&n, &n, false)
}

// ── error page ───────────────────────────────────────────────────────────────

func TestHTMLErrIsAWholeDocument(t *testing.T) {
	got := string(htmlErr("something went wrong"))
	for _, want := range []string{"<!DOCTYPE html>", "<html>", "</html>", "something went wrong"} {
		if !strings.Contains(got, want) {
			t.Errorf("the error page has no %q:\n%s", want, got)
		}
	}
}

// The message is a compile error or a stack trace, so its newlines have to
// survive into the page or it arrives as one unreadable line.
func TestHTMLErrKeepsLineBreaks(t *testing.T) {
	got := string(htmlErr("line one\nline two\nline three"))
	if n := strings.Count(got, "<br>"); n != 2 {
		t.Errorf("two newlines became %d line breaks:\n%s", n, got)
	}
	if strings.Contains(got, "line one\nline two") {
		t.Error("a raw newline survived instead of becoming a break")
	}
}

// ── log colors ──────────────────────────────────────────────────────────────

// Every status class gets its own color, and the ranges have to line up with
// the boundaries rather than one either side of them.
func TestBackgroundColorByStatusClass(t *testing.T) {
	for _, tc := range []struct {
		code int
		want string
	}{
		{http.StatusOK, green},
		{299, green},
		{http.StatusMultipleChoices, white},
		{399, white},
		{http.StatusBadRequest, yellow},
		{http.StatusNotFound, yellow},
		{499, yellow},
		{http.StatusInternalServerError, red},
		{599, red},
	} {
		if got := getBackgroundColor(tc.code); got != tc.want {
			t.Errorf("status %d got color %q, want %q", tc.code, got, tc.want)
		}
	}
}

func TestMethodColorsAreDistinct(t *testing.T) {
	seen := map[string][]string{}
	for _, m := range []string{
		http.MethodGet, http.MethodPost, http.MethodPut,
		http.MethodDelete, http.MethodPatch, http.MethodHead, http.MethodOptions,
	} {
		c := getMethodColor(m)
		if c == "" {
			t.Errorf("%s has no color", m)
		}
		seen[c] = append(seen[c], m)
	}
	// GET and POST at least must not look the same in a log.
	if getMethodColor(http.MethodGet) == getMethodColor(http.MethodPost) {
		t.Error("GET and POST are the same color")
	}
}

func TestResetColorIsTheResetSequence(t *testing.T) {
	if resetColor() != reset {
		t.Errorf("resetColor = %q, want %q", resetColor(), reset)
	}
}

// ── goroot ───────────────────────────────────────────────────────────────────

// goroot asks the installed toolchain rather than reporting the path this
// binary was built with, because the file it goes on to read is a real file on
// the machine serving.
func TestGorootPointsAtARealInstallation(t *testing.T) {
	got := goroot()
	if got == "" {
		t.Fatal("goroot is empty")
	}
	if !strings.Contains(got, "go") {
		t.Errorf("goroot = %q, which does not look like a Go installation", got)
	}
}

// ── asset table ──────────────────────────────────────────────────────────────

// The embedded pages are compiled in, so they are present before anything runs
// — a build that dropped one would serve an empty page rather than fail.
func TestEmbeddedAssetsArePresent(t *testing.T) {
	// Indexed rather than ranged by value: FileAsset carries a sync.Mutex, so
	// a range variable would copy the lock.
	for i := range htmlFiles {
		if len(readFile(htmlFiles, i)) == 0 {
			t.Errorf("%s is empty", htmlFiles[i].Name)
		}
	}
}

func TestAssetsRecordWhenTheyWereBuilt(t *testing.T) {
	for i := range htmlFiles {
		if htmlFiles[i].Built.IsZero() {
			t.Errorf("%s has no build time", htmlFiles[i].Name)
		}
		if htmlFiles[i].Built.After(time.Now().Add(time.Minute)) {
			t.Errorf("%s claims to have been built in the future", htmlFiles[i].Name)
		}
	}
}
