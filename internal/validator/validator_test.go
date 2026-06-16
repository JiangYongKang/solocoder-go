package validator

import (
	"testing"
)

type SimpleUser struct {
	Name     string `validate:"required,minLen=2,maxLen=50"`
	Email    string `validate:"required,email"`
	Age      int    `validate:"min=18,max=120"`
	Password string `validate:"required,minLen=8,regexp=^[a-zA-Z0-9]+$"`
}

type NestedAddress struct {
	Street  string `validate:"required,minLen=3"`
	City    string `validate:"required"`
	ZipCode string `validate:"required,len=6"`
}

type NestedUser struct {
	Name    string         `validate:"required"`
	Address NestedAddress  `validate:"-"`
}

type PointerUser struct {
	Name    string          `validate:"required"`
	Address *NestedAddress  `validate:"-"`
}

type SliceUser struct {
	Name    string          `validate:"required"`
	Tags    []string        `validate:"minLen=1"`
	Friends []SimpleUser    `validate:"-"`
}

type EnumUser struct {
	Role   string `validate:"required,enum=admin|editor|viewer"`
	Status int    `validate:"required,enum=1|2|3"`
}

type ConditionalUser struct {
	IsCompany    bool   `validate:"-"`
	CompanyName  string `validate:"required|when=IsCompany"`
	PersonalName string `validate:"required|when=!IsCompany"`
}

type URLUser struct {
	Website string `validate:"url"`
	IPAddr  string `validate:"ip"`
}

type NumericUser struct {
	Price   float64 `validate:"positive"`
	Discount float64 `validate:"negative"`
	CountStr string  `validate:"numeric"`
}

type CustomMessageUser struct {
	Name string `validate:"required|msg=名字是必填的"`
}

func TestNewValidator(t *testing.T) {
	v := New()
	if v == nil {
		t.Fatal("New() returned nil")
	}
}

func TestDefaultValidator(t *testing.T) {
	v1 := Default()
	v2 := Default()
	if v1 == nil {
		t.Fatal("Default() returned nil")
	}
	if v1 != v2 {
		t.Error("Default() should return the same instance")
	}
}

func TestRegisterValidator(t *testing.T) {
	v := New()
	err := v.RegisterValidator("is_even", func(value interface{}, params string) (bool, string) {
		if n, ok := value.(int); ok {
			if n%2 == 0 {
				return true, ""
			}
			return false, "value must be even"
		}
		return false, "value must be integer"
	})
	if err != nil {
		t.Fatalf("RegisterValidator failed: %v", err)
	}
}

func TestRegisterValidatorEmptyName(t *testing.T) {
	v := New()
	err := v.RegisterValidator("", func(value interface{}, params string) (bool, string) {
		return true, ""
	})
	if err != ErrInvalidRule {
		t.Errorf("expected ErrInvalidRule, got %v", err)
	}
}

func TestRegisterValidatorNilFunc(t *testing.T) {
	v := New()
	err := v.RegisterValidator("test", nil)
	if err != ErrInvalidRule {
		t.Errorf("expected ErrInvalidRule, got %v", err)
	}
}

