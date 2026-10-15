package validator

import (
	"errors"
	"slices"
	"testing"
)

type address struct {
	City string `validate:"required"`
	Zip  string `validate:"min=5,max=9"`
}

type user struct {
	Name    string   `validate:"required,min=3,max=10"`
	Email   string   `validate:"required,email"`
	Age     int      `validate:"min=18,max=120"`
	Role    string   `validate:"oneof=admin editor viewer"`
	Tags    []string `validate:"max=2"`
	Nick    *string  `validate:"min=2"`
	Address address
	secret  string `validate:"required"`
	Skipped string `validate:"-"`
}

func valid() user {
	return user{
		Name: "Ana", Email: "ana@example.com", Age: 30, Role: "admin",
		Address: address{City: "Recife", Zip: "50000"},
	}
}

// violations flattens an error into sorted "Field:rule" strings.
func violations(t *testing.T, err error) []string {
	t.Helper()
	var errs Errors
	if !errors.As(err, &errs) {
		t.Fatalf("error %v is not Errors", err)
	}
	var out []string
	for _, fe := range errs {
		out = append(out, fe.Field+":"+fe.Rule)
	}
	slices.Sort(out)
	return out
}

func TestValidate(t *testing.T) {
	short, long := "x", "ok"
	tests := []struct {
		name   string
		mutate func(u *user)
		want   []string // nil means valid
	}{
		{"valid", func(u *user) {}, nil},
		{"empty optional fields are skipped", func(u *user) { u.Age, u.Role, u.Address.Zip = 0, "", "" }, nil},
		{"missing required", func(u *user) { u.Name, u.Email = "", "" }, []string{"Email:required", "Name:required"}},
		{"too short", func(u *user) { u.Name = "Al" }, []string{"Name:min"}},
		{"too long", func(u *user) { u.Name = "abcdefghijk" }, []string{"Name:max"}},
		{"length counts runes, not bytes", func(u *user) { u.Name = "João" }, nil},
		{"number below min", func(u *user) { u.Age = 17 }, []string{"Age:min"}},
		{"number above max", func(u *user) { u.Age = 121 }, []string{"Age:max"}},
		{"invalid email", func(u *user) { u.Email = "not-an-email" }, []string{"Email:email"}},
		{"email with display name is rejected", func(u *user) { u.Email = "Ana <ana@example.com>" }, []string{"Email:email"}},
		{"role outside the list", func(u *user) { u.Role = "root" }, []string{"Role:oneof"}},
		{"too many tags", func(u *user) { u.Tags = []string{"a", "b", "c"} }, []string{"Tags:max"}},
		{"pointer is dereferenced", func(u *user) { u.Nick = &short }, []string{"Nick:min"}},
		{"valid pointer", func(u *user) { u.Nick = &long }, nil},
		{"nested struct path", func(u *user) { u.Address = address{Zip: "1"} }, []string{"Address.City:required", "Address.Zip:min"}},
		{"several violations at once", func(u *user) { u.Name, u.Age, u.Role = "A", 5, "x" }, []string{"Age:min", "Name:min", "Role:oneof"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := valid()
			tt.mutate(&u)
			err := Validate(u)
			if tt.want == nil {
				if err != nil {
					t.Fatalf("Validate = %v, want nil", err)
				}
				return
			}
			if got := violations(t, err); !slices.Equal(got, tt.want) {
				t.Errorf("violations = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateAcceptsPointerAndRejectsOthers(t *testing.T) {
	u := valid()
	if err := Validate(&u); err != nil {
		t.Errorf("Validate(&u) = %v, want nil", err)
	}
	var nilUser *user
	for name, v := range map[string]any{"int": 5, "nil pointer": nilUser, "nil": nil} {
		if err := Validate(v); !errors.Is(err, ErrNotStruct) {
			t.Errorf("%s: err = %v, want ErrNotStruct", name, err)
		}
	}
}

func TestErrorMessages(t *testing.T) {
	u := valid()
	u.Name, u.Age = "", 5
	err := Validate(u)
	want := "Name: failed required; Age: failed min=18"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestBadTagsPanic(t *testing.T) {
	tests := map[string]any{
		"unknown rule": struct {
			A string `validate:"bogus"`
		}{A: "x"},
		"bad parameter": struct {
			A string `validate:"min=abc"`
		}{A: "x"},
	}
	for name, v := range tests {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("expected a panic")
				}
			}()
			_ = Validate(v)
		})
	}
}
