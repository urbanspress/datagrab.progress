package dbstudio

import (
	"strings"

	"groxy-desk/internal/security"
)

// mariadbWriteStatements splits reviewed MariaDB SQL into single statements
// and accepts only data and schema statements confined to the selected
// database. MariaDB executes /*! ... */ and /*M! ... */ comment bodies, so
// those are rejected outright instead of being treated as comments. Account,
// privilege, session, routine, event, file and cross-database statements are
// outside the recoverable write scope. The server independently enforces one
// statement per command and the writer's database-scoped privileges.
func mariadbWriteStatements(raw string) ([]string, error) {
	tokens, spans, err := mariadbTokens(raw)
	if err != nil {
		return nil, err
	}
	var statements []string
	start, from := 0, 0
	flush := func(end, upto int) error {
		text := strings.TrimSpace(raw[start:end])
		if text != "" && upto > from {
			if err := checkMariaDBStatement(tokens[from:upto]); err != nil {
				return err
			}
			statements = append(statements, text)
		}
		return nil
	}
	for i, token := range tokens {
		if token.kind == mariaSemicolon {
			if err := flush(spans[i], i); err != nil {
				return nil, err
			}
			start, from = spans[i]+1, i+1
		}
	}
	if err := flush(len(raw), len(tokens)); err != nil {
		return nil, err
	}
	if len(statements) == 0 {
		return nil, invalidCell("supply a MariaDB write statement")
	}
	return statements, nil
}

const (
	mariaWord = iota
	mariaQuoted
	mariaString
	mariaSemicolon
	mariaSymbol
)

type mariaToken struct {
	kind int
	text string // upper-case for words, raw for the remaining kinds
}

func mariaOutsideScope() error {
	return security.NewError(security.CodeDeskE406, 403, "choose data or schema SQL confined to the selected MariaDB database", nil)
}

func mariadbTokens(raw string) ([]mariaToken, []int, error) {
	var tokens []mariaToken
	var spans []int
	add := func(kind int, text string, at int) {
		tokens = append(tokens, mariaToken{kind, text})
		spans = append(spans, at)
	}
	for i := 0; i < len(raw); {
		c := raw[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == '\v':
			i++
		case c == ';':
			add(mariaSemicolon, ";", i)
			i++
		case c == '\'' || c == '"' || c == '`':
			at, quote := i, c
			closed := false
			for i++; i < len(raw); i++ {
				if quote != '`' && raw[i] == '\\' {
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
				return nil, nil, invalidCell("unterminated MariaDB quoted value")
			}
			kind := mariaQuoted
			if quote != '`' {
				kind = mariaString
			}
			add(kind, raw[at:i], at)
		case c == '#' || strings.HasPrefix(raw[i:], "-- ") || strings.HasPrefix(raw[i:], "--\t") || strings.HasPrefix(raw[i:], "--\n") || raw[i:] == "--":
			end := strings.IndexByte(raw[i:], '\n')
			if end < 0 {
				end = len(raw) - i
			}
			i += end
		case strings.HasPrefix(raw[i:], "/*"):
			if strings.HasPrefix(raw[i:], "/*!") || strings.HasPrefix(raw[i:], "/*M!") {
				return nil, nil, mariaOutsideScope()
			}
			end := strings.Index(raw[i+2:], "*/")
			if end < 0 {
				return nil, nil, invalidCell("unterminated MariaDB comment")
			}
			i += end + 4
		case c == '_' || c == '$' || c >= 0x80 || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			at := i
			for i < len(raw) && (raw[i] == '_' || raw[i] == '$' || raw[i] >= 0x80 || raw[i] >= '0' && raw[i] <= '9' ||
				raw[i] >= 'a' && raw[i] <= 'z' || raw[i] >= 'A' && raw[i] <= 'Z') {
				i++
			}
			add(mariaWord, strings.ToUpper(raw[at:i]), at)
		default:
			add(mariaSymbol, string(c), i)
			i++
		}
	}
	return tokens, spans, nil
}

var mariaSystemSchemas = map[string]bool{"MYSQL": true, "INFORMATION_SCHEMA": true, "PERFORMANCE_SCHEMA": true, "SYS": true}

func checkMariaDBStatement(tokens []mariaToken) error {
	if len(tokens) == 0 || tokens[0].kind != mariaWord {
		return mariaOutsideScope()
	}
	for i, token := range tokens {
		switch {
		case token.kind == mariaWord && (token.text == "DIRECTORY" || token.text == "OUTFILE" || token.text == "DUMPFILE" || token.text == "LOAD_FILE"):
			return mariaOutsideScope()
		case (token.kind == mariaWord || token.kind == mariaQuoted) && i+1 < len(tokens) && tokens[i+1].text == ".":
			name := strings.ToUpper(strings.Trim(token.text, "`"))
			if mariaSystemSchemas[name] {
				return mariaOutsideScope()
			}
		}
	}
	switch tokens[0].text {
	case "INSERT", "UPDATE", "DELETE", "REPLACE", "WITH", "TRUNCATE":
		return nil
	case "RENAME", "ALTER", "CREATE", "DROP":
	default:
		return mariaOutsideScope()
	}
	object := ""
	for _, token := range tokens[1:] {
		if token.kind != mariaWord {
			continue
		}
		switch token.text {
		case "TABLE", "TABLES", "VIEW", "INDEX", "TRIGGER":
			object = token.text
		case "OR", "REPLACE", "TEMPORARY", "ALGORITHM", "UNDEFINED", "MERGE", "TEMPTABLE", "DEFINER", "SQL", "SECURITY",
			"INVOKER", "UNIQUE", "FULLTEXT", "SPATIAL", "ONLINE", "OFFLINE", "IGNORE", "CURRENT_USER":
			continue
		default:
			return mariaOutsideScope()
		}
		break
	}
	if object == "" {
		return mariaOutsideScope()
	}
	if tokens[0].text == "CREATE" && object == "TRIGGER" {
		// A compound body contains statement separators that Desk does not
		// attempt to parse. Existing compound triggers still replay from a
		// recovery snapshot, where each server-produced definition stays whole.
		for _, token := range tokens {
			if token.kind == mariaWord && (token.text == "BEGIN" || token.text == "DECLARE") {
				return security.NewError(security.CodeDeskE406, 403, "use a single-statement MariaDB trigger body", nil)
			}
		}
	}
	return nil
}
