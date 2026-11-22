package clip

import (
	"reflect"
	"testing"
)

func TestFakeKeepsWhatWasCopied(t *testing.T) {
	var f Fake
	f.CopySecret("a")
	f.CopySecret("b")
	if got := f.Copied(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("copied %v", got)
	}
}
