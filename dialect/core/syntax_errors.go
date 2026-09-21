package core

import "github.com/antlr4-go/antlr/v4"

// SyntaxErrors records where a parser could not read its input. An inspector
// attaches one in place of the listener it removes, so a statement the parser
// stumbled over is reported as one we could not read rather than as whatever
// fragment error recovery salvaged.
type SyntaxErrors struct {
	*antlr.DefaultErrorListener
	at      []int
	covered [][2]int
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

// In reports whether an error falls in the half-open token span [from, to).
func (s *SyntaxErrors) In(from, to int) bool {
	for _, at := range s.at {
		if at >= from && at < to {
			return true
		}
	}
	return false
}

// Cover records that a reported statement accounts for the span a node covers,
// falling back to [from, to) for a node error recovery left without bounds,
// which covers more and so reports less.
func (s *SyntaxErrors) Cover(node interface {
	GetStart() antlr.Token
	GetStop() antlr.Token
}, from, to int) {
	if start, stop, ok := NodeSpan(node); ok {
		s.covered = append(s.covered, [2]int{start, stop})
		return
	}
	s.covered = append(s.covered, [2]int{from, to})
}

// Uncovered reports whether the parser stumbled over text no reported statement
// covers, which is SQL the caller will run and we never looked at.
func (s *SyntaxErrors) Uncovered() bool {
	for _, at := range s.at {
		covered := false
		for _, span := range s.covered {
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