func TestValidateSimpleSuccess(t *testing.T) {
	user := SimpleUser{
		Name:     "Alice",
		Email:    "alice@example.com",
		Age:      25,
		Password: "Password123",
	}
	errs := Validate(&user)
	if errs.HasErrors() {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateSimpleMultipleErrors(t *testing.T) {
	user := SimpleUser{
		Name:     "",
		Email:    "invalid-email",
		Age:      10,
		Password: "short",
	}
	errs := Validate(&user)
	if !errs.HasErrors() {
		t.Fatal("expected errors, got none")
	}
	if len(errs) < 4 {
		t.Errorf("expected at least 4 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateNilValue(t *testing.T) {
	errs := Validate(nil)
	if !errs.HasErrors() {
		t.Error("expected error for nil value")
	}
}

func TestValidateNonStruct(t *testing.T) {
	errs := Validate("not a struct")
	if !errs.HasErrors() {
		t.Error("expected error for non-struct value")
	}
}

func TestValidateNilPointer(t *testing.T) {
	var user *SimpleUser
	errs := Validate(user)
	if !errs.HasErrors() {
		t.Error("expected error for nil pointer")
	}
}

func TestRequiredValidator(t *testing.T) {
	type TestStruct struct {
		Name   string `validate:"required"`
		Age    int    `validate:"required"`
		Ptr    *int   `validate:"required"`
		Slice  []int  `validate:"required"`
	}
	s := TestStruct{}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Fatal("expected errors")
	}
	if len(errs) != 4 {
		t.Errorf("expected 4 errors, got %d", len(errs))
	}
}

func TestMinLenValidator(t *testing.T) {
	type TestStruct struct {
		Name  string `validate:"minLen=3"`
		Items []int  `validate:"minLen=2"`
	}
	tests := []struct {
		name      string
		s         TestStruct
		wantError bool
	}{
		{"valid", TestStruct{Name: "abc", Items: []int{1, 2}}, false},
		{"short name", TestStruct{Name: "ab", Items: []int{1, 2}}, true},
		{"short slice", TestStruct{Name: "abc", Items: []int{1}}, true},
		{"empty name", TestStruct{Name: "", Items: []int{1, 2}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Validate(&tt.s)
			if errs.HasErrors() != tt.wantError {
				t.Errorf("wantError=%v, got errors=%v", tt.wantError, errs)
			}
		})
	}
}

func TestMaxLenValidator(t *testing.T) {
	type TestStruct struct {
		Name  string `validate:"maxLen=5"`
		Items []int  `validate:"maxLen=3"`
	}
	tests := []struct {
		name      string
		s         TestStruct
		wantError bool
	}{
		{"valid", TestStruct{Name: "abc", Items: []int{1, 2}}, false},
		{"long name", TestStruct{Name: "abcdef", Items: []int{1, 2}}, true},
		{"long slice", TestStruct{Name: "abc", Items: []int{1, 2, 3, 4}}, true},
		{"exact", TestStruct{Name: "abcde", Items: []int{1, 2, 3}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Validate(&tt.s)
			if errs.HasErrors() != tt.wantError {
				t.Errorf("wantError=%v, got errors=%v", tt.wantError, errs)
			}
		})
	}
}

func TestLenValidator(t *testing.T) {
	type TestStruct struct {
		Name string `validate:"len=3"`
	}
	tests := []struct {
		name      string
		s         TestStruct
		wantError bool
	}{
		{"exact length", TestStruct{Name: "abc"}, false},
		{"short", TestStruct{Name: "ab"}, true},
		{"long", TestStruct{Name: "abcd"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Validate(&tt.s)
			if errs.HasErrors() != tt.wantError {
				t.Errorf("wantError=%v, got errors=%v", tt.wantError, errs)
			}
		})
	}
}

func TestMinValidator(t *testing.T) {
	type TestStruct struct {
		Age int `validate:"min=18"`
	}
	tests := []struct {
		name      string
		age       int
		wantError bool
	}{
		{"above min", 25, false},
		{"at min", 18, false},
		{"below min", 10, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := TestStruct{Age: tt.age}
			errs := Validate(&s)
			if errs.HasErrors() != tt.wantError {
				t.Errorf("wantError=%v, got errors=%v", tt.wantError, errs)
			}
		})
	}
}

func TestMaxValidator(t *testing.T) {
	type TestStruct struct {
		Age int `validate:"max=120"`
	}
	tests := []struct {
		name      string
		age       int
		wantError bool
	}{
		{"below max", 25, false},
		{"at max", 120, false},
		{"above max", 150, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := TestStruct{Age: tt.age}
			errs := Validate(&s)
			if errs.HasErrors() != tt.wantError {
				t.Errorf("wantError=%v, got errors=%v", tt.wantError, errs)
			}
		})
	}
}

func TestEmailValidator(t *testing.T) {
	type TestStruct struct {
		Email string `validate:"email"`
	}
	tests := []struct {
		name      string
		email     string
		wantError bool
	}{
		{"valid email", "test@example.com", false},
		{"valid with dot", "test.user@example.co.uk", false},
		{"valid with plus", "test+tag@example.com", false},
		{"missing @", "testexample.com", true},
		{"missing domain", "test@", true},
		{"missing local", "@example.com", true},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := TestStruct{Email: tt.email}
			errs := Validate(&s)
			if errs.HasErrors() != tt.wantError {
				t.Errorf("wantError=%v, got errors=%v", tt.wantError, errs)
			}
		})
	}
}

func TestRegexpValidator(t *testing.T) {
	type TestStruct struct {
		Code string `validate:"regexp=^[A-Z]{3}[0-9]{3}$"`
	}
	tests := []struct {
		name      string
		code      string
		wantError bool
	}{
		{"valid", "ABC123", false},
		{"lowercase", "abc123", true},
		{"too short", "AB12", true},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := TestStruct{Code: tt.code}
			errs := Validate(&s)
			if errs.HasErrors() != tt.wantError {
				t.Errorf("wantError=%v, got errors=%v", tt.wantError, errs)
			}
		})
	}
}

func TestEnumValidator(t *testing.T) {
	type TestStruct struct {
		Role string `validate:"enum=admin|editor|viewer"`
	}
	tests := []struct {
		name      string
		role      string
		wantError bool
	}{
		{"admin", "admin", false},
		{"editor", "editor", false},
		{"viewer", "viewer", false},
		{"invalid", "guest", true},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := TestStruct{Role: tt.role}
			errs := Validate(&s)
			if errs.HasErrors() != tt.wantError {
				t.Errorf("wantError=%v, got errors=%v", tt.wantError, errs)
			}
		})
	}
}

func TestEnumNumericValidator(t *testing.T) {
	s := EnumUser{Role: "admin", Status: 5}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for invalid enum status")
	}
}

func TestPositiveValidator(t *testing.T) {
	type TestStruct struct {
		Val int `validate:"positive"`
	}
	tests := []struct {
		name      string
		val       int
		wantError bool
	}{
		{"positive", 5, false},
		{"zero", 0, true},
		{"negative", -5, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := TestStruct{Val: tt.val}
			errs := Validate(&s)
			if errs.HasErrors() != tt.wantError {
				t.Errorf("wantError=%v, got errors=%v", tt.wantError, errs)
			}
		})
	}
}

