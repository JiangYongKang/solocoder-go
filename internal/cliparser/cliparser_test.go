package cliparser

import (
	"errors"
	"testing"
)

func TestNewParser(t *testing.T) {
	p := NewParser("testapp")
	if p == nil {
		t.Fatal("expected non-nil parser")
	}
	if p.AppName != "testapp" {
		t.Errorf("expected app name testapp, got %s", p.AppName)
	}
	if len(p.Options) != 0 {
		t.Errorf("expected 0 options, got %d", len(p.Options))
	}
	if len(p.Commands) != 0 {
		t.Errorf("expected 0 commands, got %d", len(p.Commands))
	}
}

func TestAddOption_Nil(t *testing.T) {
	p := NewParser("test")
	err := p.AddOption(nil)
	if !errors.Is(err, ErrNilOption) {
		t.Errorf("expected ErrNilOption, got %v", err)
	}
}

func TestAddOption_NilTarget(t *testing.T) {
	p := NewParser("test")
	var s string
	opt := &Option{
		Long:   "name",
		Short:  "n",
		Type:   StringType,
		Target: nil,
	}
	_ = s
	err := p.AddOption(opt)
	if !errors.Is(err, ErrNilTarget) {
		t.Errorf("expected ErrNilTarget, got %v", err)
	}
}

func TestAddOption_NoName(t *testing.T) {
	p := NewParser("test")
	var s string
	opt := &Option{
		Type:   StringType,
		Target: &s,
	}
	err := p.AddOption(opt)
	if !errors.Is(err, ErrOptionNoName) {
		t.Errorf("expected ErrOptionNoName, got %v", err)
	}
}

func TestAddOption_InvalidShort(t *testing.T) {
	p := NewParser("test")
	var s string
	opt := &Option{
		Short:  "ab",
		Type:   StringType,
		Target: &s,
	}
	err := p.AddOption(opt)
	if !errors.Is(err, ErrShortOptionFormat) {
		t.Errorf("expected ErrShortOptionFormat, got %v", err)
	}
}

