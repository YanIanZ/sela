package db

import (
	"errors"
	"strconv"
	"strings"
)

// sqlStatement is one statement of a migration file as the lexer sees it:
// its tokens in order, with comments removed. A bare word (keyword or
// unquoted identifier) is kept upper-cased; every other token (string,
// quoted identifier, number, operator, parenthesis) is "". nested holds
// the statements inside its BEGIN ATOMIC routine bodies, if any.
type sqlStatement struct {
	tokens []string
	nested []sqlStatement
}

// leading returns the first n tokens (fewer when the statement is shorter).
func (s sqlStatement) leading(n int) []string {
	if len(s.tokens) < n {
		return s.tokens
	}
	return s.tokens[:n]
}

// splitStatements splits SQL text into statements the way the PostgreSQL
// server does for a multi-statement query string: it understands -- and
// nested /* */ comments, single-quoted strings (with E-string backslash
// escapes), dollar-quoted bodies, quoted identifiers, parentheses, and
// the body of an SQL-standard routine, whose semicolons do not end the
// statement. A body opens only with the token pair BEGIN ATOMIC at
// parenthesis depth 0 of a statement that starts with CREATE [OR REPLACE]
// FUNCTION|PROCEDURE (a bare BEGIN may be a function or column name), and
// it closes only with an END that starts one of its inner statements
// (that is the grammar: BEGIN ATOMIC { stmt ; } END); an END or CASE
// inside an inner statement is a CASE expression or a column label
// ("SELECT 1 AS end") and never counts. psql's own heuristic counts every
// BEGIN, CASE and END and can be fooled by both. The inner statements are
// returned in nested so that transaction control inside a body is caught
// too. Statements without any token (only whitespace and comments) are
// dropped; a file that ends inside a body or with an unclosed parenthesis
// is refused. It is used to check migration files, never to execute them.
func splitStatements(sql string) ([]sqlStatement, error) {
	var out []sqlStatement
	var cur []string
	var nested []sqlStatement
	paren := 0
	// bodies holds, per open routine body, the index in cur where its
	// current inner statement starts.
	var bodies []int
	stmtStart := func() int {
		if len(bodies) == 0 {
			return 0
		}
		return bodies[len(bodies)-1]
	}
	endInner := func() {
		if start := stmtStart(); start < len(cur) {
			nested = append(nested, sqlStatement{tokens: append([]string(nil), cur[start:]...)})
		}
	}
	flush := func() {
		if len(cur) > 0 {
			out = append(out, sqlStatement{tokens: cur, nested: nested})
		}
		cur, nested, paren, bodies = nil, nil, 0, nil
	}
	isWordStart := func(c byte) bool {
		return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
	}
	isTagPart := func(c byte) bool { return isWordStart(c) || (c >= '0' && c <= '9') }
	isWordPart := func(c byte) bool { return isTagPart(c) || c == '$' }
	n := len(sql)
	for i := 0; i < n; {
		c := sql[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v':
			i++
		case c == '-' && i+1 < n && sql[i+1] == '-':
			for i < n && sql[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && sql[i+1] == '*':
			depth := 0
			for i < n {
				if i+1 < n && sql[i] == '/' && sql[i+1] == '*' {
					depth++
					i += 2
					continue
				}
				if i+1 < n && sql[i] == '*' && sql[i+1] == '/' {
					depth--
					i += 2
					if depth == 0 {
						break
					}
					continue
				}
				i++
			}
			if depth != 0 {
				return nil, errors.New("unterminated /* comment")
			}
		case c == '\'':
			j, err := skipQuoted(sql, i, false)
			if err != nil {
				return nil, err
			}
			cur, i = append(cur, ""), j
		case c == '"':
			j := i + 1
			for {
				k := strings.IndexByte(sql[j:], '"')
				if k < 0 {
					return nil, errors.New("unterminated quoted identifier")
				}
				j += k + 1
				if j < n && sql[j] == '"' {
					j++
					continue
				}
				break
			}
			cur, i = append(cur, ""), j
		case c == '$' && i+1 < n && (sql[i+1] == '$' || isWordStart(sql[i+1])):
			// A dollar quote $tag$ ... $tag$ (a $ followed by digits is a
			// parameter, handled as an operator character below).
			j := i + 1
			for j < n && isTagPart(sql[j]) {
				j++
			}
			if j >= n || sql[j] != '$' {
				cur, i = append(cur, ""), i+1
				continue
			}
			tag := sql[i : j+1]
			k := strings.Index(sql[j+1:], tag)
			if k < 0 {
				return nil, errors.New("unterminated dollar-quoted string")
			}
			cur, i = append(cur, ""), j+1+k+len(tag)
		case isWordStart(c):
			j := i
			for j < n && isWordPart(sql[j]) {
				j++
			}
			word := strings.ToUpper(sql[i:j])
			if (word == "E" || word == "X" || word == "B" || word == "N") && j < n && sql[j] == '\'' {
				k, err := skipQuoted(sql, j, word == "E")
				if err != nil {
					return nil, err
				}
				cur, i = append(cur, ""), k
				continue
			}
			start := stmtStart()
			cur, i = append(cur, word), j
			switch {
			case len(bodies) > 0 && len(cur)-1 == start && word == "END":
				// END as the first word of an inner statement closes the body;
				// the enclosing statement continues.
				bodies = bodies[:len(bodies)-1]
			case word == "ATOMIC" && paren == 0 && len(cur)-2 > start && cur[len(cur)-2] == "BEGIN" &&
				isRoutineStart(cur[start:min(start+4, len(cur))]):
				bodies = append(bodies, len(cur))
			}
		case c == ';':
			switch {
			case paren != 0:
				cur = append(cur, "")
			case len(bodies) == 0:
				flush()
			default:
				endInner()
				cur = append(cur, "")
				bodies[len(bodies)-1] = len(cur)
			}
			i++
		default:
			switch c {
			case '(':
				paren++
			case ')':
				if paren > 0 {
					paren--
				}
			}
			cur, i = append(cur, ""), i+1
		}
	}
	if len(bodies) > 0 {
		return nil, errors.New("the file ends inside a BEGIN ATOMIC routine body (no closing END)")
	}
	if paren != 0 {
		return nil, errors.New("the file ends inside an unclosed parenthesis")
	}
	flush()
	return out, nil
}

// isRoutineStart reports whether the leading words are CREATE [OR
// REPLACE] FUNCTION|PROCEDURE.
func isRoutineStart(tokens []string) bool {
	switch {
	case len(tokens) >= 2 && tokens[0] == "CREATE" && (tokens[1] == "FUNCTION" || tokens[1] == "PROCEDURE"):
		return true
	case len(tokens) >= 4 && tokens[0] == "CREATE" && tokens[1] == "OR" && tokens[2] == "REPLACE" &&
		(tokens[3] == "FUNCTION" || tokens[3] == "PROCEDURE"):
		return true
	}
	return false
}

// skipQuoted returns the index after the single-quoted string starting at
// i (a doubled quote is an escaped quote; with backslash, so is a
// backslash-quote).
func skipQuoted(sql string, i int, backslash bool) (int, error) {
	n := len(sql)
	j := i + 1
	for j < n {
		switch {
		case backslash && sql[j] == '\\':
			j += 2
		case sql[j] == '\'':
			if j+1 < n && sql[j+1] == '\'' {
				j += 2
				continue
			}
			return j + 1, nil
		default:
			j++
		}
	}
	return 0, errors.New("unterminated quoted string")
}

// transactionControl names the transaction-control command a statement
// is (BEGIN, START TRANSACTION, COMMIT, END, ROLLBACK, ABORT, SAVEPOINT,
// RELEASE, PREPARE TRANSACTION; COMMIT/ROLLBACK PREPARED are COMMIT and
// ROLLBACK), or "" when it is none.
func transactionControl(s sqlStatement) string {
	lead := s.leading(2)
	if len(lead) == 0 {
		return ""
	}
	switch lead[0] {
	case "BEGIN", "COMMIT", "END", "ROLLBACK", "ABORT", "SAVEPOINT", "RELEASE":
		if len(lead) == 2 && lead[1] == "PREPARED" {
			return lead[0] + " PREPARED"
		}
		return lead[0]
	case "START", "PREPARE":
		if len(lead) == 2 && lead[1] == "TRANSACTION" {
			return lead[0] + " TRANSACTION"
		}
	}
	return ""
}

// checkTransactional reports the problems of one file of a migration that
// runs inside the runner's transaction: a transaction-control statement
// would end that transaction early, so the validate gate, the dry-run
// rollback and the schema_migrations record could no longer hold.
func checkTransactional(file, sql string) []string {
	stmts, err := splitStatements(sql)
	if err != nil {
		return []string{file + ": " + err.Error()}
	}
	var errs []string
	for i, s := range stmts {
		if tc := transactionControl(s); tc != "" {
			errs = append(errs, file+": statement "+strconv.Itoa(i+1)+" is "+tc+"; transaction control is not allowed in a migration that runs in the runner's transaction")
		}
		for _, in := range s.nested {
			if tc := transactionControl(in); tc != "" {
				errs = append(errs, file+": statement "+strconv.Itoa(i+1)+" holds "+tc+" in a routine body; transaction control is not allowed in a migration that runs in the runner's transaction")
			}
		}
	}
	return errs
}

// checkValidate reports the problems of a validate file: it must be one
// query (the runner prepares it, and a file of only comments would pass
// the gate without checking anything) and must not control the
// transaction.
func checkValidate(file, sql string) []string {
	stmts, err := splitStatements(sql)
	if err != nil {
		return []string{file + ": " + err.Error()}
	}
	switch len(stmts) {
	case 0:
		return []string{file + ": the validate file holds no SQL statement (only comments or whitespace); write the query that must return zero rows"}
	case 1:
	default:
		return []string{file + ": the validate file must be exactly one query, found " + strconv.Itoa(len(stmts)) + " statements"}
	}
	return checkTransactional(file, sql)
}