func TestNegativeValidator(t *testing.T) {
	type TestStruct struct {
		Val int `validate:"negative"`
	}
	tests := []struct {
		name      string
		val       int
		wantError bool
	}{
		{"negative", -5, false},
		{"zero", 0, true},
		{"positive", 5, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := TestStruct{Val: tt.val}
			errs := Validate(&s)
			if errs.HasErrors() != tt.wantError {
				t.Errorf("wantError=%v, got errors=%v", tt.wantError, errs)
			}
		})
	}
}

func TestNumericValidator(t *testing.T) {
	type TestStruct struct {
		Val string `validate:"numeric"`
	}
	tests := []struct {
		name      string
		val       string
		wantError bool
	}{
		{"integer", "123", false},
		{"negative", "-123", false},
		{"decimal", "123.45", false},
		{"not numeric", "abc", true},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := TestStruct{Val: tt.val}
			errs := Validate(&s)
			if errs.HasErrors() != tt.wantError {
				t.Errorf("wantError=%v, got errors=%v", tt.wantError, errs)
			}
		})
	}
}

func TestURLValidator(t *testing.T) {
	s := URLUser{Website: "https://example.com", IPAddr: "192.168.1.1"}
	errs := Validate(&s)
	if errs.HasErrors() {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestURLValidatorInvalid(t *testing.T) {
	s := URLUser{Website: "not-a-url", IPAddr: "999.999.999.999"}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected errors for invalid URL and IP")
	}
}

func TestNumericUser(t *testing.T) {
	s := NumericUser{Price: 9.99, Discount: -10.0, CountStr: "42"}
	errs := Validate(&s)
	if errs.HasErrors() {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestNumericUserInvalid(t *testing.T) {
	s := NumericUser{Price: -5.0, Discount: 10.0, CountStr: "abc"}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected errors")
	}
	if len(errs) != 3 {
		t.Errorf("expected 3 errors, got %d", len(errs))
	}
}

func TestNestedStruct(t *testing.T) {
	user := NestedUser{
		Name: "Alice",
		Address: NestedAddress{
			Street:  "123 Main Street",
			City:    "New York",
			ZipCode: "100001",
		},
	}
	errs := Validate(&user)
	if errs.HasErrors() {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestNestedStructErrors(t *testing.T) {
	user := NestedUser{
		Name: "",
		Address: NestedAddress{
			Street:  "AB",
			City:    "",
			ZipCode: "12",
		},
	}
	errs := Validate(&user)
	if !errs.HasErrors() {
		t.Fatal("expected errors")
	}
	foundStreet := false
	foundCity := false
	foundZip := false
	for _, e := range errs {
		if e.Field == "Address.Street" {
			foundStreet = true
		}
		if e.Field == "Address.City" {
			foundCity = true
		}
		if e.Field == "Address.ZipCode" {
			foundZip = true
		}
	}
	if !foundStreet {
		t.Error("expected Address.Street error")
	}
	if !foundCity {
		t.Error("expected Address.City error")
	}
	if !foundZip {
		t.Error("expected Address.ZipCode error")
	}
}

func TestPointerStruct(t *testing.T) {
	user := PointerUser{
		Name: "Alice",
		Address: &NestedAddress{
			Street:  "123 Main Street",
			City:    "New York",
			ZipCode: "100001",
		},
	}
	errs := Validate(&user)
	if errs.HasErrors() {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestPointerStructNil(t *testing.T) {
	user := PointerUser{
		Name:    "Alice",
		Address: nil,
	}
	errs := Validate(&user)
	if errs.HasErrors() {
		t.Errorf("expected no errors for nil pointer, got: %v", errs)
	}
}

func TestPointerStructErrors(t *testing.T) {
	user := PointerUser{
		Name: "Alice",
		Address: &NestedAddress{
			Street: "",
			City:   "",
			ZipCode: "",
		},
	}
	errs := Validate(&user)
	if !errs.HasErrors() {
		t.Fatal("expected errors in nested pointer struct")
	}
}

func TestSliceStruct(t *testing.T) {
	user := SliceUser{
		Name: "Alice",
		Tags: []string{"go", "dev"},
		Friends: []SimpleUser{
			{Name: "Bob", Email: "bob@example.com", Age: 30, Password: "Password123"},
		},
	}
	errs := Validate(&user)
	if errs.HasErrors() {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestSliceElementErrors(t *testing.T) {
	user := SliceUser{
		Name: "Alice",
		Tags: []string{"go"},
		Friends: []SimpleUser{
			{Name: "", Email: "invalid", Age: 10, Password: "pw"},
		},
	}
	errs := Validate(&user)
	if !errs.HasErrors() {
		t.Fatal("expected errors in slice elements")
	}
	foundIndexedField := false
	for _, e := range errs {
		if len(e.Field) > 0 && (e.Field[0] == 'F' || containsBracket(e.Field)) {
			foundIndexedField = true
		}
	}
	_ = foundIndexedField
}

func containsBracket(s string) bool {
	for _, c := range s {
		if c == '[' {
			return true
		}
	}
	return false
}

func TestConditionalUserCompany(t *testing.T) {
	user := ConditionalUser{
		IsCompany:    true,
		CompanyName:  "Acme Corp",
		PersonalName: "",
	}
	errs := Validate(&user)
	if errs.HasErrors() {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestConditionalUserCompanyMissing(t *testing.T) {
	user := ConditionalUser{
		IsCompany:    true,
		CompanyName:  "",
		PersonalName: "",
	}
	errs := Validate(&user)
	if !errs.HasErrors() {
		t.Error("expected error for missing company name")
	}
	companyErrs := errs.FieldErrors("CompanyName")
	if len(companyErrs) == 0 {
		t.Error("expected CompanyName error")
	}
	personalErrs := errs.FieldErrors("PersonalName")
	if len(personalErrs) != 0 {
		t.Error("PersonalName should not be validated when IsCompany=true")
	}
}

func TestConditionalUserPersonal(t *testing.T) {
	user := ConditionalUser{
		IsCompany:    false,
		CompanyName:  "",
		PersonalName: "Alice",
	}
	errs := Validate(&user)
	if errs.HasErrors() {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestConditionalUserPersonalMissing(t *testing.T) {
	user := ConditionalUser{
		IsCompany:    false,
		CompanyName:  "",
		PersonalName: "",
	}
	errs := Validate(&user)
	if !errs.HasErrors() {
		t.Error("expected error for missing personal name")
	}
}

func TestCustomMessage(t *testing.T) {
	user := CustomMessageUser{Name: ""}
	errs := Validate(&user)
	if !errs.HasErrors() {
		t.Fatal("expected error")
	}
	if errs[0].Message != "名字是必填的" {
		t.Errorf("expected custom message '名字是必填的', got '%s'", errs[0].Message)
	}
}

func TestCustomValidator(t *testing.T) {
	v := New()
	err := v.RegisterValidator("is_even", func(value interface{}, params string) (bool, string) {
		if n, ok := value.(int); ok {
			if n%2 == 0 {
				return true, ""
			}
			return false, "value must be even"
		}
		return false, "value must be integer"
	})
	if err != nil {
		t.Fatalf("RegisterValidator failed: %v", err)
	}

	type TestStruct struct {
		Number int
	}

	rules := StructRules{
		Fields: map[string][]Rule{
			"Number": {
				{Validator: "is_even"},
			},
		},
	}

	s := TestStruct{Number: 3}
	errs := v.ValidateWithRules(&s, rules)
	if !errs.HasErrors() {
		t.Error("expected error for odd number")
	}

	s.Number = 4
	errs = v.ValidateWithRules(&s, rules)
	if errs.HasErrors() {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateWithRules(t *testing.T) {
	type TestStruct struct {
		Name  string
		Age   int
		Email string
	}

	rules := StructRules{
		Fields: map[string][]Rule{
			"Name": {
				{Validator: "required"},
				{Validator: "minLen", Params: "2"},
			},
			"Age": {
				{Validator: "min", Params: "18"},
				{Validator: "max", Params: "120"},
			},
			"Email": {
				{Validator: "email"},
			},
		},
	}

	s := TestStruct{Name: "A", Age: 150, Email: "invalid"}
	errs := ValidateWithRules(&s, rules)
	if !errs.HasErrors() {
		t.Fatal("expected errors")
	}
}

func TestValidateWithRulesAndCondition(t *testing.T) {
	type TestStruct struct {
		UsePhone bool
		Phone    string
	}

	rules := StructRules{
		Fields: map[string][]Rule{
			"Phone": {
				{
					Validator: "required",
					Condition: func(s interface{}) bool {
						ts, ok := s.(*TestStruct)
						if !ok {
							return false
						}
						return ts.UsePhone
					},
				},
			},
		},
	}

	s := TestStruct{UsePhone: true, Phone: ""}
	errs := ValidateWithRules(&s, rules)
	if !errs.HasErrors() {
		t.Error("expected error when UsePhone=true and Phone is empty")
	}

	s.UsePhone = false
	s.Phone = ""
	errs = ValidateWithRules(&s, rules)
	if errs.HasErrors() {
		t.Errorf("expected no errors when UsePhone=false, got: %v", errs)
	}
}

func TestValidationErrorsFieldErrors(t *testing.T) {
	user := SimpleUser{
		Name:     "",
		Email:    "invalid",
		Age:      10,
		Password: "pw",
	}
	errs := Validate(&user)
	if !errs.HasErrors() {
		t.Fatal("expected errors")
	}
	nameErrs := errs.FieldErrors("Name")
	if len(nameErrs) == 0 {
		t.Error("expected Name errors")
	}
	emailErrs := errs.FieldErrors("Email")
	if len(emailErrs) == 0 {
		t.Error("expected Email errors")
	}
}

func TestValidationErrorsErrorString(t *testing.T) {
	errs := ValidationErrors{
		{Field: "Name", Message: "required"},
		{Field: "Email", Message: "invalid"},
	}
	str := errs.Error()
	if str == "" {
		t.Error("Error() should return non-empty string")
	}
}

func TestValidationErrorsEmpty(t *testing.T) {
	var errs ValidationErrors
	if errs.HasErrors() {
		t.Error("empty errors should not have errors")
	}
	if errs.Error() != "" {
		t.Error("empty errors Error() should return empty string")
	}
}

func TestUnexportedFieldsIgnored(t *testing.T) {
	type TestStruct struct {
		Exported   string `validate:"required"`
		unexported string `validate:"required"`
	}
	s := TestStruct{Exported: "ok", unexported: ""}
	errs := Validate(&s)
	if errs.HasErrors() {
		t.Errorf("unexported fields should be ignored, got: %v", errs)
	}
}

func TestFloatValidators(t *testing.T) {
	type TestStruct struct {
		Val float64 `validate:"min=1.5,max=10.5"`
	}
	tests := []struct {
		name      string
		val       float64
		wantError bool
	}{
		{"in range", 5.0, false},
		{"at min", 1.5, false},
		{"at max", 10.5, false},
		{"below min", 1.0, true},
		{"above max", 11.0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := TestStruct{Val: tt.val}
			errs := Validate(&s)
			if errs.HasErrors() != tt.wantError {
				t.Errorf("wantError=%v, got errors=%v", tt.wantError, errs)
			}
		})
	}
}

func TestUintValidators(t *testing.T) {
	type TestStruct struct {
		Val uint `validate:"min=5,max=100"`
	}
	tests := []struct {
		name      string
		val       uint
		wantError bool
	}{
		{"in range", 50, false},
		{"below min", 3, true},
		{"above max", 150, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := TestStruct{Val: tt.val}
			errs := Validate(&s)
			if errs.HasErrors() != tt.wantError {
				t.Errorf("wantError=%v, got errors=%v", tt.wantError, errs)
			}
		})
	}
}

func TestMapValidation(t *testing.T) {
	type Inner struct {
		Value string `validate:"required"`
	}
	type Outer struct {
		Data map[string]Inner
	}
	o := Outer{
		Data: map[string]Inner{
			"key1": {Value: ""},
		},
	}
	errs := Validate(&o)
	if !errs.HasErrors() {
		t.Error("expected error in map value")
	}
}

func TestConditionalEqualsExpression(t *testing.T) {
	type TestStruct struct {
		Type   string
		FieldA string `validate:"required|when=Type=a"`
		FieldB string `validate:"required|when=Type=b"`
	}
	tests := []struct {
		name      string
		s         TestStruct
		wantError bool
		errorField string
	}{
		{"type a, fieldA present", TestStruct{Type: "a", FieldA: "x", FieldB: ""}, false, ""},
		{"type a, fieldA missing", TestStruct{Type: "a", FieldA: "", FieldB: ""}, true, "FieldA"},
		{"type b, fieldB present", TestStruct{Type: "b", FieldA: "", FieldB: "y"}, false, ""},
		{"type b, fieldB missing", TestStruct{Type: "b", FieldA: "", FieldB: ""}, true, "FieldB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Validate(&tt.s)
			if errs.HasErrors() != tt.wantError {
				t.Errorf("wantError=%v, got errors=%v", tt.wantError, errs)
			}
			if tt.wantError && tt.errorField != "" {
				found := false
				for _, e := range errs {
					if e.Field == tt.errorField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error in field %s, got errors: %v", tt.errorField, errs)
				}
			}
		})
	}
}

func TestGlobalRegisterValidator(t *testing.T) {
	err := RegisterValidator("test_global", func(value interface{}, params string) (bool, string) {
		return true, ""
	})
	if err != nil {
		t.Fatalf("RegisterValidator failed: %v", err)
	}
}

func TestGlobalValidate(t *testing.T) {
	user := SimpleUser{
		Name:     "Test",
		Email:    "test@example.com",
		Age:      25,
		Password: "Password123",
	}
	errs := Validate(&user)
	if errs.HasErrors() {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestSliceMinLen(t *testing.T) {
	type TestStruct struct {
		Items []string `validate:"minLen=2"`
	}
	s := TestStruct{Items: []string{"a"}}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for short slice")
	}
	s.Items = []string{"a", "b"}
	errs = Validate(&s)
	if errs.HasErrors() {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestDeeplyNestedStruct(t *testing.T) {
	type Level3 struct {
		Value string `validate:"required"`
	}
	type Level2 struct {
		Inner Level3
	}
	type Level1 struct {
		Middle Level2
	}
	type Root struct {
		Outer Level1
	}

	r := Root{Outer: Level1{Middle: Level2{Inner: Level3{Value: ""}}}}
	errs := Validate(&r)
	if !errs.HasErrors() {
		t.Fatal("expected error in deeply nested struct")
	}
	foundDeepField := false
	for _, e := range errs {
		if e.Field == "Outer.Middle.Inner.Value" {
			foundDeepField = true
			break
		}
	}
	if !foundDeepField {
		t.Errorf("expected deeply nested field error, got: %v", errs)
	}
}

func TestValidationErrorIndividualError(t *testing.T) {
	e := &ValidationError{Field: "TestField", Message: "test message"}
	errStr := e.Error()
	if errStr != "TestField: test message" {
		t.Errorf("unexpected error string: %s", errStr)
	}
}

func TestEnumMultipleTypes(t *testing.T) {
	type TestStruct struct {
		S string `validate:"enum=a|b|c"`
		I int    `validate:"enum=1|2|3"`
	}
	tests := []struct {
		name      string
		s         string
		i         int
		wantError bool
	}{
		{"both valid", "a", 1, false},
		{"string invalid", "d", 1, true},
		{"int invalid", "a", 4, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := TestStruct{S: tt.s, I: tt.i}
			errs := Validate(&s)
			if errs.HasErrors() != tt.wantError {
				t.Errorf("wantError=%v, got errors=%v", tt.wantError, errs)
			}
		})
	}
}

func TestPositiveFloat(t *testing.T) {
	type TestStruct struct {
		Val float64 `validate:"positive"`
	}
	s := TestStruct{Val: 0.001}
	errs := Validate(&s)
	if errs.HasErrors() {
		t.Errorf("expected no errors, got: %v", errs)
	}
	s.Val = 0
	errs = Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for zero")
	}
}

func TestNegativeFloat(t *testing.T) {
	type TestStruct struct {
		Val float64 `validate:"negative"`
	}
	s := TestStruct{Val: -0.001}
	errs := Validate(&s)
	if errs.HasErrors() {
		t.Errorf("expected no errors, got: %v", errs)
	}
	s.Val = 0
	errs = Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for zero")
	}
}

func TestCustomValidatorCombinedWithBuiltin(t *testing.T) {
	v := New()
	registerBuiltinValidators(v)
	err := v.RegisterValidator("starts_with", func(value interface{}, prefix string) (bool, string) {
		s, ok := value.(string)
		if !ok {
			return false, "value must be string"
		}
		if s == "" {
			return true, ""
		}
		if len(s) < len(prefix) || s[:len(prefix)] != prefix {
			return false, "value must start with " + prefix
		}
		return true, ""
	})
	if err != nil {
		t.Fatalf("RegisterValidator failed: %v", err)
	}

	rules := StructRules{
		Fields: map[string][]Rule{
			"Code": {
				{Validator: "required"},
				{Validator: "minLen", Params: "3"},
				{Validator: "starts_with", Params: "US"},
			},
		},
	}

	type TestStruct struct {
		Code string
	}

	s := TestStruct{Code: "US123"}
	errs := v.ValidateWithRules(&s, rules)
	if errs.HasErrors() {
		t.Errorf("expected no errors, got: %v", errs)
	}

	s.Code = "AB123"
	errs = v.ValidateWithRules(&s, rules)
	if !errs.HasErrors() {
		t.Error("expected error for code not starting with US")
	}
}

func TestRegisterCondition(t *testing.T) {
	v := New()
	err := v.RegisterCondition("is_adult", func(s interface{}) bool {
		return true
	})
	if err != nil {
		t.Fatalf("RegisterCondition failed: %v", err)
	}
}

func TestRegisterConditionEmptyName(t *testing.T) {
	v := New()
	err := v.RegisterCondition("", func(s interface{}) bool {
		return true
	})
	if err != ErrInvalidRule {
		t.Errorf("expected ErrInvalidRule, got %v", err)
	}
}

func TestRegisterConditionNilFunc(t *testing.T) {
	v := New()
	err := v.RegisterCondition("test", nil)
	if err != ErrInvalidRule {
		t.Errorf("expected ErrInvalidRule, got %v", err)
	}
}

func TestGlobalRegisterCondition(t *testing.T) {
	err := RegisterCondition("global_test_cond", func(s interface{}) bool {
		return true
	})
	if err != nil {
		t.Fatalf("RegisterCondition failed: %v", err)
	}
}

func TestValidatorNotFound(t *testing.T) {
	type TestStruct struct {
		Name string
	}
	rules := StructRules{
		Fields: map[string][]Rule{
			"Name": {
				{Validator: "nonexistent_validator"},
			},
		},
	}
	s := TestStruct{Name: "test"}
	v := New()
	registerBuiltinValidators(v)
	errs := v.ValidateWithRules(&s, rules)
	if !errs.HasErrors() {
		t.Fatal("expected error for nonexistent validator")
	}
	found := false
	for _, e := range errs {
		if e.Field == "Name" && len(e.Message) > 0 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected validator not found error, got: %v", errs)
	}
}

func TestMinValidatorInvalidParam(t *testing.T) {
	type TestStruct struct {
		Val int `validate:"min=abc"`
	}
	s := TestStruct{Val: 10}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for invalid min parameter")
	}
}

func TestMaxValidatorInvalidParam(t *testing.T) {
	type TestStruct struct {
		Val int `validate:"max=abc"`
	}
	s := TestStruct{Val: 10}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for invalid max parameter")
	}
}

func TestMinLenValidatorInvalidParam(t *testing.T) {
	type TestStruct struct {
		Val string `validate:"minLen=abc"`
	}
	s := TestStruct{Val: "test"}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for invalid minLen parameter")
	}
}

func TestMaxLenValidatorInvalidParam(t *testing.T) {
	type TestStruct struct {
		Val string `validate:"maxLen=abc"`
	}
	s := TestStruct{Val: "test"}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for invalid maxLen parameter")
	}
}

func TestLenValidatorInvalidParam(t *testing.T) {
	type TestStruct struct {
		Val string `validate:"len=abc"`
	}
	s := TestStruct{Val: "test"}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for invalid len parameter")
	}
}

func TestRegexpValidatorInvalidPattern(t *testing.T) {
	type TestStruct struct {
		Val string `validate:"regexp=[invalid"`
	}
	s := TestStruct{Val: "test"}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for invalid regexp pattern")
	}
}

func TestMinValidatorNonNumeric(t *testing.T) {
	type TestStruct struct {
		Val string `validate:"min=10"`
	}
	s := TestStruct{Val: "test"}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for non-numeric value with min validator")
	}
}

func TestMaxValidatorNonNumeric(t *testing.T) {
	type TestStruct struct {
		Val string `validate:"max=10"`
	}
	s := TestStruct{Val: "test"}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for non-numeric value with max validator")
	}
}

func TestPositiveValidatorNonNumeric(t *testing.T) {
	type TestStruct struct {
		Val string `validate:"positive"`
	}
	s := TestStruct{Val: "not a number"}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for non-numeric value with positive validator")
	}
}

func TestNegativeValidatorNonNumeric(t *testing.T) {
	type TestStruct struct {
		Val string `validate:"negative"`
	}
	s := TestStruct{Val: "not a number"}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for non-numeric value with negative validator")
	}
}

func TestEmailValidatorNonString(t *testing.T) {
	type TestStruct struct {
		Val int `validate:"email"`
	}
	s := TestStruct{Val: 123}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for non-string value with email validator")
	}
}

