package sourcemap

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SourceMap represents a parsed source map
type SourceMap struct {
	Version    int                    `json:"version"`
	File       string                 `json:"file"`
	SourceRoot string                 `json:"sourceRoot"`
	Sources    []string               `json:"sources"`
	SourcesContent []string           `json:"sourcesContent"`
	Mappings   string                 `json:"mappings"`
	Names      []string               `json:"names"`
}

// OriginalPosition represents an original source position
type OriginalPosition struct {
	Source     string `json:"source"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	Name       string `json:"name,omitempty"`
}

// Parser parses and processes source maps
type Parser struct {
	sm *SourceMap
}

// NewParser creates a new source map parser
func NewParser(content []byte) (*Parser, error) {
	var sm SourceMap
	if err := json.Unmarshal(content, &sm); err != nil {
		return nil, fmt.Errorf("failed to parse source map JSON: %w", err)
	}

	if sm.Version != 3 {
		return nil, fmt.Errorf("unsupported source map version: %d", sm.Version)
	}

	return &Parser{sm: &sm}, nil
}

// FindOriginal finds the original position for a generated line and column
func (p *Parser) FindOriginal(genLine, genCol int) (*OriginalPosition, error) {
	// Source map mappings use 0-based indexing
	line := genLine - 1
	if line < 0 {
		return nil, fmt.Errorf("invalid line number")
	}

	// Split mappings by lines
	mappingsLines := strings.Split(p.sm.Mappings, ";")
	if line >= len(mappingsLines) {
		return nil, fmt.Errorf("line out of range")
	}

	lineMapping := mappingsLines[line]
	if lineMapping == "" {
		return nil, fmt.Errorf("no mapping for line")
	}

	// Decode VLQ and find the segment
	segments := strings.Split(lineMapping, ",")

	var genColOffset, sourceIndex, origLine, origCol int

	for _, segment := range segments {
		if segment == "" {
			continue
		}

		values, err := decodeVLQSegment(segment)
		if err != nil {
			continue
		}

		if len(values) == 0 {
			continue
		}

		// First value is generated column (relative to previous generated column)
		genColOffset += values[0]

		// Check if this segment matches our target column
		if genColOffset > genCol {
			break
		}

		// If we have more values, this segment has source mapping
		if len(values) >= 4 {
			sourceIndex += values[1]
			origLine += values[2]
			origCol += values[3]

			// Check if this is our match
			if genColOffset == genCol || (len(values) > 0 && genColOffset >= genCol) {
				result := &OriginalPosition{
					Line:   origLine + 1, // Convert to 1-based
					Column: origCol + 1,  // Convert to 1-based
				}

				if sourceIndex >= 0 && sourceIndex < len(p.sm.Sources) {
					result.Source = p.sm.Sources[sourceIndex]
				}

				// Extract name if present (5th value)
				if len(values) >= 5 {
					nameIndex := values[4]
					if nameIndex >= 0 && nameIndex < len(p.sm.Names) {
						result.Name = p.sm.Names[nameIndex]
					}
				}

				return result, nil
			}
		} else {
			// Reset source mapping values for next segment
			sourceIndex = 0
			origLine = 0
			origCol = 0
		}
	}

	return nil, fmt.Errorf("no mapping found for position %d:%d", genLine, genCol)
}

// GetSourceContent returns the original source content if available
func (p *Parser) GetSourceContent(sourceIndex int) (string, error) {
	if sourceIndex < 0 || sourceIndex >= len(p.sm.SourcesContent) {
		return "", fmt.Errorf("source index out of range")
	}
	return p.sm.SourcesContent[sourceIndex], nil
}

// decodeVLQSegment decodes a VLQ encoded segment
// VLQ encoding uses base64 character set for each digit
func decodeVLQSegment(segment string) ([]int, error) {
	var values []int
	var value int
	var shift int

	for _, c := range segment {
		// Decode base64 character
		v, ok := vlqDecodeMap[string(c)]
		if !ok {
			return nil, fmt.Errorf("invalid VLQ character: %c", c)
		}

		// Add lower 5 bits to value
		value |= (v & 0x1F) << shift

		// Check continuation bit (bit 5)
		if (v & 0x20) == 0 {
			// Extract sign bit (bit 0 of each VLQ value)
			if value & 1 == 1 {
				value = -(value >> 1)
			} else {
				value = value >> 1
			}
			values = append(values, value)
			value = 0
			shift = 0
		} else {
			shift += 5
		}
	}

	// Handle last value
	if shift > 0 {
		if value & 1 == 1 {
			value = -(value >> 1)
		} else {
			value = value >> 1
		}
		values = append(values, value)
	}

	return values, nil
}

// vlqDecodeMap maps base64 characters to their VLQ values
var vlqDecodeMap = map[string]int{
	"A": 0, "B": 1, "C": 2, "D": 3, "E": 4, "F": 5, "G": 6, "H": 7, "I": 8,
	"J": 9, "K": 10, "L": 11, "M": 12, "N": 13, "O": 14, "P": 15, "Q": 16, "R": 17,
	"S": 18, "T": 19, "U": 20, "V": 21, "W": 22, "X": 23, "Y": 24, "Z": 25,
	"a": 26, "b": 27, "c": 28, "d": 29, "e": 30, "f": 31, "g": 32, "h": 33, "i": 34,
	"j": 35, "k": 36, "l": 37, "m": 38, "n": 39, "o": 40, "p": 41, "q": 42, "r": 43,
	"s": 44, "t": 45, "u": 46, "v": 47, "w": 48, "x": 49, "y": 50, "z": 51,
	"0": 52, "1": 53, "2": 54, "3": 55, "4": 56, "5": 57, "6": 58, "7": 59,
	"8": 60, "9": 61, "+": 62, "/": 63,
}

// DeobfuscateStackFrame deobfuscates a single stack frame
type StackFrame struct {
	Filename    string `json:"filename"`
	Line        int    `json:"line"`
	Column      int    `json:"column"`
	FunctionName string `json:"functionName,omitempty"`
}

// DeobfuscateResult represents the result of deobfuscation
type DeobfuscateResult struct {
	Original  StackFrame       `json:"original"`
	OriginalPosition *OriginalPosition `json:"originalPosition,omitempty"`
	Found     bool             `json:"found"`
}

// DeobfuscateStackTrace deobfuscates a full stack trace
func (p *Parser) DeobfuscateStackTrace(frames []StackFrame) []DeobfuscateResult {
	results := make([]DeobfuscateResult, len(frames))

	for i, frame := range frames {
		// Skip if line is 0 or invalid
		if frame.Line <= 0 {
			results[i] = DeobfuscateResult{
				Original: frame,
				Found:    false,
			}
			continue
		}

		// Use column 1 if not specified
		col := frame.Column
		if col <= 0 {
			col = 1
		}

		origPos, err := p.FindOriginal(frame.Line, col)
		if err != nil {
			results[i] = DeobfuscateResult{
				Original: frame,
				Found:    false,
			}
			continue
		}

		results[i] = DeobfuscateResult{
			Original:         frame,
			OriginalPosition: origPos,
			Found:            true,
		}
	}

	return results
}

// ParseStackFrame parses a stack frame from a string
// Format: "at functionName (https://example.com/app.js:123:45)"
func ParseStackFrame(frameStr string) *StackFrame {
	// Simplified parser for common stack frame formats
	frame := &StackFrame{}

	// Try to extract line and column
	parts := strings.Split(frameStr, ":")
	if len(parts) >= 3 {
		// Last two parts should be line and column
		lineStr := parts[len(parts)-2]
		colStr := strings.Split(parts[len(parts)-1], ")")[0]

		fmt.Sscanf(lineStr, "%d", &frame.Line)
		fmt.Sscanf(colStr, "%d", &frame.Column)

		// Reconstruct filename
		frame.Filename = strings.Join(parts[:len(parts)-2], ":")
	}

	return frame
}
