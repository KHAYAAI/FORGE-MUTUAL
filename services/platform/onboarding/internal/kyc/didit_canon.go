package kyc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

// canonicalJSON re-serialises a JSON document the way Didit does before it signs a webhook with X-Signature-V2
// (https://docs.didit.me/integration/webhooks, "Python / FastAPI (recommended: X-Signature-V2)"):
//
//	json.dumps(shorten_floats(body), sort_keys=True, separators=(",", ":"), ensure_ascii=False)
//
// so that the signature survives any middleware that re-encodes the body. In Go terms:
//
//   - object keys are sorted by code point, with no spaces around "," or ":";
//   - strings keep their Unicode characters; only the quote, the backslash and control characters are escaped,
//     as \b \f \n \r \t or \u00XX, exactly as Python does (Go's own encoder escapes <, > and & as well, and
//     U+2028 and U+2029, which would not match);
//   - an integer is written in full, whatever its size;
//   - a number written as a float but with a whole value (1.0, 1e3) is written as an integer ("shorten_floats");
//   - any other float is written as Python's repr writes it: the shortest digits that read back the same, in plain
//     notation, or with an exponent such as 1e-05 below 1e-4.
//
// The differential test in didit_test.go checks this against Didit's own Python code on recorded vectors.
func canonicalJSON(body []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("kyc: webhook body is not JSON: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("kyc: webhook body has data after the JSON document")
	}
	var out bytes.Buffer
	if err := writeCanonical(&out, v); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func writeCanonical(b *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case string:
		writeCanonicalString(b, x)
	case json.Number:
		n, err := canonicalNumber(x)
		if err != nil {
			return err
		}
		b.WriteString(n)
	case []any:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := writeCanonical(b, e); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys) // byte order of UTF-8 is code point order, which is Python's string order
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			writeCanonicalString(b, k)
			b.WriteByte(':')
			if err := writeCanonical(b, x[k]); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		return fmt.Errorf("kyc: unexpected JSON value of type %T", v)
	}
	return nil
}

// writeCanonicalString writes s as Python's json.dumps does with ensure_ascii=False.
func writeCanonicalString(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// canonicalNumber writes a JSON number as Python would after shorten_floats.
func canonicalNumber(n json.Number) (string, error) {
	text := n.String()
	if !strings.ContainsAny(text, ".eE") {
		// An integer literal: Python reads it as an int of any size. big.Int also turns "-0" into "0".
		i, ok := new(big.Int).SetString(text, 10)
		if !ok {
			return "", fmt.Errorf("kyc: bad integer %q", text)
		}
		return i.String(), nil
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return "", fmt.Errorf("kyc: number %q is out of range", text)
	}
	if f == 0 {
		return "0", nil // -0.0 is a whole float, and Python's int(-0.0) is 0
	}
	if f == math.Trunc(f) {
		// A whole-valued float is written as an integer. big.Float keeps every digit of a large one (1e20).
		return new(big.Float).SetFloat64(f).Text('f', 0), nil
	}
	return pythonFloat(f), nil
}

// pythonFloat formats a finite, non-whole float64 as Python's repr does. Non-whole floats are below 2^52, so the
// exponent form is needed only for tiny values: Python switches to it below 1e-4 and writes at least two exponent
// digits ("1e-05", "1.5e-07").
func pythonFloat(f float64) string {
	e := strconv.FormatFloat(f, 'e', -1, 64) // shortest digits, for example "1.5e-05" or "-2.5e-01"
	i := strings.IndexByte(e, 'e')
	exp, _ := strconv.Atoi(e[i+1:])
	if exp < -4 {
		return e
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}