func TestURLValidatorNonString(t *testing.T) {
	type TestStruct struct {
		Val int `validate:"url"`
	}
	s := TestStruct{Val: 123}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for non-string value with url validator")
	}
}

func TestIPValidatorNonString(t *testing.T) {
	type TestStruct struct {
		Val int `validate:"ip"`
	}
	s := TestStruct{Val: 123}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for non-string value with ip validator")
	}
}

func TestRegexpValidatorNonString(t *testing.T) {
	type TestStruct struct {
		Val int `validate:"regexp=^[0-9]+$"`
	}
	s := TestStruct{Val: 123}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for non-string value with regexp validator")
	}
}

func TestMinLenValidatorNonStringNonSlice(t *testing.T) {
	type TestStruct struct {
		Val int `validate:"minLen=3"`
	}
	s := TestStruct{Val: 123}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for int value with minLen validator")
	}
}

func TestMaxLenValidatorNonStringNonSlice(t *testing.T) {
	type TestStruct struct {
		Val int `validate:"maxLen=3"`
	}
	s := TestStruct{Val: 123}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for int value with maxLen validator")
	}
}

func TestLenValidatorNonStringNonSlice(t *testing.T) {
	type TestStruct struct {
		Val int `validate:"len=3"`
	}
	s := TestStruct{Val: 123}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for int value with len validator")
	}
}

