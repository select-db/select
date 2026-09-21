package core

import "github.com/antlr4-go/antlr/v4"

// SyntaxErrors records where a parser could not read its input. An inspector
// attaches one in place of the listener it removes, so a statement the parser
// stumbled over is reported as one we could not read rather than as whatever
// fragment error recovery salvaged.
type SyntaxErrors struct {
	*antlr.DefaultErrorListener
	at []int
}

// NewSyntaxErrors returns a listener ready to attach to a parser.
func NewSyntaxErrors() *SyntaxErrors {
	return &SyntaxErrors{DefaultErrorListener: antlr.NewDefaultErrorListener()}
}

// SyntaxError implements antlr.ErrorListener. An error the offending token
// cannot place is recorded at index 0, which no span excludes.
func (s *SyntaxErrors) SyntaxError(
	_ antlr.Recognizer,
	offending any,
	_, _ int,
	_ string,
	_ antlr.RecognitionException,
) {
	if token, ok := offending.(antlr.Token); ok && token != nil {
		s.at = append(s.at, token.GetTokenIndex())
		return
	}
	s.at = append(s.at, 0)
}

// Any reports whether the parser raised anything at all.
func (s *SyntaxErrors) Any() bool { return len(s.at) > 0 }

// In reports whether an error falls in the half-open token span [from, to).
func (s *SyntaxErrors) In(from, to int) bool {
	for _, at := range s.at {
		if at >= from && at < to {
			return true
		}
	}
	return false
}

// Outside reports whether an error falls in none of the given spans, which
// means the parser stumbled over text no statement we report covers.
func (s *SyntaxErrors) Outside(spans [][2]int) bool {
	for _, at := range s.at {
		covered := false
		for _, span := range spans {
			if at >= span[0] && at < span[1] {
				covered = true
				break
			}
		}
		if !covered {
			return true
		}
	}
	return false
}
