package cliparser

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrUnknownOption     = errors.New("cliparser: unknown option")
	ErrMissingValue      = errors.New("cliparser: missing value for option")
	ErrInvalidType       = errors.New("cliparser: invalid value type")
	ErrUnknownCommand    = errors.New("cliparser: unknown command")
	ErrNoCommand         = errors.New("cliparser: no command specified")
	ErrTooManyArgs       = errors.New("cliparser: too many positional arguments")
	ErrTooFewArgs        = errors.New("cliparser: too few positional arguments")
	ErrShortOptionFormat = errors.New("cliparser: invalid short option format")
	ErrDuplicateOption   = errors.New("cliparser: duplicate option definition")
	ErrNilTarget         = errors.New("cliparser: nil target pointer")
	ErrCommandNotFound   = errors.New("cliparser: command not found")
	ErrNilOption         = errors.New("cliparser: nil option")
	ErrNilCommand        = errors.New("cliparser: nil command")
	ErrNilArg            = errors.New("cliparser: nil positional argument")
	ErrNoHandler         = errors.New("cliparser: no handler set for command")
	ErrOptionNoName      = errors.New("cliparser: option has neither long nor short name")
	ErrDuplicateCommand  = errors.New("cliparser: duplicate command name")
	ErrCommandNoName     = errors.New("cliparser: command name is empty")
)

type OptionType int

const (
	StringType OptionType = iota
	IntType
	BoolType
	FloatType
)

type Option struct {
	Long        string
	Short       string
	Description string
	Type        OptionType
	Default     interface{}
	Target      interface{}
}

type PositionalArg struct {
	Name        string
	Description string
	Type        OptionType
	Target      interface{}
}

type HandlerFunc func() error

type Command struct {
	Name        string
	Description string
	Options     []*Option
	Args        []*PositionalArg
	Handler     HandlerFunc
	optionsMap  map[string]*Option
}

type Parser struct {
	AppName     string
	Description string
	Options     []*Option
	Commands    []*Command
	commandsMap map[string]*Command
	optionsMap  map[string]*Option
	args        []*PositionalArg
	parsedCmd   *Command
}

func NewParser(appName string) *Parser {
	return &Parser{
		AppName:     appName,
		optionsMap:  make(map[string]*Option),
		commandsMap: make(map[string]*Command),
	}
}

func (p *Parser) AddOption(opt *Option) error {
	if opt == nil {
		return ErrNilOption
	}
	if opt.Target == nil {
		return ErrNilTarget
	}
	if opt.Long == "" && opt.Short == "" {
		return ErrOptionNoName
	}
	if opt.Long != "" {
		if _, exists := p.optionsMap["--"+opt.Long]; exists {
			return fmt.Errorf("%w: --%s", ErrDuplicateOption, opt.Long)
		}
		p.optionsMap["--"+opt.Long] = opt
	}
	if opt.Short != "" {
		if len(opt.Short) != 1 {
			return fmt.Errorf("%w: -%s", ErrShortOptionFormat, opt.Short)
		}
		if _, exists := p.optionsMap["-"+opt.Short]; exists {
			return fmt.Errorf("%w: -%s", ErrDuplicateOption, opt.Short)
		}
		p.optionsMap["-"+opt.Short] = opt
	}
	p.Options = append(p.Options, opt)
	return nil
}

func (p *Parser) AddCommand(cmd *Command) error {
	if cmd == nil {
		return ErrNilCommand
	}
	if cmd.Name == "" {
		return ErrCommandNoName
	}
	if _, exists := p.commandsMap[cmd.Name]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateCommand, cmd.Name)
	}
	cmd.optionsMap = make(map[string]*Option)
	for _, opt := range cmd.Options {
		if opt.Long != "" {
			cmd.optionsMap["--"+opt.Long] = opt
		}
		if opt.Short != "" {
			cmd.optionsMap["-"+opt.Short] = opt
		}
	}
	p.commandsMap[cmd.Name] = cmd
	p.Commands = append(p.Commands, cmd)
	return nil
}

func (p *Parser) AddPositionalArg(arg *PositionalArg) error {
	if arg == nil {
		return ErrNilArg
	}
	if arg.Target == nil {
		return ErrNilTarget
	}
	p.args = append(p.args, arg)
	return nil
}

func (p *Parser) applyDefaultOptions(optionsMap map[string]*Option, appliedOpts map[*Option]bool) {
	for _, opt := range optionsMap {
		if !appliedOpts[opt] && opt.Default != nil {
			setOptionValue(opt, opt.Default)
		}
	}
}

func (p *Parser) applyDefaultAllOptions() {
	p.applyDefaultOptions(p.optionsMap, make(map[*Option]bool))
}