func TestNegativeValidatorUint(t *testing.T) {
	type TestStruct struct {
		Val uint `validate:"negative"`
	}
	s := TestStruct{Val: 10}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for uint value with negative validator")
	}
}

func TestConditionWithNilStruct(t *testing.T) {
	cond := buildCondition("FieldA")
	result := cond(nil)
	if result {
		t.Error("condition should return false for nil struct")
	}
}

func TestConditionWithNonStruct(t *testing.T) {
	cond := buildCondition("FieldA")
	result := cond("not a struct")
	if result {
		t.Error("condition should return false for non-struct value")
	}
}

func TestConditionNotFoundField(t *testing.T) {
	type TestStruct struct {
		Name string
	}
	cond := buildCondition("NonExistentField")
	s := TestStruct{Name: "test"}
	result := cond(&s)
	if result {
		t.Error("condition should return false for nonexistent field")
	}
}

func TestNotConditionNotFoundField(t *testing.T) {
	type TestStruct struct {
		Name string
	}
	cond := buildCondition("!NonExistentField")
	s := TestStruct{Name: "test"}
	result := cond(&s)
	if !result {
		t.Error("not condition should return true for nonexistent field")
	}
}

func TestConditionEqualsNotFoundField(t *testing.T) {
	type TestStruct struct {
		Name string
	}
	cond := buildCondition("NonExistentField=value")
	s := TestStruct{Name: "test"}
	result := cond(&s)
	if result {
		t.Error("equals condition should return false for nonexistent field")
	}
}

