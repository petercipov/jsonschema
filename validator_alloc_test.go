package jsonschema_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// manyObjects is an instance of n objects of five properties each, like the
// reply of a list call, and the schema it is checked against.
func manyObjects(t testing.TB, n int) (*jsonschema.Schema, any, any) {
	t.Helper()
	schema := `{
		"type": "object",
		"required": ["Parameters"],
		"properties": {"Parameters": {"type": "array", "items": {"$ref": "#/$defs/parameter"}}},
		"$defs": {"parameter": {
			"type": "object",
			"required": ["Name", "Type"],
			"properties": {
				"Name": {"type": "string", "minLength": 1, "maxLength": 2048},
				"Type": {"enum": ["String", "StringList", "SecureString"]},
				"Value": {"type": "string"},
				"Version": {"type": "integer"},
				"DataType": {"type": "string"}
			}
		}}
	}`
	sch, err := jsonschema.UnmarshalJSON(strings.NewReader(schema))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema.json", sch); err != nil {
		t.Fatal(err)
	}
	compiled, err := c.Compile("schema.json")
	if err != nil {
		t.Fatal(err)
	}
	build := func(typ any) any {
		var b strings.Builder
		b.WriteString(`{"Parameters":[`)
		for i := 0; i < n; i++ {
			if i > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, `{"Name":"/shop/param-%d","Type":%q,"Value":"value-%0200d","Version":1,"DataType":"text"}`, i, typ, i)
		}
		b.WriteString(`]}`)
		v, err := jsonschema.UnmarshalJSON(strings.NewReader(b.String()))
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	return compiled, build("String"), build("Wrong")
}

func BenchmarkValidateManyObjects(b *testing.B) {
	sch, good, _ := manyObjects(b, 400)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := sch.Validate(good); err != nil {
			b.Fatal(err)
		}
	}
}

// The checks run in parallel must give the same answers as one at a time.
func TestValidateManyObjectsInParallel(t *testing.T) {
	sch, good, bad := manyObjects(t, 50)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				if err := sch.Validate(good); err != nil {
					t.Error("good value rejected:", err)
					return
				}
				err := sch.Validate(bad)
				if err == nil {
					t.Error("bad value accepted")
					return
				}
				if ve, ok := err.(*jsonschema.ValidationError); !ok || len(ve.Causes) == 0 {
					t.Error("expected causes in the error, got", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