func TestAddOption_DuplicateLong(t *testing.T) {
	p := NewParser("test")
	var s1, s2 string
	err := p.AddOption(&Option{Long: "name", Type: StringType, Target: &s1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddOption(&Option{Long: "name", Type: StringType, Target: &s2})
	if !errors.Is(err, ErrDuplicateOption) {
		t.Errorf("expected ErrDuplicateOption, got %v", err)
	}
}

func TestAddOption_DuplicateShort(t *testing.T) {
	p := NewParser("test")
	var s1, s2 string
	err := p.AddOption(&Option{Short: "n", Type: StringType, Target: &s1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddOption(&Option{Short: "n", Type: StringType, Target: &s2})
	if !errors.Is(err, ErrDuplicateOption) {
		t.Errorf("expected ErrDuplicateOption, got %v", err)
	}
}

func TestParse_LongOption_String(t *testing.T) {
	p := NewParser("test")
	var name string
	err := p.AddOption(&Option{
		Long:   "name",
		Short:  "n",
		Type:   StringType,
		Target: &name,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"--name", "Alice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Alice" {
		t.Errorf("expected Alice, got %s", name)
	}
}

func TestParse_LongOption_WithEquals(t *testing.T) {
	p := NewParser("test")
	var name string
	err := p.AddOption(&Option{
		Long:   "name",
		Type:   StringType,
		Target: &name,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"--name=Bob"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Bob" {
		t.Errorf("expected Bob, got %s", name)
	}
}

func TestParse_ShortOption_String(t *testing.T) {
	p := NewParser("test")
	var name string
	err := p.AddOption(&Option{
		Long:   "name",
		Short:  "n",
		Type:   StringType,
		Target: &name,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"-n", "Charlie"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Charlie" {
		t.Errorf("expected Charlie, got %s", name)
	}
}

func TestParse_ShortOption_CombinedValue(t *testing.T) {
	p := NewParser("test")
	var name string
	err := p.AddOption(&Option{
		Short:  "n",
		Type:   StringType,
		Target: &name,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"-nDave"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Dave" {
		t.Errorf("expected Dave, got %s", name)
	}
}

func TestParse_BoolOption(t *testing.T) {
	p := NewParser("test")
	var verbose bool
	err := p.AddOption(&Option{
		Long:   "verbose",
		Short:  "v",
		Type:   BoolType,
		Target: &verbose,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"--verbose"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !verbose {
		t.Error("expected verbose to be true")
	}
}

func TestParse_BoolOption_Short(t *testing.T) {
	p := NewParser("test")
	var verbose bool
	err := p.AddOption(&Option{
		Long:   "verbose",
		Short:  "v",
		Type:   BoolType,
		Target: &verbose,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"-v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !verbose {
		t.Error("expected verbose to be true")
	}
}

func TestParse_BoolOption_DefaultFalse(t *testing.T) {
	p := NewParser("test")
	var verbose bool
	err := p.AddOption(&Option{
		Long:   "verbose",
		Type:   BoolType,
		Target: &verbose,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verbose {
		t.Error("expected verbose to be false by default")
	}
}

func TestParse_BoolOption_ExplicitValue(t *testing.T) {
	p := NewParser("test")
	var flag bool
	err := p.AddOption(&Option{
		Long:   "flag",
		Type:   BoolType,
		Target: &flag,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"--flag=false"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if flag {
		t.Error("expected flag to be false")
	}
}

func TestParse_IntOption(t *testing.T) {
	p := NewParser("test")
	var count int
	err := p.AddOption(&Option{
		Long:   "count",
		Short:  "c",
		Type:   IntType,
		Target: &count,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"--count", "42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 42 {
		t.Errorf("expected 42, got %d", count)
	}
}

func TestParse_IntOption_Invalid(t *testing.T) {
	p := NewParser("test")
	var count int
	err := p.AddOption(&Option{
		Long:   "count",
		Type:   IntType,
		Target: &count,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"--count", "notanumber"})
	if !errors.Is(err, ErrInvalidType) {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}
}

func TestParse_FloatOption(t *testing.T) {
	p := NewParser("test")
	var ratio float64
	err := p.AddOption(&Option{
		Long:   "ratio",
		Type:   FloatType,
		Target: &ratio,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"--ratio", "3.14"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ratio != 3.14 {
		t.Errorf("expected 3.14, got %f", ratio)
	}
}

func TestParse_FloatOption_Invalid(t *testing.T) {
	p := NewParser("test")
	var ratio float64
	err := p.AddOption(&Option{
		Long:   "ratio",
		Type:   FloatType,
		Target: &ratio,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"--ratio", "abc"})
	if !errors.Is(err, ErrInvalidType) {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}
}

func TestParse_CombinedShortBooleans(t *testing.T) {
	p := NewParser("test")
	var v, d, f bool
	err := p.AddOption(&Option{Short: "v", Type: BoolType, Target: &v})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddOption(&Option{Short: "d", Type: BoolType, Target: &d})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddOption(&Option{Short: "f", Type: BoolType, Target: &f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"-vdf"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v || !d || !f {
		t.Errorf("expected all true: v=%v d=%v f=%v", v, d, f)
	}
}

func TestParse_CombinedShortWithValue(t *testing.T) {
	p := NewParser("test")
	var v, f bool
	var name string
	err := p.AddOption(&Option{Short: "v", Type: BoolType, Target: &v})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddOption(&Option{Short: "f", Type: BoolType, Target: &f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddOption(&Option{Short: "n", Type: StringType, Target: &name})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"-vfn", "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v || !f {
		t.Errorf("expected v and f true: v=%v f=%v", v, f)
	}
	if name != "hello" {
		t.Errorf("expected hello, got %s", name)
	}
}

func TestParse_DefaultValue(t *testing.T) {
	p := NewParser("test")
	var name string
	err := p.AddOption(&Option{
		Long:    "name",
		Type:    StringType,
		Default: "default_name",
		Target:  &name,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "default_name" {
		t.Errorf("expected default_name, got %s", name)
	}
}

func TestParse_DefaultValue_Overridden(t *testing.T) {
	p := NewParser("test")
	var name string
	err := p.AddOption(&Option{
		Long:    "name",
		Type:    StringType,
		Default: "default_name",
		Target:  &name,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"--name", "override"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "override" {
		t.Errorf("expected override, got %s", name)
	}
}

func TestParse_UnknownOption(t *testing.T) {
	p := NewParser("test")
	err := p.Parse([]string{"--unknown"})
	if !errors.Is(err, ErrUnknownOption) {
		t.Errorf("expected ErrUnknownOption, got %v", err)
	}
}

func TestParse_UnknownShortOption(t *testing.T) {
	p := NewParser("test")
	err := p.Parse([]string{"-x"})
	if !errors.Is(err, ErrUnknownOption) {
		t.Errorf("expected ErrUnknownOption, got %v", err)
	}
}

func TestParse_MissingValue(t *testing.T) {
	p := NewParser("test")
	var name string
	err := p.AddOption(&Option{
		Long:   "name",
		Type:   StringType,
		Target: &name,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"--name"})
	if !errors.Is(err, ErrMissingValue) {
		t.Errorf("expected ErrMissingValue, got %v", err)
	}
}

func TestParse_MissingValue_Short(t *testing.T) {
	p := NewParser("test")
	var name string
	err := p.AddOption(&Option{
		Short:  "n",
		Type:   StringType,
		Target: &name,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"-n"})
	if !errors.Is(err, ErrMissingValue) {
		t.Errorf("expected ErrMissingValue, got %v", err)
	}
}

func TestParse_LongAndShortBinding(t *testing.T) {
	p := NewParser("test")
	var verbose bool
	err := p.AddOption(&Option{
		Long:   "verbose",
		Short:  "v",
		Type:   BoolType,
		Target: &verbose,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"-v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !verbose {
		t.Error("expected verbose true via short option")
	}

	verbose = false
	err = p.Parse([]string{"--verbose"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !verbose {
		t.Error("expected verbose true via long option")
	}
}

func TestParse_MultipleOptions(t *testing.T) {
	p := NewParser("test")
	var name string
	var count int
	var verbose bool
	err := p.AddOption(&Option{Long: "name", Short: "n", Type: StringType, Target: &name})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddOption(&Option{Long: "count", Short: "c", Type: IntType, Target: &count})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddOption(&Option{Long: "verbose", Short: "v", Type: BoolType, Target: &verbose})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"-v", "--name=Alice", "-c", "10"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Alice" {
		t.Errorf("expected Alice, got %s", name)
	}
	if count != 10 {
		t.Errorf("expected 10, got %d", count)
	}
	if !verbose {
		t.Error("expected verbose true")
	}
}

func TestParse_SubCommand_Basic(t *testing.T) {
	p := NewParser("test")
	addCmd := NewCommand("add")
	var addName string
	err := addCmd.AddOption(&Option{Long: "name", Short: "n", Type: StringType, Target: &addName})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	addHandlerCalled := false
	addCmd.Handler = func() error {
		addHandlerCalled = true
		return nil
	}
	err = p.AddCommand(addCmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = p.Parse([]string{"add", "--name", "item1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addName != "item1" {
		t.Errorf("expected item1, got %s", addName)
	}
	cmd := p.GetCommand()
	if cmd == nil || cmd.Name != "add" {
		t.Error("expected add command")
	}
	err = p.Execute()
	if err != nil {
		t.Fatalf("unexpected execute error: %v", err)
	}
	if !addHandlerCalled {
		t.Error("expected add handler to be called")
	}
}

func TestParse_SubCommand_Unknown(t *testing.T) {
	p := NewParser("test")
	addCmd := NewCommand("add")
	err := p.AddCommand(addCmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"unknown"})
	if !errors.Is(err, ErrUnknownCommand) {
		t.Errorf("expected ErrUnknownCommand, got %v", err)
	}
}

func TestParse_SubCommand_NoCommand(t *testing.T) {
	p := NewParser("test")
	addCmd := NewCommand("add")
	err := p.AddCommand(addCmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{})
	if !errors.Is(err, ErrNoCommand) {
		t.Errorf("expected ErrNoCommand, got %v", err)
	}
}

func TestParse_SubCommand_RootOptions(t *testing.T) {
	p := NewParser("test")
	var verbose bool
	err := p.AddOption(&Option{Long: "verbose", Short: "v", Type: BoolType, Target: &verbose})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	listCmd := NewCommand("list")
	var filter string
	err = listCmd.AddOption(&Option{Long: "filter", Short: "f", Type: StringType, Target: &filter})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddCommand(listCmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = p.Parse([]string{"-v", "list", "-f", "active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !verbose {
		t.Error("expected verbose true")
	}
	if filter != "active" {
		t.Errorf("expected active, got %s", filter)
	}
}

func TestParse_SubCommand_MultipleCommands(t *testing.T) {
	p := NewParser("test")
	var addItem, removeItem, listFilter string

	addCmd := NewCommand("add")
	err := addCmd.AddOption(&Option{Long: "item", Type: StringType, Target: &addItem})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddCommand(addCmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	removeCmd := NewCommand("remove")
	err = removeCmd.AddOption(&Option{Long: "item", Type: StringType, Target: &removeItem})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddCommand(removeCmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	listCmd := NewCommand("list")
	err = listCmd.AddOption(&Option{Long: "filter", Type: StringType, Target: &listFilter})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddCommand(listCmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = p.Parse([]string{"add", "--item", "apple"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addItem != "apple" {
		t.Errorf("expected apple, got %s", addItem)
	}
	if removeItem != "" {
		t.Errorf("expected empty, got %s", removeItem)
	}
}

func TestParse_PositionalArgs_String(t *testing.T) {
	p := NewParser("test")
	var src, dst string
	err := p.AddPositionalArg(&PositionalArg{
		Name:   "source",
		Type:   StringType,
		Target: &src,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddPositionalArg(&PositionalArg{
		Name:   "dest",
		Type:   StringType,
		Target: &dst,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = p.Parse([]string{"from.txt", "to.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src != "from.txt" {
		t.Errorf("expected from.txt, got %s", src)
	}
	if dst != "to.txt" {
		t.Errorf("expected to.txt, got %s", dst)
	}
}

func TestParse_PositionalArgs_Int(t *testing.T) {
	p := NewParser("test")
	var a, b int
	err := p.AddPositionalArg(&PositionalArg{Name: "a", Type: IntType, Target: &a})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddPositionalArg(&PositionalArg{Name: "b", Type: IntType, Target: &b})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = p.Parse([]string{"10", "20"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a != 10 || b != 20 {
		t.Errorf("expected 10,20 got %d,%d", a, b)
	}
}

func TestParse_PositionalArgs_TooFew(t *testing.T) {
	p := NewParser("test")
	var a, b string
	err := p.AddPositionalArg(&PositionalArg{Name: "a", Type: StringType, Target: &a})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddPositionalArg(&PositionalArg{Name: "b", Type: StringType, Target: &b})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = p.Parse([]string{"only_one"})
	if !errors.Is(err, ErrTooFewArgs) {
		t.Errorf("expected ErrTooFewArgs, got %v", err)
	}
}

func TestParse_PositionalArgs_TooMany(t *testing.T) {
	p := NewParser("test")
	var a string
	err := p.AddPositionalArg(&PositionalArg{Name: "a", Type: StringType, Target: &a})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = p.Parse([]string{"one", "two"})
	if !errors.Is(err, ErrTooManyArgs) {
		t.Errorf("expected ErrTooManyArgs, got %v", err)
	}
}

func TestParse_PositionalArgs_InvalidInt(t *testing.T) {
	p := NewParser("test")
	var a int
	err := p.AddPositionalArg(&PositionalArg{Name: "a", Type: IntType, Target: &a})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = p.Parse([]string{"notint"})
	if !errors.Is(err, ErrInvalidType) {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}
}

func TestParse_PositionalArgs_WithOptions(t *testing.T) {
	p := NewParser("test")
	var verbose bool
	var src, dst string
	err := p.AddOption(&Option{Long: "verbose", Short: "v", Type: BoolType, Target: &verbose})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddPositionalArg(&PositionalArg{Name: "src", Type: StringType, Target: &src})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddPositionalArg(&PositionalArg{Name: "dst", Type: StringType, Target: &dst})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = p.Parse([]string{"-v", "src.txt", "dst.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !verbose {
		t.Error("expected verbose true")
	}
	if src != "src.txt" || dst != "dst.txt" {
		t.Errorf("expected src.txt,dst.txt got %s,%s", src, dst)
	}
}

func TestParse_PositionalArgs_AfterDoubleDash(t *testing.T) {
	p := NewParser("test")
	var verbose bool
	var arg1, arg2 string
	err := p.AddOption(&Option{Long: "verbose", Short: "v", Type: BoolType, Target: &verbose})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddPositionalArg(&PositionalArg{Name: "a", Type: StringType, Target: &arg1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddPositionalArg(&PositionalArg{Name: "b", Type: StringType, Target: &arg2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = p.Parse([]string{"-v", "--", "-notanoption", "normal"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !verbose {
		t.Error("expected verbose true")
	}
	if arg1 != "-notanoption" || arg2 != "normal" {
		t.Errorf("expected -notanoption,normal got %s,%s", arg1, arg2)
	}
}

func TestParse_SubCommand_PositionalArgs(t *testing.T) {
	p := NewParser("test")
	addCmd := NewCommand("add")
	var item string
	var qty int
	err := addCmd.AddPositionalArg(&PositionalArg{Name: "item", Type: StringType, Target: &item})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = addCmd.AddPositionalArg(&PositionalArg{Name: "qty", Type: IntType, Target: &qty})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddCommand(addCmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = p.Parse([]string{"add", "apple", "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item != "apple" {
		t.Errorf("expected apple, got %s", item)
	}
	if qty != 5 {
		t.Errorf("expected 5, got %d", qty)
	}
}

func TestParse_SubCommand_PositionalArgs_TooFew(t *testing.T) {
	p := NewParser("test")
	addCmd := NewCommand("add")
	var item, qty string
	err := addCmd.AddPositionalArg(&PositionalArg{Name: "item", Type: StringType, Target: &item})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = addCmd.AddPositionalArg(&PositionalArg{Name: "qty", Type: StringType, Target: &qty})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddCommand(addCmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = p.Parse([]string{"add", "onlyone"})
	if !errors.Is(err, ErrTooFewArgs) {
		t.Errorf("expected ErrTooFewArgs, got %v", err)
	}
}

func TestParse_SubCommand_DefaultValues(t *testing.T) {
	p := NewParser("test")
	listCmd := NewCommand("list")
	var format string
	var limit int
	err := listCmd.AddOption(&Option{
		Long:    "format",
		Type:    StringType,
		Default: "table",
		Target:  &format,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = listCmd.AddOption(&Option{
		Long:    "limit",
		Type:    IntType,
		Default: 50,
		Target:  &limit,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddCommand(listCmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = p.Parse([]string{"list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != "table" {
		t.Errorf("expected table, got %s", format)
	}
	if limit != 50 {
		t.Errorf("expected 50, got %d", limit)
	}
}

func TestExecute_NoHandler(t *testing.T) {
	p := NewParser("test")
	listCmd := NewCommand("list")
	err := p.AddCommand(listCmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Execute()
	if !errors.Is(err, ErrNoHandler) {
		t.Errorf("expected ErrNoHandler, got %v", err)
	}
}

func TestExecute_NoCommand(t *testing.T) {
	p := NewParser("test")
	err := p.Execute()
	if !errors.Is(err, ErrCommandNotFound) {
		t.Errorf("expected ErrCommandNotFound, got %v", err)
	}
}

func TestExecute_HandlerError(t *testing.T) {
	p := NewParser("test")
	expectedErr := errors.New("handler error")
	cmd := NewCommand("run")
	cmd.Handler = func() error {
		return expectedErr
	}
	err := p.AddCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"run"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Execute()
	if err != expectedErr {
		t.Errorf("expected handler error, got %v", err)
	}
}

func TestAddCommand_Nil(t *testing.T) {
	p := NewParser("test")
	err := p.AddCommand(nil)
	if !errors.Is(err, ErrNilCommand) {
		t.Errorf("expected ErrNilCommand, got %v", err)
	}
}

func TestAddCommand_EmptyName(t *testing.T) {
	p := NewParser("test")
	cmd := NewCommand("")
	err := p.AddCommand(cmd)
	if !errors.Is(err, ErrCommandNoName) {
		t.Errorf("expected ErrCommandNoName, got %v", err)
	}
}

func TestAddCommand_Duplicate(t *testing.T) {
	p := NewParser("test")
	err := p.AddCommand(NewCommand("add"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddCommand(NewCommand("add"))
	if !errors.Is(err, ErrDuplicateCommand) {
		t.Errorf("expected ErrDuplicateCommand, got %v", err)
	}
}

func TestAddPositionalArg_Nil(t *testing.T) {
	p := NewParser("test")
	err := p.AddPositionalArg(nil)
	if !errors.Is(err, ErrNilArg) {
		t.Errorf("expected ErrNilArg, got %v", err)
	}
}

func TestAddPositionalArg_NilTarget(t *testing.T) {
	p := NewParser("test")
	err := p.AddPositionalArg(&PositionalArg{Name: "a", Type: StringType})
	if !errors.Is(err, ErrNilTarget) {
		t.Errorf("expected ErrNilTarget, got %v", err)
	}
}

func TestCommand_AddOption_Nil(t *testing.T) {
	c := NewCommand("test")
	err := c.AddOption(nil)
	if !errors.Is(err, ErrNilOption) {
		t.Errorf("expected ErrNilOption, got %v", err)
	}
}

func TestCommand_AddOption_NilTarget(t *testing.T) {
	c := NewCommand("test")
	err := c.AddOption(&Option{Long: "n", Type: StringType})
	if !errors.Is(err, ErrNilTarget) {
		t.Errorf("expected ErrNilTarget, got %v", err)
	}
}

func TestCommand_AddOption_Duplicate(t *testing.T) {
	c := NewCommand("test")
	var s1, s2 string
	err := c.AddOption(&Option{Long: "name", Type: StringType, Target: &s1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = c.AddOption(&Option{Long: "name", Type: StringType, Target: &s2})
	if !errors.Is(err, ErrDuplicateOption) {
		t.Errorf("expected ErrDuplicateOption, got %v", err)
	}
}

func TestCommand_AddPositionalArg_Nil(t *testing.T) {
	c := NewCommand("test")
	err := c.AddPositionalArg(nil)
	if !errors.Is(err, ErrNilArg) {
		t.Errorf("expected ErrNilArg, got %v", err)
	}
}

func TestCommand_AddPositionalArg_NilTarget(t *testing.T) {
	c := NewCommand("test")
	err := c.AddPositionalArg(&PositionalArg{Name: "a", Type: StringType})
	if !errors.Is(err, ErrNilTarget) {
		t.Errorf("expected ErrNilTarget, got %v", err)
	}
}

func TestParse_Default_Int(t *testing.T) {
	p := NewParser("test")
	var port int
	err := p.AddOption(&Option{
		Long:    "port",
		Type:    IntType,
		Default: 8080,
		Target:  &port,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port != 8080 {
		t.Errorf("expected 8080, got %d", port)
	}
}

func TestParse_Default_Float(t *testing.T) {
	p := NewParser("test")
	var threshold float64
	err := p.AddOption(&Option{
		Long:    "threshold",
		Type:    FloatType,
		Default: 0.5,
		Target:  &threshold,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if threshold != 0.5 {
		t.Errorf("expected 0.5, got %f", threshold)
	}
}

func TestParse_Default_Bool(t *testing.T) {
	p := NewParser("test")
	var debug bool
	err := p.AddOption(&Option{
		Long:    "debug",
		Type:    BoolType,
		Default: true,
		Target:  &debug,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !debug {
		t.Error("expected debug true by default")
	}
}

func TestParse_FloatOption_Equals(t *testing.T) {
	p := NewParser("test")
	var val float64
	err := p.AddOption(&Option{Long: "val", Type: FloatType, Target: &val})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"--val=2.718"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 2.718 {
		t.Errorf("expected 2.718, got %f", val)
	}
}

func TestParse_IntOption_Equals(t *testing.T) {
	p := NewParser("test")
	var val int
	err := p.AddOption(&Option{Long: "val", Type: IntType, Target: &val})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"--val=99"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 99 {
		t.Errorf("expected 99, got %d", val)
	}
}

func TestParse_SubCommand_UnknownOption(t *testing.T) {
	p := NewParser("test")
	cmd := NewCommand("test")
	err := p.AddCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"test", "--nonexistent"})
	if !errors.Is(err, ErrUnknownOption) {
		t.Errorf("expected ErrUnknownOption, got %v", err)
	}
}

func TestParse_PositionalArgs_Float(t *testing.T) {
	p := NewParser("test")
	var val float64
	err := p.AddPositionalArg(&PositionalArg{Name: "val", Type: FloatType, Target: &val})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"1.5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 1.5 {
		t.Errorf("expected 1.5, got %f", val)
	}
}

func TestParse_PositionalArgs_Float_Invalid(t *testing.T) {
	p := NewParser("test")
	var val float64
	err := p.AddPositionalArg(&PositionalArg{Name: "val", Type: FloatType, Target: &val})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"bad"})
	if !errors.Is(err, ErrInvalidType) {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}
}

func TestParse_PositionalArgs_Bool(t *testing.T) {
	p := NewParser("test")
	var flag bool
	err := p.AddPositionalArg(&PositionalArg{Name: "flag", Type: BoolType, Target: &flag})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"true"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !flag {
		t.Error("expected true")
	}
}

func TestParse_PositionalArgs_Bool_Invalid(t *testing.T) {
	p := NewParser("test")
	var flag bool
	err := p.AddPositionalArg(&PositionalArg{Name: "flag", Type: BoolType, Target: &flag})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"notbool"})
	if !errors.Is(err, ErrInvalidType) {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}
}

func TestParse_NoCommands_NoArgs(t *testing.T) {
	p := NewParser("test")
	err := p.Parse([]string{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestParse_SubCommand_OptionsBeforeCommand(t *testing.T) {
	p := NewParser("test")
	var verbose bool
	err := p.AddOption(&Option{Long: "verbose", Short: "v", Type: BoolType, Target: &verbose})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cmd := NewCommand("test")
	var name string
	err = cmd.AddOption(&Option{Long: "name", Short: "n", Type: StringType, Target: &name})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"--verbose", "test", "--name=hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !verbose {
		t.Error("expected verbose true")
	}
	if name != "hello" {
		t.Errorf("expected hello, got %s", name)
	}
}

func TestParse_SubCommand_EqualsInLongOption(t *testing.T) {
	p := NewParser("test")
	cmd := NewCommand("add")
	var item string
	err := cmd.AddOption(&Option{Long: "item", Type: StringType, Target: &item})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"add", "--item=banana"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item != "banana" {
		t.Errorf("expected banana, got %s", item)
	}
}

func TestErrorStrings(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{"ErrUnknownOption", ErrUnknownOption, "cliparser: unknown option"},
		{"ErrMissingValue", ErrMissingValue, "cliparser: missing value for option"},
		{"ErrInvalidType", ErrInvalidType, "cliparser: invalid value type"},
		{"ErrUnknownCommand", ErrUnknownCommand, "cliparser: unknown command"},
		{"ErrNoCommand", ErrNoCommand, "cliparser: no command specified"},
		{"ErrTooManyArgs", ErrTooManyArgs, "cliparser: too many positional arguments"},
		{"ErrTooFewArgs", ErrTooFewArgs, "cliparser: too few positional arguments"},
		{"ErrShortOptionFormat", ErrShortOptionFormat, "cliparser: invalid short option format"},
		{"ErrDuplicateOption", ErrDuplicateOption, "cliparser: duplicate option definition"},
		{"ErrNilTarget", ErrNilTarget, "cliparser: nil target pointer"},
		{"ErrCommandNotFound", ErrCommandNotFound, "cliparser: command not found"},
		{"ErrNilOption", ErrNilOption, "cliparser: nil option"},
		{"ErrNilCommand", ErrNilCommand, "cliparser: nil command"},
		{"ErrNilArg", ErrNilArg, "cliparser: nil positional argument"},
		{"ErrNoHandler", ErrNoHandler, "cliparser: no handler set for command"},
		{"ErrOptionNoName", ErrOptionNoName, "cliparser: option has neither long nor short name"},
		{"ErrDuplicateCommand", ErrDuplicateCommand, "cliparser: duplicate command name"},
		{"ErrCommandNoName", ErrCommandNoName, "cliparser: command name is empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.msg {
				t.Errorf("expected %q, got %q", tt.msg, tt.err.Error())
			}
		})
	}
}

func TestCommand_AddOption_NoName(t *testing.T) {
	c := NewCommand("test")
	var s string
	err := c.AddOption(&Option{
		Type:   StringType,
		Target: &s,
	})
	if !errors.Is(err, ErrOptionNoName) {
		t.Errorf("expected ErrOptionNoName, got %v", err)
	}
}

func TestParse_BoolOption_EmptyEquals(t *testing.T) {
	p := NewParser("test")
	var flag bool
	err := p.AddOption(&Option{
		Long:   "flag",
		Type:   BoolType,
		Target: &flag,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"--flag="})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !flag {
		t.Error("expected flag true when --flag= (empty value)")
	}
}

func TestParse_BoolOption_TrueEquals(t *testing.T) {
	p := NewParser("test")
	var flag bool
	err := p.AddOption(&Option{
		Long:   "flag",
		Type:   BoolType,
		Target: &flag,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"--flag=true"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !flag {
		t.Error("expected flag true when --flag=true")
	}
}

func TestParse_BoolOption_FalseEquals(t *testing.T) {
	p := NewParser("test")
	var flag bool
	err := p.AddOption(&Option{
		Long:   "flag",
		Type:   BoolType,
		Target: &flag,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	flag = true
	err = p.Parse([]string{"--flag=false"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if flag {
		t.Error("expected flag false when --flag=false")
	}
}

func TestParse_SubCommand_ShortCombinedInCmd(t *testing.T) {
	p := NewParser("test")
	cmd := NewCommand("test")
	var v, d bool
	var n string
	err := cmd.AddOption(&Option{Short: "v", Type: BoolType, Target: &v})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = cmd.AddOption(&Option{Short: "d", Type: BoolType, Target: &d})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = cmd.AddOption(&Option{Short: "n", Type: StringType, Target: &n})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.AddCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = p.Parse([]string{"test", "-vdn", "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v || !d {
		t.Errorf("expected v and d true: v=%v d=%v", v, d)
	}
	if n != "value" {
		t.Errorf("expected value, got %s", n)
	}
}