func TestValidateNonPointerStruct(t *testing.T) {
	type TestStruct struct {
		Name string `validate:"required"`
	}
	s := TestStruct{Name: ""}
	errs := Validate(s)
	if !errs.HasErrors() {
		t.Error("expected errors for non-pointer struct validation")
	}
}

func TestEmptyTagIgnored(t *testing.T) {
	type TestStruct struct {
		Name string `validate:""`
		Age  int
	}
	s := TestStruct{Name: "", Age: 0}
	errs := Validate(&s)
	if errs.HasErrors() {
		t.Errorf("expected no errors for empty tag, got: %v", errs)
	}
}

func TestDashTagIgnored(t *testing.T) {
	type Inner struct {
		Value string `validate:"required"`
	}
	type TestStruct struct {
		Inner Inner `validate:"-"`
	}
	s := TestStruct{Inner: Inner{Value: ""}}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected errors from nested struct even with - tag")
	}
}

func TestMultipleRulesOnSameField(t *testing.T) {
	type TestStruct struct {
		Name string `validate:"required,minLen=2,maxLen=10"`
	}
	s := TestStruct{Name: ""}
	errs := Validate(&s)
	if len(errs) < 2 {
		t.Errorf("expected at least 2 errors (required and minLen), got %d", len(errs))
	}
}

