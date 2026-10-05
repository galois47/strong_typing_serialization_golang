# strong_typing_serialization_golang

A single-file Go program that round-trips a strongly typed struct tree through
JSON: it serializes a `Company` value, deserializes it back into the same type,
and then shows what happens to input that is incomplete, wrongly typed or
malformed. Required fields are declared with struct tags and enforced inside the
serialize/deserialize calls, because `encoding/json` has no concept of a
required field.

## Requirements

- **Go 1.26 or newer** — `go.mod` declares `go 1.26.0`, so an older toolchain
  will refuse to build. Check with `go version`.
- One dependency, [`github.com/go-playground/validator/v10`](https://github.com/go-playground/validator),
  for the required-field checking. `go run .` fetches it automatically on the
  first build (it needs network access once); `go mod download` fetches it
  ahead of time. Everything else is the standard library.

Install Go from <https://go.dev/dl/> if `go version` is not found, or use your
package manager (`sudo apt install golang-go`, `brew install go`, …) as long as
it ships 1.26+.

## Running it

From the repository root:

```sh
go run .
```

That compiles and runs in one step without leaving a binary behind. To build an
executable instead:

```sh
go build -o json-demo .
./json-demo
```

## What you should see

Five numbered sections on stdout:

1. **Serialized** — the `Company` value as indented JSON.
2. **Deserialized** — values read back out of the decoded tree at both levels,
   including through the slice and the `*int` pointer.
3. **Round-trip identical: true** — every field survived the trip.
   `reflect.DeepEqual` follows pointers, so the pointer fields compare by the
   values they point at, not by address.
4. **Decoding JSON with required keys absent** — three inputs, each missing a
   required key at a different level (root, nested struct, nested slice
   element), reported by the validator's own error:
   `Key: 'Company.HQ.Street' Error:Field validation for 'Street' failed on the
   'required' tag`.
5. **Rejected on the way in** — six inputs showing which layer rejects what:
   five wrong-type or malformed payloads caught by `json.Unmarshal`, and one
   correctly typed but invalid value (`"id": -5`) caught by the validator.

## Required fields

`encoding/json` has no notion of a required field — an absent key just leaves
the zero value, and `json/v2` (Go 1.26, behind `GOEXPERIMENT=jsonv2`) has no
`required` tag option either. So required-ness is declared with
[go-playground/validator](https://github.com/go-playground/validator) tags, and
every required field is a **pointer**:

```go
type Company struct {
	Name      *string    `json:"name" validate:"required"`
	HQ        Address    `json:"hq" validate:"required"`
	Employees []Employee `json:"employees" validate:"required,dive"`
}
```

The pointer matters: `required` means *non-zero*, so a required `string` would
reject `{"name": ""}` exactly as it rejects an absent `name`. For a pointer the
zero value is `nil`, and an empty string still decodes into a non-nil pointer —
so a required `*string` rejects only a key that was never sent, while
`{"name": ""}` is accepted as "sent, but empty". Optional fields such as `Zip`
stay plain strings.

`dive` makes the validator check each element of the slice rather than only the
slice itself; nested structs such as `HQ` are walked automatically. Both
`serialize` and `deserialize` call `validate.Struct` directly, so no per-field
`if` exists anywhere and tagging a new field is the only change needed to
enforce it.

The validator's error is returned unchanged — it already names the struct, the
field path and the tag that failed. The `validate` instance is a package-level
variable created once and reused: it caches its reflection per type and is safe
for concurrent use.

### Types vs. values

The validator does **not** check types, and cannot: `validate.Struct` runs on
an already-decoded Go value, where the types are guaranteed by the compiler. A
wrong JSON type is rejected one step earlier, by `json.Unmarshal`:

```
json: cannot unmarshal string into Go struct field Employee.employees.id of type int
```

What the validator adds is rules about *values* that happen to be well typed —
`gt`, `lt`, `min`, `max`, `len`, `oneof`, `email`, `url`, `uuid`, and so on.
`Employee.ID` carries `validate:"required,gt=0"`, so `{"id": -5}` decodes
cleanly and is then rejected:

```
Key: 'Company.Employees[0].ID' Error:Field validation for 'ID' failed on the 'gt' tag
```

Step 5 of the output runs both layers past each other.

## Checks

```sh
gofmt -l .      # no output means formatting is clean
go vet ./...    # no output means no suspect constructs
```

## Layout

| File | Contents |
| --- | --- |
| `main.go` | the types (`Company`, `Address`, `Employee`, three fields each); `serialize`/`deserialize`, each guarded by `validate.Struct`; `sampleCompany` and `printHighlights`; and the three demos `demoRoundTrip`, `demoMissingField`, `demoBadTypes`, called in that order by `main` |
| `go.mod`, `go.sum` | module declaration, Go version, and the validator dependency |
