package schema

import "fmt"

// ValidateName reports whether a name may be used for a schema element.
//
// A name must be non-empty, start with a letter or an underscore, and contain
// only letters, digits and underscores.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("expected name to be a non-empty string")
	}
	// The whole name is checked before its first character, so that a name
	// with a bad character anywhere is reported as such rather than being
	// blamed on its start.
	for i := 1; i < len(name); i++ {
		if !isNameContinue(name[i]) {
			return fmt.Errorf("names must only contain [_a-zA-Z0-9] but %q does not", name)
		}
	}
	if !isNameStart(name[0]) {
		return fmt.Errorf("names must start with [_a-zA-Z] but %q does not", name)
	}
	return nil
}

// ValidateEnumValueName reports whether a name may be used for an enum member.
//
// It has to be a valid name that is not true, false or null, because those
// spell other kinds of value in a document and an enum member named after one
// could never be written.
func ValidateEnumValueName(name string) error {
	switch name {
	case "true", "false", "null":
		return fmt.Errorf("enum values cannot be named: %s", name)
	}
	return ValidateName(name)
}

// The character classes are duplicated from the language package rather than
// imported, because they are three lines each and importing them would tie the
// schema package to the parser for no other reason.

// isNameStart reports whether c may begin a name.
func isNameStart(c byte) bool {
	return isLetter(c) || c == '_'
}

// isNameContinue reports whether c may appear after the first character.
func isNameContinue(c byte) bool {
	return isLetter(c) || (c >= '0' && c <= '9') || c == '_'
}

// isLetter reports whether c is an ASCII letter.
func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