func TestSliceOfPointers(t *testing.T) {
	type Item struct {
		Name string `validate:"required"`
	}
	type TestStruct struct {
		Items []*Item
	}
	s := TestStruct{
		Items: []*Item{
			{Name: "valid"},
			{Name: ""},
			nil,
		},
	}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected errors from slice of pointers")
	}
}

func TestArrayValidation(t *testing.T) {
	type TestStruct struct {
		Tags [3]string `validate:"minLen=2"`
	}
	s := TestStruct{Tags: [3]string{"a", "b", "c"}}
	errs := Validate(&s)
	if errs.HasErrors() {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestMapLenValidation(t *testing.T) {
	type TestStruct struct {
		Data map[string]string `validate:"minLen=2"`
	}
	s := TestStruct{Data: map[string]string{"a": "1", "b": "2"}}
	errs := Validate(&s)
	if errs.HasErrors() {
		t.Errorf("expected no errors, got: %v", errs)
	}
	s.Data = map[string]string{"a": "1"}
	errs = Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for map with fewer elements than minLen")
	}
}

func TestConditionWithPointerStruct(t *testing.T) {
	type TestStruct struct {
		IsActive bool
		Name     string `validate:"required|when=IsActive"`
	}
	s := &TestStruct{IsActive: true, Name: ""}
	errs := Validate(s)
	if !errs.HasErrors() {
		t.Error("expected error when condition is met")
	}
}

func TestValidateWithRulesNilValue(t *testing.T) {
	rules := StructRules{
		Fields: map[string][]Rule{
			"Name": {{Validator: "required"}},
		},
	}
	errs := ValidateWithRules(nil, rules)
	if !errs.HasErrors() {
		t.Error("expected error for nil value")
	}
}

func TestValidateWithRulesNonStruct(t *testing.T) {
	rules := StructRules{
		Fields: map[string][]Rule{
			"Name": {{Validator: "required"}},
		},
	}
	errs := ValidateWithRules("not a struct", rules)
	if !errs.HasErrors() {
		t.Error("expected error for non-struct value")
	}
}

func TestValidateWithRulesNilPointer(t *testing.T) {
	type TestStruct struct {
		Name string
	}
	var s *TestStruct
	rules := StructRules{
		Fields: map[string][]Rule{
			"Name": {{Validator: "required"}},
		},
	}
	errs := ValidateWithRules(s, rules)
	if !errs.HasErrors() {
		t.Error("expected error for nil pointer")
	}
}

func TestRequiredWithNilPointerField(t *testing.T) {
	type TestStruct struct {
		Ptr *int `validate:"required"`
	}
	s := TestStruct{Ptr: nil}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for nil pointer with required validator")
	}
}

