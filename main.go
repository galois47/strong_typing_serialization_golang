// Demonstrates JSON serialization and deserialization of a strongly typed Go
// struct tree. The types nest two levels deep: Company at the root, with
// Address and Employee beneath it by value and in a slice.
//
// Encoding and decoding are encoding/json's job. Required fields are not: an
// absent key leaves the zero value and is not an error, so required-ness is
// declared with go-playground/validator tags, and serialize and deserialize
// each call validate.Struct themselves. The program prints five steps: a round
// trip (1-3), what happens when required keys are absent (4), and what each
// layer rejects (5).
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"

	"github.com/go-playground/validator/v10"
)

// Address is a level-2 type: scalars only. The json tags name the JSON keys
// ("omitempty" on Zip leaves it out when empty); the validate tags declare
// what must be present.
//
// Every required field is a pointer. "required" means non-zero, and for a
// pointer the zero value is nil, so a required *string rejects only an absent
// key: an empty string still arrives as a non-nil pointer. A required plain
// string could not tell those apart. Zip, which is optional, stays a string.
type Address struct {
	Street *string `json:"street" validate:"required"`
	City   *string `json:"city" validate:"required"`
	Zip    string  `json:"zip,omitempty"`
}

// Employee is the other level-2 type. A manager is referenced by id rather
// than by a nested Employee, so nothing is nested below this level. ID carries
// a value rule as well as a presence rule: "gt=0" rejects a negative or zero
// id that decoded perfectly well.
type Employee struct {
	ID        *int    `json:"id" validate:"required,gt=0"`
	Name      *string `json:"name" validate:"required"`
	ManagerID *int    `json:"manager_id,omitempty"`
}

// Company is the root of the tree, level 1. "dive" tells the validator to
// check each element of the slice, not just the slice itself.
type Company struct {
	Name      *string    `json:"name" validate:"required"`
	HQ        Address    `json:"hq" validate:"required"`
	Employees []Employee `json:"employees" validate:"required,dive"`
}

// validate is the tag-driven checker from go-playground/validator, called by
// both serialize and deserialize so neither direction can skip it. One
// instance is created per program and reused: it caches its reflection work
// per type, is safe for concurrent use, and is cheap to call.
var validate = validator.New(validator.WithRequiredStructEnabled())

func main() {
	demoRoundTrip()
	demoMissingField()
	demoBadTypes()
}

// demoRoundTrip serializes a value, reads it back, and confirms nothing was
// lost on the way.
func demoRoundTrip() {
	original := sampleCompany()

	data, err := serialize(original)
	if err != nil {
		log.Fatalf("serialize: %v", err)
	}
	fmt.Printf("1. Serialized (struct tree -> JSON):\n%s\n\n", data)

	back, err := deserialize(data)
	if err != nil {
		log.Fatalf("deserialize: %v", err)
	}
	fmt.Println("2. Deserialized (JSON -> struct tree); reaching into the nesting:")
	printHighlights(back)

	fmt.Printf("\n3. Round-trip identical: %t\n\n", reflect.DeepEqual(original, back))
}

// serialize turns the struct tree into indented JSON, refusing to emit a value
// whose validate tags are unsatisfied.
func serialize(c Company) ([]byte, error) {
	if err := validate.Struct(c); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal company: %w", err)
	}
	return data, nil
}

// deserialize parses JSON back into the typed struct tree, then checks the
// validate tags. Two layers reject input here, in order: json.Unmarshal
// refuses JSON whose types do not fit the fields, and validate.Struct refuses
// values that decoded fine but break a tag. The decoder owns types because the
// validator cannot see them: it runs on a value whose fields are already
// typed.
func deserialize(data []byte) (Company, error) {
	var c Company
	if err := json.Unmarshal(data, &c); err != nil {
		return Company{}, fmt.Errorf("unmarshal company: %w", err)
	}
	if err := validate.Struct(c); err != nil {
		return Company{}, err
	}
	return c, nil
}

// sampleCompany builds the value the demo round-trips. new takes an
// expression as of Go 1.26, which is what fills the pointer fields from
// literals; adaID is a named local because two fields share that one pointer.
func sampleCompany() Company {
	adaID := 1
	return Company{
		Name: new("Analytical Engines Ltd"),
		HQ: Address{
			Street: new("1 Clerkenwell Rd"),
			City:   new("London"),
			Zip:    "EC1M",
		},
		Employees: []Employee{
			{ID: &adaID, Name: new("Ada Lovelace")},
			{ID: new(2), Name: new("Charles Babbage"), ManagerID: &adaID},
		},
	}
}

// printHighlights reads values at both levels of the decoded tree. The
// dereferences are safe because deserialize has already rejected a nil in any
// field tagged required.
func printHighlights(c Company) {
	babbage := c.Employees[1]
	fmt.Printf("   hq city:            %s\n", *c.HQ.City)
	fmt.Printf("   employee 1 name:    %s\n", *babbage.Name)
	fmt.Printf("   employee 1 mgr id:  %d\n", *babbage.ManagerID)
}

// demoMissingField decodes plain JSON with required keys left out, at the root
// and at both nested levels. Nothing here names a field: the tags decide what
// is required and deserialize enforces them on the way through.
func demoMissingField() {
	cases := []struct {
		desc string
		json string
	}{
		{"name absent", `{"hq": {"street": "5 Fleet St", "city": "London"}, "employees": []}`},
		{"hq.street absent", `{"name": "Difference Works", "hq": {"city": "London"}, "employees": []}`},
		{"employee name absent", `{"name": "Difference Works", "hq": {"street": "5 Fleet St", "city": "London"}, "employees": [{"id": 9}]}`},
	}

	fmt.Println("4. Decoding JSON with required keys absent:")
	for _, tc := range cases {
		_, err := deserialize([]byte(tc.json))
		fmt.Printf("   %-24s %v\n", tc.desc+":", err)
	}
}

// demoBadTypes runs both rejection layers past each other. The first five
// cases are wrong types or malformed JSON, which json.Unmarshal stops; the
// last decodes cleanly and is stopped by the validator instead, on the "gt=0"
// rule from Employee.ID.
func demoBadTypes() {
	cases := []struct {
		desc string
		json string
	}{
		{"string where int belongs", `{"name": "DW", "hq": {"street": "5 Fleet St", "city": "London"}, "employees": [{"id": "nine", "name": "J"}]}`},
		{"number where string belongs", `{"name": 1066, "hq": {"street": "5 Fleet St", "city": "London"}, "employees": []}`},
		{"array where object belongs", `{"name": "DW", "hq": ["5 Fleet St", "London"], "employees": []}`},
		{"int overflow", `{"name": "DW", "hq": {"street": "5 Fleet St", "city": "London"}, "employees": [{"id": 99999999999999999999, "name": "J"}]}`},
		{"truncated JSON", `{"name": "DW", "hq": {"street": "5 Fle`},
		{"right type, bad value", `{"name": "DW", "hq": {"street": "5 Fleet St", "city": "London"}, "employees": [{"id": -5, "name": "J"}]}`},
	}

	fmt.Println("\n5. Rejected on the way in:")
	for _, tc := range cases {
		_, err := deserialize([]byte(tc.json))
		fmt.Printf("   %-28s %v\n", tc.desc+":", err)
	}
}
