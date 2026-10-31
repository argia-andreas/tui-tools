package main

import (
	"io"
	"testing"
)

func TestModeValidation(t *testing.T) {
	for _, args := range [][]string{nil, {"--fixture", "--config", "x"}, {"--config", "x"}, {"--fixture", "--connection", "x"}, {"--fixture", "extra"}} {
		if code := run(args, io.Discard); code != 2 {
			t.Errorf("%v: code %d", args, code)
		}
	}
	if run([]string{"--help"}, io.Discard) != 0 {
		t.Fatal("help failed")
	}
}