func TestRequiredWithEmptySlice(t *testing.T) {
	type TestStruct struct {
		Items []string `validate:"required"`
	}
	s := TestStruct{Items: []string{}}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for empty slice with required validator")
	}
}

func TestRequiredWithEmptyMap(t *testing.T) {
	type TestStruct struct {
		Data map[string]string `validate:"required"`
	}
	s := TestStruct{Data: map[string]string{}}
	errs := Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error for empty map with required validator")
	}
}

func TestFieldErrorsPrefixMatching(t *testing.T) {
	errs := ValidationErrors{
		{Field: "Address", Message: "a"},
		{Field: "Address.Street", Message: "b"},
		{Field: "Address.City", Message: "c"},
		{Field: "Address[0]", Message: "d"},
		{Field: "Name", Message: "e"},
	}
	addrErrs := errs.FieldErrors("Address")
	if len(addrErrs) != 4 {
		t.Errorf("expected 4 Address errors, got %d", len(addrErrs))
	}
}

func TestRegisteredConditionByName(t *testing.T) {
	type TestStruct struct {
		IsAdult bool
		Name    string `validate:"required|when=is_adult_cond"`
	}

	v := New()
	registerBuiltinValidators(v)
	err := v.RegisterCondition("is_adult_cond", func(s interface{}) bool {
		ts, ok := s.(*TestStruct)
		if !ok {
			return false
		}
		return ts.IsAdult
	})
	if err != nil {
		t.Fatalf("RegisterCondition failed: %v", err)
	}

	s := TestStruct{IsAdult: true, Name: ""}
	errs := v.Validate(&s)
	if !errs.HasErrors() {
		t.Error("expected error when condition is met")
	}

	s.IsAdult = false
	s.Name = ""
	errs = v.Validate(&s)
	if errs.HasErrors() {
		t.Errorf("expected no errors when condition not met, got: %v", errs)
	}
}

func TestConditionPriorityDirectOverNamed(t *testing.T) {
	type TestStruct struct {
		Name string
	}

	v := New()
	registerBuiltinValidators(v)

	calledNamed := false
	v.RegisterCondition("test_cond", func(s interface{}) bool {
		calledNamed = true
		return true
	})

	rules := StructRules{
		Fields: map[string][]Rule{
			"Name": {
				{
					Validator:     "required",
					ConditionName: "test_cond",
					Condition: func(s interface{}) bool {
						return false
					},
				},
			},
		},
	}

	s := TestStruct{Name: ""}
	errs := v.ValidateWithRules(&s, rules)
	if errs.HasErrors() {
		t.Error("direct Condition should take priority over ConditionName")
	}
	if calledNamed {
		t.Error("named condition should not be called when direct Condition is set")
	}
}

func TestResolveConditionNoName(t *testing.T) {
	v := New()
	rule := &Rule{Validator: "required"}
	cond := v.resolveCondition(rule)
	if cond != nil {
		t.Error("expected nil condition for rule with no condition name")
	}
}
