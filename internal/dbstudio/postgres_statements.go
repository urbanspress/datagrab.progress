package dbstudio

import (
	"regexp"
	"strings"

	"groxy-desk/internal/security"
)

var postgresDollarTag = regexp.MustCompile(`^\$(?:[A-Za-z_][A-Za-z_0-9]*)?\$`)

// Keep the reviewed bytes, including routine bodies and quoted SQL data.
// PostgreSQL additionally parses each statement via the extended protocol.
func postgresWriteStatements(raw string) ([]string, error) {
	var statements []string
	start := 0
	appendStatement := func(end int) error {
		s := strings.TrimSpace(raw[start:end])
		if s == "" {
			return nil
		}
		words := strings.Fields(strings.ToUpper(s))
		switch words[0] {
		case "UPDATE", "INSERT", "DELETE", "WITH":
		case "ALTER", "CREATE", "DROP":
			if len(words) > 1 && postgresLocalObject(words[1:]) {
				break
			}
			fallthrough
		default:
			return security.NewError(security.CodeDeskE406, 403, "choose data or schema SQL confined to the selected PostgreSQL database", nil)
		}
		statements = append(statements, s)
		return nil
	}
	for i := 0; i < len(raw); {
		switch raw[i] {
		case ';':
			if err := appendStatement(i); err != nil {
				return nil, err
			}
			i++
			start = i
		case '\'', '"':
			quote := raw[i]
			escape := quote == '\'' && i > 0 && (raw[i-1] == 'E' || raw[i-1] == 'e')
			closed := false
			for i++; i < len(raw); i++ {
				if escape && raw[i] == '\\' {
					i++
				} else if raw[i] == quote {
					if i+1 < len(raw) && raw[i+1] == quote {
						i++
						continue
					}
					i++
					closed = true
					break
				}
			}
			if !closed {
				return nil, invalidCell("unterminated PostgreSQL quoted value")
			}
		case '$':
			tag := postgresDollarTag.FindString(raw[i:])
			if tag == "" {
				i++
				continue
			}
			end := strings.Index(raw[i+len(tag):], tag)
			if end < 0 {
				return nil, invalidCell("unterminated PostgreSQL dollar-quoted body")
			}
			i += 2*len(tag) + end
		default:
			if strings.HasPrefix(raw[i:], "--") {
				end := strings.IndexByte(raw[i:], '\n')
				if end < 0 {
					end = len(raw) - i
				}
				if strings.TrimSpace(raw[start:i]) == "" {
					start = i + end
				}
				i += end
			} else if strings.HasPrefix(raw[i:], "/*") {
				depth, first := 1, i
				for i += 2; i < len(raw) && depth != 0; {
					switch {
					case strings.HasPrefix(raw[i:], "/*"):
						depth++
						i += 2
					case strings.HasPrefix(raw[i:], "*/"):
						depth--
						i += 2
					default:
						i++
					}
				}
				if depth != 0 {
					return nil, invalidCell("unterminated PostgreSQL comment")
				}
				if strings.TrimSpace(raw[start:first]) == "" {
					start = i
				}
			} else {
				i++
			}
		}
	}
	if err := appendStatement(len(raw)); err != nil {
		return nil, err
	}
	if len(statements) == 0 {
		return nil, invalidCell("supply a PostgreSQL write statement")
	}
	return statements, nil
}

func postgresLocalObject(words []string) bool {
	switch words[0] {
	case "TABLE", "VIEW", "SEQUENCE", "INDEX", "TRIGGER", "FUNCTION", "PROCEDURE", "SCHEMA", "TYPE", "DOMAIN":
		return true
	case "UNIQUE":
		return len(words) > 1 && words[1] == "INDEX"
	case "OR":
		return len(words) > 2 && words[1] == "REPLACE" && (words[2] == "VIEW" || words[2] == "FUNCTION" || words[2] == "PROCEDURE")
	}
	return false
}