func (p *Parser) Parse(args []string) error {
	p.applyDefaultAllOptions()

	appliedRootOpts := make(map[*Option]bool)
	appliedCmdOpts := make(map[*Option]bool)
	parsedArgs := []string{}
	i := 0

	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			parsedArgs = append(parsedArgs, args[i+1:]...)
			break
		}

		if strings.HasPrefix(arg, "--") {
			opt, rest, err := p.parseLongOption(arg, args, &i)
			if err != nil {
				return err
			}
			if opt != nil {
				if p.parsedCmd != nil {
					appliedCmdOpts[opt] = true
				} else {
					appliedRootOpts[opt] = true
				}
				if rest != "" {
					if err := setOptionValue(opt, rest); err != nil {
						return err
					}
				}
			}
			i++
			continue
		}

		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			opts, lastValue, hasValue, err := p.parseShortOptions(arg, args, &i)
			if err != nil {
				return err
			}
			for j, opt := range opts {
				if p.parsedCmd != nil {
					appliedCmdOpts[opt] = true
				} else {
					appliedRootOpts[opt] = true
				}
				if j == len(opts)-1 && hasValue {
					if err := setOptionValue(opt, lastValue); err != nil {
						return err
					}
				} else if opt.Type == BoolType {
					setOptionValue(opt, true)
				}
			}
			i++
			continue
		}

		if p.parsedCmd == nil && len(p.Commands) > 0 {
			cmd, ok := p.commandsMap[arg]
			if !ok {
				return fmt.Errorf("%w: %s", ErrUnknownCommand, arg)
			}
			p.parsedCmd = cmd
			i++
			continue
		}

		parsedArgs = append(parsedArgs, arg)
		i++
	}

	if len(p.Commands) > 0 && p.parsedCmd == nil {
		return ErrNoCommand
	}

	rootOpts := make(map[string]*Option)
	for k, v := range p.optionsMap {
		rootOpts[k] = v
	}
	p.applyDefaultOptions(rootOpts, appliedRootOpts)

	if p.parsedCmd != nil {
		p.applyDefaultOptions(p.parsedCmd.optionsMap, appliedCmdOpts)
		argDefs := p.parsedCmd.Args
		if err := bindPositionalArgs(parsedArgs, argDefs); err != nil {
			return err
		}
	} else {
		if err := bindPositionalArgs(parsedArgs, p.args); err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) parseLongOption(arg string, args []string, i *int) (*Option, string, error) {
	eqIdx := strings.Index(arg, "=")
	var name, value string
	var hasValue bool

	if eqIdx > 0 {
		name = arg[:eqIdx]
		value = arg[eqIdx+1:]
		hasValue = true
	} else {
		name = arg
	}

	var opt *Option
	if p.parsedCmd != nil {
		opt = p.parsedCmd.optionsMap[name]
	}
	if opt == nil {
		opt = p.optionsMap[name]
	}
	if opt == nil {
		return nil, "", fmt.Errorf("%w: %s", ErrUnknownOption, name)
	}

	if opt.Type == BoolType {
		if hasValue {
			if value == "" {
				setOptionValue(opt, true)
			} else {
				b, err := strconv.ParseBool(value)
				if err != nil {
					return nil, "", fmt.Errorf("%w: %s=%s", ErrInvalidType, name, value)
				}
				setOptionValue(opt, b)
			}
		} else {
			setOptionValue(opt, true)
		}
		return opt, "", nil
	}

	if !hasValue {
		if *i+1 >= len(args) {
			return nil, "", fmt.Errorf("%w: %s", ErrMissingValue, name)
		}
		*i++
		value = args[*i]
	}
	if err := setOptionValue(opt, value); err != nil {
		return nil, "", err
	}
	return opt, "", nil
}

func (p *Parser) parseShortOptions(arg string, args []string, i *int) ([]*Option, string, bool, error) {
	opts := []*Option{}
	var lastOpt *Option
	hasValue := false
	lastValue := ""

	pos := 1
	for pos < len(arg) {
		ch := string(arg[pos])
		optKey := "-" + ch

		var opt *Option
		if p.parsedCmd != nil {
			opt = p.parsedCmd.optionsMap[optKey]
		}
		if opt == nil {
			opt = p.optionsMap[optKey]
		}
		if opt == nil {
			return nil, "", false, fmt.Errorf("%w: %s", ErrUnknownOption, optKey)
		}

		opts = append(opts, opt)
		lastOpt = opt
		pos++

		if opt.Type != BoolType {
			if pos < len(arg) {
				lastValue = arg[pos:]
				hasValue = true
			} else {
				if *i+1 >= len(args) {
					return nil, "", false, fmt.Errorf("%w: %s", ErrMissingValue, optKey)
				}
				*i++
				lastValue = args[*i]
				hasValue = true
			}
			break
		}
	}

	if lastOpt != nil && lastOpt.Type == BoolType {
		for _, o := range opts {
			setOptionValue(o, true)
		}
	}

	return opts, lastValue, hasValue, nil
}

