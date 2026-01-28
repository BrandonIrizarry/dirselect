package dirselect

import (
	"testing"

	"github.com/charmbracelet/x/exp/teatest"
)

func TestModel(t *testing.T) {
	m, err := New()
	if err != nil {
		t.Fatal(err)
	}

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))
	tm.FinalModel(t)
}
