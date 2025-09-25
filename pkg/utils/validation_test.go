package utils

import (
	"regexp"
	"testing"
)

func TestParseErrorWithRegex(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		patterns map[string]*regexp.Regexp
		wantLine int
		wantCol  int
		wantMsg  string
	}{
		{
			name:   "postgres style error",
			output: "postgres: line 42: syntax error near unexpected token",
			patterns: map[string]*regexp.Regexp{
				"line":    regexp.MustCompile(`line (\d+)`),
				"message": regexp.MustCompile(`line \d+:\s*(.+)`),
			},
			wantLine: 42,
			wantMsg:  "syntax error near unexpected token",
		},
		{
			name:   "pgbouncer style error",
			output: "pgbouncer.ini:15: invalid parameter 'foo'",
			patterns: map[string]*regexp.Regexp{
				"line":    regexp.MustCompile(`:(\d+):`),
				"message": regexp.MustCompile(`:\d+:\s*(.+)`),
			},
			wantLine: 15,
			wantMsg:  "invalid parameter 'foo'",
		},
		{
			name:   "json style error with line and column",
			output: "Error at line 10, column 5: unexpected character",
			patterns: map[string]*regexp.Regexp{
				"combined": regexp.MustCompile(`line (\d+), column (\d+):\s*(.+)`),
			},
			wantLine: 10,
			wantCol:  5,
			wantMsg:  "unexpected character",
		},
		{
			name:   "separate patterns",
			output: "Line: 25\nColumn: 3\nError: missing semicolon",
			patterns: map[string]*regexp.Regexp{
				"line":    regexp.MustCompile(`Line:\s*(\d+)`),
				"column":  regexp.MustCompile(`Column:\s*(\d+)`),
				"message": regexp.MustCompile(`Error:\s*(.+)`),
			},
			wantLine: 25,
			wantCol:  3,
			wantMsg:  "missing semicolon",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseErrorWithRegex(tt.output, tt.patterns)
			if err != nil {
				t.Fatalf("ParseErrorWithRegex() error = %v", err)
			}

			if result.Line != tt.wantLine {
				t.Errorf("ParseErrorWithRegex() Line = %v, want %v", result.Line, tt.wantLine)
			}
			if result.Column != tt.wantCol {
				t.Errorf("ParseErrorWithRegex() Column = %v, want %v", result.Column, tt.wantCol)
			}
			if result.Message != tt.wantMsg {
				t.Errorf("ParseErrorWithRegex() Message = %v, want %v", result.Message, tt.wantMsg)
			}
		})
	}
}

func TestParseErrorWithRegex_EmptyOutput(t *testing.T) {
	patterns := map[string]*regexp.Regexp{
		"combined": regexp.MustCompile(`line (\d+):\s*(.+)`),
	}

	result, err := ParseErrorWithRegex("", patterns)
	if err != nil {
		t.Fatalf("ParseErrorWithRegex() error = %v", err)
	}

	if result.Message != "" {
		t.Errorf("ParseErrorWithRegex() Message = %v, want empty", result.Message)
	}
	if result.Line != 0 {
		t.Errorf("ParseErrorWithRegex() Line = %v, want 0", result.Line)
	}
}

func TestParseErrorWithRegex_NoMatch(t *testing.T) {
	patterns := map[string]*regexp.Regexp{
		"combined": regexp.MustCompile(`line (\d+):\s*(.+)`),
	}

	output := "This is an error without line number"
	result, err := ParseErrorWithRegex(output, patterns)
	if err != nil {
		t.Fatalf("ParseErrorWithRegex() error = %v", err)
	}

	if result.Message != output {
		t.Errorf("ParseErrorWithRegex() Message = %v, want %v", result.Message, output)
	}
	if result.Line != 0 {
		t.Errorf("ParseErrorWithRegex() Line = %v, want 0", result.Line)
	}
}