func setOptionValue(opt *Option, rawValue interface{}) error {
	var val interface{}
	switch v := rawValue.(type) {
	case string:
		switch opt.Type {
		case StringType:
			val = v
		case IntType:
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("%w: cannot parse %q as int", ErrInvalidType, v)
			}
			val = n
		case FloatType:
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return fmt.Errorf("%w: cannot parse %q as float", ErrInvalidType, v)
			}
			val = f
		case BoolType:
			b, err := strconv.ParseBool(v)
			if err != nil {
				return fmt.Errorf("%w: cannot parse %q as bool", ErrInvalidType, v)
			}
			val = b
		}
	default:
		val = v
	}

	switch t := opt.Target.(type) {
	case *string:
		s, ok := val.(string)
		if !ok {
			return fmt.Errorf("%w: expected string", ErrInvalidType)
		}
		*t = s
	case *int:
		n, ok := val.(int)
		if !ok {
			return fmt.Errorf("%w: expected int", ErrInvalidType)
		}
		*t = n
	case *bool:
		b, ok := val.(bool)
		if !ok {
			return fmt.Errorf("%w: expected bool", ErrInvalidType)
		}
		*t = b
	case *float64:
		f, ok := val.(float64)
		if !ok {
			return fmt.Errorf("%w: expected float64", ErrInvalidType)
		}
		*t = f
	default:
		return fmt.Errorf("%w: unsupported target type", ErrInvalidType)
	}
	return nil
}

func bindPositionalArgs(parsedArgs []string, argDefs []*PositionalArg) error {
	if len(parsedArgs) > len(argDefs) {
		return fmt.Errorf("%w: expected %d, got %d", ErrTooManyArgs, len(argDefs), len(parsedArgs))
	}
	if len(parsedArgs) < len(argDefs) {
		return fmt.Errorf("%w: expected %d, got %d", ErrTooFewArgs, len(argDefs), len(parsedArgs))
	}

	for i, def := range argDefs {
		rawVal := parsedArgs[i]
		var val interface{}
		switch def.Type {
		case StringType:
			val = rawVal
		case IntType:
			n, err := strconv.Atoi(rawVal)
			if err != nil {
				return fmt.Errorf("%w: argument %s: cannot parse %q as int", ErrInvalidType, def.Name, rawVal)
			}
			val = n
		case FloatType:
			f, err := strconv.ParseFloat(rawVal, 64)
			if err != nil {
				return fmt.Errorf("%w: argument %s: cannot parse %q as float", ErrInvalidType, def.Name, rawVal)
			}
			val = f
		case BoolType:
			b, err := strconv.ParseBool(rawVal)
			if err != nil {
				return fmt.Errorf("%w: argument %s: cannot parse %q as bool", ErrInvalidType, def.Name, rawVal)
			}
			val = b
		}
		opt := &Option{Type: def.Type, Target: def.Target}
		if err := setOptionValue(opt, val); err != nil {
			return err
		}
	}
	return nil
}

func (p *Parser) Execute() error {
	if p.parsedCmd == nil {
		return ErrCommandNotFound
	}
	if p.parsedCmd.Handler == nil {
		return ErrNoHandler
	}
	return p.parsedCmd.Handler()
}

func (p *Parser) GetCommand() *Command {
	return p.parsedCmd
}

func NewCommand(name string) *Command {
	return &Command{
		Name:       name,
		optionsMap: make(map[string]*Option),
	}
}

func (c *Command) AddOption(opt *Option) error {
	if opt == nil {
		return ErrNilOption
	}
	if opt.Target == nil {
		return ErrNilTarget
	}
	if opt.Long == "" && opt.Short == "" {
		return ErrOptionNoName
	}
	if opt.Long != "" {
		if _, exists := c.optionsMap["--"+opt.Long]; exists {
			return fmt.Errorf("%w: --%s", ErrDuplicateOption, opt.Long)
		}
		c.optionsMap["--"+opt.Long] = opt
	}
	if opt.Short != "" {
		if len(opt.Short) != 1 {
			return fmt.Errorf("%w: -%s", ErrShortOptionFormat, opt.Short)
		}
		if _, exists := c.optionsMap["-"+opt.Short]; exists {
			return fmt.Errorf("%w: -%s", ErrDuplicateOption, opt.Short)
		}
		c.optionsMap["-"+opt.Short] = opt
	}
	c.Options = append(c.Options, opt)
	return nil
}

func (c *Command) AddPositionalArg(arg *PositionalArg) error {
	if arg == nil {
		return ErrNilArg
	}
	if arg.Target == nil {
		return ErrNilTarget
	}
	c.Args = append(c.Args, arg)
	return nil
}
