package pdfout

import (
	"fmt"
	"reflect"
	"testing"
)

func TestApplyOverridesFieldTypes(t *testing.T) {
	base := DefaultOptions()

	cases := []struct {
		name  string
		set   OptionsOverrides
		check func(Options) bool
	}{
		{"bool false over true", OptionsOverrides{Images: Ptr(false)}, func(o Options) bool { return !o.Images }},
		{"bool true over false", OptionsOverrides{RunningHeader: Ptr(true)}, func(o Options) bool { return o.RunningHeader }},
		{"float zero over non-zero", OptionsOverrides{FontSize: Ptr(0.0)}, func(o Options) bool { return o.FontSize == 0 }},
		{"float over default", OptionsOverrides{LineSpacing: Ptr(2.0)}, func(o Options) bool { return o.LineSpacing == 2.0 }},
		{"string empty over non-empty", OptionsOverrides{Font: Ptr("")}, func(o Options) bool { return o.Font == "" }},
		{"string over default", OptionsOverrides{Font: Ptr("mono")}, func(o Options) bool { return o.Font == "mono" }},
		{"struct", OptionsOverrides{PageSize: Ptr(pageSizes["letter"])}, func(o Options) bool { return o.PageSize.Name == "letter" }},
		{"margins", OptionsOverrides{Margins: Ptr(UniformMargins(30))}, func(o Options) bool { return o.Margins == UniformMargins(30) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := base.ApplyOverrides(tc.set)
			if !tc.check(got) {
				t.Errorf("override not applied: %+v", got)
			}
		})
	}
}

func TestApplyOverridesUnsetKeepsBase(t *testing.T) {
	base := DefaultOptions()
	if got := base.ApplyOverrides(OptionsOverrides{}); !reflect.DeepEqual(got, base) {
		t.Errorf("an empty override changed the options:\n got %+v\nwant %+v", got, base)
	}

	got := base.ApplyOverrides(OptionsOverrides{FontSize: Ptr(14.0)})
	want := base
	want.FontSize = 14
	if !reflect.DeepEqual(got, want) {
		t.Errorf("only FontSize should change:\n got %+v\nwant %+v", got, want)
	}
}

func TestApplyOverridesDoesNotMutateReceiver(t *testing.T) {
	base := DefaultOptions()
	before := base
	_ = base.ApplyOverrides(OverridesFrom(Options{FontSize: 20, Font: "sans"}))
	if !reflect.DeepEqual(base, before) {
		t.Errorf("the receiver was modified: %+v", base)
	}
}

// TestOverridesCoverEveryOption fails when a field is added to Options without
// being added to OptionsOverrides, ApplyOverrides and OverridesFrom.
func TestOverridesCoverEveryOption(t *testing.T) {
	optType := reflect.TypeFor[Options]()
	ovType := reflect.TypeFor[OptionsOverrides]()

	for i := 0; i < optType.NumField(); i++ {
		f := optType.Field(i)
		of, ok := ovType.FieldByName(f.Name)
		if !ok {
			t.Errorf("OptionsOverrides has no field %s", f.Name)
			continue
		}
		if of.Type != reflect.PointerTo(f.Type) {
			t.Errorf("OptionsOverrides.%s is %s, want *%s", f.Name, of.Type, f.Type)
		}
	}
	if ovType.NumField() != optType.NumField() {
		t.Errorf("OptionsOverrides has %d fields, Options has %d", ovType.NumField(), optType.NumField())
	}

	// Give every field a distinct value the zero Options does not have, then
	// push it through each path and expect it back. Bools can only be true, so
	// a field wired to the wrong source is caught by the one-field-at-a-time
	// pass below instead.
	want := Options{}
	n := 0
	fillNonZero(reflect.ValueOf(&want).Elem(), &n)

	if got := (Options{}).ApplyOverrides(OverridesFrom(want)); !reflect.DeepEqual(got, want) {
		t.Errorf("OverridesFrom then ApplyOverrides lost a field: got %+v, want %+v", got, want)
	}

	for i := 0; i < optType.NumField(); i++ {
		name := optType.Field(i).Name

		// ApplyOverrides must honour this field.
		var p OptionsOverrides
		field := reflect.ValueOf(&p).Elem().FieldByName(name)
		src := reflect.ValueOf(want).FieldByName(name)
		ptr := reflect.New(src.Type())
		ptr.Elem().Set(src)
		field.Set(ptr)

		got := (Options{}).ApplyOverrides(p)
		if !reflect.DeepEqual(reflect.ValueOf(got).FieldByName(name).Interface(), src.Interface()) {
			t.Errorf("ApplyOverrides ignores %s", name)
		}

		// OverridesFrom must carry this field and no other.
		var only Options
		reflect.ValueOf(&only).Elem().FieldByName(name).Set(src)
		if got := (Options{}).ApplyOverrides(OverridesFrom(only)); !reflect.DeepEqual(got, only) {
			t.Errorf("OverridesFrom mishandles %s: got %+v, want %+v", name, got, only)
		}
	}
}

// fillNonZero gives every leaf a different non-zero value, and panics on a
// kind it does not know so that a new field type cannot be skipped silently.
func fillNonZero(v reflect.Value, n *int) {
	switch v.Kind() {
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Float64:
		*n++
		v.SetFloat(float64(*n) + 0.5)
	case reflect.String:
		*n++
		v.SetString(fmt.Sprintf("s%d", *n))
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			fillNonZero(v.Field(i), n)
		}
	default:
		panic(fmt.Sprintf("fillNonZero: unhandled kind %s", v.Kind()))
	}
}
