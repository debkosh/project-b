package content

import (
	"reflect"
	"testing"
)

// pack lays Go values out the way the adapter does, so a test can fake a crossing.
// The generated structs declare their fields in the adapter's order.
func pack(values ...any) []Value {
	var out []Value
	for _, v := range values {
		out = append(out, packOne(reflect.ValueOf(v))...)
	}
	return out
}

func packOne(v reflect.Value) []Value {
	switch v.Kind() {
	case reflect.Bool:
		return []Value{boolValue(v.Bool())}
	case reflect.Int, reflect.Int32, reflect.Int64:
		return []Value{{I: v.Int()}}
	case reflect.Float64:
		return []Value{{D: v.Float()}}
	case reflect.String:
		return []Value{{S: v.String()}}
	case reflect.Slice:
		out := []Value{{I: int64(v.Len())}}
		for i := 0; i < v.Len(); i++ {
			out = append(out, packOne(v.Index(i))...)
		}
		return out
	case reflect.Struct:
		var out []Value
		for i := 0; i < v.NumField(); i++ {
			out = append(out, packOne(v.Field(i))...)
		}
		return out
	}
	panic("cannot pack " + v.Kind().String())
}

// No hook crosses until something subscribes to it: the app is told through the
// adapter's events alone (core/abi).
func TestNothingIsSubscribedUntilAsked(t *testing.T) {
	subscribed := map[Hook]bool{}
	New(func(id Hook) { subscribed[id] = true })
	if len(subscribed) != 0 {
		t.Fatalf("subscribed to %v", subscribed)
	}
}

// A hook crosses for whoever subscribes to it.
func TestASubscribedHookReachesItsHandler(t *testing.T) {
	subscribed := map[Hook]bool{}
	h := New(func(id Hook) { subscribed[id] = true })
	var got []float64
	h.Observer.OnLoadProgressChanged(func(c WebContents, progress float64) { got = append(got, progress) })
	if !subscribed[ObserverLoadProgressChanged] {
		t.Fatal("not subscribed")
	}
	h.Dispatch(ObserverLoadProgressChanged, pack(WebContents{ID: 2}, 0.4))
	if !reflect.DeepEqual(got, []float64{0.4}) {
		t.Fatalf("got %v", got)
	}
}
