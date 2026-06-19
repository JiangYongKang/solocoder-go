package templater

import "errors"

var (
	ErrTemplateNotFound        = errors.New("templater: template not found")
	ErrVariableNotFound        = errors.New("templater: variable not found")
	ErrInvalidVariablePath     = errors.New("templater: invalid variable path")
	ErrInvalidCondition        = errors.New("templater: invalid condition expression")
	ErrInvalidRange            = errors.New("templater: invalid range expression")
	ErrRangeNotIterable        = errors.New("templater: range value is not iterable")
	ErrUnclosedBlock           = errors.New("templater: unclosed block")
	ErrInvalidBlockSyntax      = errors.New("templater: invalid block syntax")
	ErrTemplateInheritanceLoop = errors.New("templater: template inheritance loop detected")
	ErrParentTemplateNotFound  = errors.New("templater: parent template not found")
	ErrFunctionNotFound        = errors.New("templater: function not found")
	ErrInvalidFunctionCall     = errors.New("templater: invalid function call")
	ErrInvalidArgumentCount    = errors.New("templater: invalid argument count")
	ErrEmptyTemplateName       = errors.New("templater: empty template name")
	ErrBlockNotFound           = errors.New("templater: block not found")
)
