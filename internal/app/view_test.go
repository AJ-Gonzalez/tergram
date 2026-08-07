package app

import (
	"testing"

	"tergram/internal/tgc"
)

func TestOneLine(t *testing.T) {
	cases := []struct{ in, want string }{
		{"line one\n\nline two\tand  more", "line one line two and more"},
		{"   \n  \t ", ""},
		{"already one line", "already one line"},
	}
	for _, c := range cases {
		if got := oneLine(c.in); got != c.want {
			t.Errorf("oneLine(%q): got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestListWindowKeepsCursorVisible(t *testing.T) {
	d := make([]tgc.Dialog, 10)
	for i := range d {
		d[i] = tgc.Dialog{ID: int64(i), Title: "t"}
	}
	m := New(&fakeClient{})
	m.dialogs = d
	m.height = 6 // visible = height-4 = 2 rows

	m.listIdx = 0
	start, end := m.listWindow()
	if start != 0 || end != 1 {
		t.Fatalf("cursor 0: want [0,1], got [%d,%d]", start, end)
	}

	m.listIdx = 5
	start, end = m.listWindow()
	if start != 4 || end != 5 {
		t.Fatalf("cursor 5: want [4,5], got [%d,%d]", start, end)
	}

	m.listIdx = 9
	start, end = m.listWindow()
	if start != 8 || end != 9 {
		t.Fatalf("cursor 9: want [8,9], got [%d,%d]", start, end)
	}

	m.listIdx = 99 // clamped to last row
	start, end = m.listWindow()
	if start != 8 || end != 9 {
		t.Fatalf("cursor 99: want [8,9], got [%d,%d]", start, end)
	}
}
