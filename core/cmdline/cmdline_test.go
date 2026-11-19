package cmdline

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	l := Parse([]string{"pb", "--root=/r", "-picker", "https://a.test", "--profile=p1", "--profile=p2", "--version", "--", "--not-a-switch", "b"})
	if l.Value("root") != "/r" || !l.Has("picker") || l.Value("picker") != "" {
		t.Errorf("root %q picker %v", l.Value("root"), l.Has("picker"))
	}
	if l.Value("profile") != "p2" {
		t.Errorf("the last of a name wins: got %q", l.Value("profile"))
	}
	if !l.Has("version") || l.Has("not-a-switch") {
		t.Errorf("version %v, after --: %v", l.Has("version"), l.Has("not-a-switch"))
	}
	if want := []string{"https://a.test", "--not-a-switch", "b"}; !reflect.DeepEqual(l.Args(), want) {
		t.Errorf("args %q, want %q", l.Args(), want)
	}
}

func TestParseValueWithEquals(t *testing.T) {
	l := Parse([]string{"pb", "--user-data-dir=/a=b", "-", "x"})
	if l.Value("user-data-dir") != "/a=b" {
		t.Errorf("got %q", l.Value("user-data-dir"))
	}
	if want := []string{"-", "x"}; !reflect.DeepEqual(l.Args(), want) {
		t.Errorf("a lone - is an argument: args %q", l.Args())
	}
}

func TestWith(t *testing.T) {
	l := Parse([]string{"pb", "--pb-extensions=off", "--", "https://a.test"})
	got := l.With("--disable-in-process-stack-traces", "--pb-extensions", "--pb-x=1", "--pb-x=2")
	want := []string{"pb", "--disable-in-process-stack-traces", "--pb-x=1", "--pb-extensions=off", "--", "https://a.test"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestWithNothing(t *testing.T) {
	if got := Parse([]string{"pb"}).With(); !reflect.DeepEqual(got, []string{"pb"}) {
		t.Errorf("got %q", got)
	}
}
