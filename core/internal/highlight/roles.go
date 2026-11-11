package highlight

import "github.com/alecthomas/chroma/v2"

// Role is what the UI colors. The ids are wire values: quickshell/Common/SyntaxPalette.js lists the same order.
type Role uint8

const (
	RoleText Role = iota
	RoleKeyword
	RoleType
	RoleConstant
	RoleFunction
	RoleTag
	RoleAttribute
	RolePreproc
	RoleVariable
	RoleString
	RoleEscape
	RoleRegex
	RoleNumber
	RoleOperator
	RolePunctuation
	RoleComment
	RoleHeading
	RoleEmphasis
	RoleStrong
	RoleInserted
	RoleDeleted
	RoleError
	RoleLink
	RoleCode
)

var RoleNames = []string{
	"text", "keyword", "type", "constant", "function", "tag", "attribute", "preproc", "variable", "string",
	"escape", "regex", "number", "operator", "punctuation", "comment", "heading", "emphasis", "strong",
	"inserted", "deleted", "error", "link", "code",
}

func RoleOf(t chroma.TokenType) Role {
	switch t {
	case chroma.KeywordType, chroma.NameClass, chroma.NameNamespace, chroma.NameBuiltin, chroma.NameBuiltinPseudo:
		return RoleType
	case chroma.KeywordConstant, chroma.NameConstant:
		return RoleConstant
	case chroma.NameTag:
		return RoleTag
	case chroma.NameAttribute:
		return RoleAttribute
	case chroma.NameDecorator, chroma.CommentPreproc, chroma.CommentPreprocFile:
		return RolePreproc
	case chroma.LiteralStringEscape:
		return RoleEscape
	case chroma.LiteralStringRegex:
		return RoleRegex
	case chroma.LiteralStringBacktick:
		return RoleCode
	case chroma.GenericHeading, chroma.GenericSubheading:
		return RoleHeading
	case chroma.GenericEmph:
		return RoleEmphasis
	case chroma.GenericStrong:
		return RoleStrong
	case chroma.GenericInserted:
		return RoleInserted
	case chroma.GenericDeleted:
		return RoleDeleted
	case chroma.Error, chroma.GenericError:
		return RoleError
	case chroma.Operator, chroma.OperatorWord:
		return RoleOperator
	case chroma.Punctuation:
		return RolePunctuation
	}
	switch {
	case t.InSubCategory(chroma.NameFunction):
		return RoleFunction
	case t.InSubCategory(chroma.NameVariable):
		return RoleVariable
	case t.InCategory(chroma.Keyword):
		return RoleKeyword
	case t.InSubCategory(chroma.LiteralString):
		return RoleString
	case t.InSubCategory(chroma.LiteralNumber):
		return RoleNumber
	case t.InCategory(chroma.Comment):
		return RoleComment
	}
	return RoleText
}
