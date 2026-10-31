package db

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func mapFS(files map[string]string) fstest.MapFS {
	m := fstest.MapFS{}
	for name, body := range files {
		m["m/"+name] = &fstest.MapFile{Data: []byte(body)}
	}
	return m
}

const schemaUp = "-- sela:kind schema\nCREATE TABLE x (id int);\n"
const schemaDown = "DROP TABLE x;\n"

func TestMigrationFilesEmbeddedSetIsValid(t *testing.T) {
	migs, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	if len(migs) < 2 || migs[0].Version != 1 || migs[0].Name != "foundation" || migs[0].Kind != KindSchema {
		t.Fatalf("unexpected embedded set: %+v", migs)
	}
	for _, m := range migs {
		if RangeOf(m.Version) == "" || !m.HasDown || len(m.Checksum) != 32 {
			t.Errorf("%s: range %q down %t checksum %d", m.Label(), RangeOf(m.Version), m.HasDown, len(m.Checksum))
		}
		if strings.Contains(m.Up, "\r") {
			t.Errorf("%s: CR in up file", m.Label())
		}
	}
}

func TestMigrationFilesRules(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  string // substring of the error; "" means valid
	}{
		{"valid pair", map[string]string{"0001_a.up.sql": schemaUp, "0001_a.down.sql": schemaDown}, ""},
		{"irreversible without down", map[string]string{"0001_a.up.sql": "-- sela:kind schema\n-- sela:irreversible\nSELECT 1;"}, ""},
		{"bad name upper case", map[string]string{"0001_A.up.sql": schemaUp}, "file name must match"},
		{"bad name three digits", map[string]string{"001_a.up.sql": schemaUp}, "file name must match"},
		{"bad suffix", map[string]string{"0001_a.sql": schemaUp}, "file name must match"},
		{"name too long", map[string]string{"0001_" + strings.Repeat("a", 49) + ".up.sql": schemaUp}, "file name must match"},
		{"duplicate version", map[string]string{"0001_a.up.sql": schemaUp, "0001_a.down.sql": schemaDown, "0001_b.up.sql": schemaUp}, "duplicate version"},
		{"missing up", map[string]string{"0001_a.down.sql": schemaDown}, "missing up file"},
		{"missing kind", map[string]string{"0001_a.up.sql": "CREATE TABLE x (id int);", "0001_a.down.sql": schemaDown}, "missing -- sela:kind"},
		{"bad kind", map[string]string{"0001_a.up.sql": "-- sela:kind both\nSELECT 1;", "0001_a.down.sql": schemaDown}, "sela:kind must be"},
		{"duplicate directive", map[string]string{"0001_a.up.sql": "-- sela:kind schema\n-- sela:kind schema\nSELECT 1;", "0001_a.down.sql": schemaDown}, "duplicate sela:kind"},
		{"unknown directive", map[string]string{"0001_a.up.sql": "-- sela:kind schema\n-- sela:fast\nSELECT 1;", "0001_a.down.sql": schemaDown}, "unknown directive"},
		{"directive after SQL", map[string]string{"0001_a.up.sql": "-- sela:kind schema\nSELECT 1;\n-- sela:irreversible\n", "0001_a.down.sql": schemaDown}, "outside the leading comment block"},
		{"irreversible with down", map[string]string{"0001_a.up.sql": "-- sela:kind schema\n-- sela:irreversible\nSELECT 1;", "0001_a.down.sql": schemaDown}, "must not have a down file"},
		{"reversible without down", map[string]string{"0001_a.up.sql": schemaUp}, "needs a down file"},
		{"data without validate", map[string]string{"0001_a.up.sql": "-- sela:kind data\nUPDATE x SET id = 1;", "0001_a.down.sql": schemaDown}, "need a validate file"},
		{"data no-transaction", map[string]string{"0001_a.up.sql": "-- sela:kind data\n-- sela:no-transaction\nUPDATE x SET id = 1;", "0001_a.down.sql": schemaDown, "0001_a.validate.sql": "SELECT 1 WHERE false"}, "cannot use sela:no-transaction"},
		{"requires unknown", map[string]string{"0101_a.up.sql": "-- sela:kind schema\n-- sela:requires 0001\nSELECT 1;", "0101_a.down.sql": schemaDown}, "not embedded"},
		{"requires self", map[string]string{"0101_a.up.sql": "-- sela:kind schema\n-- sela:requires 0101\nSELECT 1;", "0101_a.down.sql": schemaDown}, "requires itself"},
		{"requires malformed", map[string]string{"0101_a.up.sql": "-- sela:kind schema\n-- sela:requires 1\nSELECT 1;", "0101_a.down.sql": schemaDown}, "4-digit"},
		{"directory", map[string]string{"sub/0001_a.up.sql": schemaUp}, "unexpected directory"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(mapFS(c.files), "m")
			if c.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want error containing %q, got %v", c.want, err)
			}
		})
	}
}

func TestMigrationFilesReservedRanges(t *testing.T) {
	allowed := []string{"0001", "0099", "0100", "0199", "0200", "0299", "0300", "0399", "0400", "0499", "0500", "0599", "0600", "0699", "0700", "0799", "0800", "0899", "0900", "0999", "1000", "1099"}
	refused := []string{"0000", "1100", "9999"}
	for _, v := range allowed {
		if _, err := Parse(mapFS(map[string]string{v + "_a.up.sql": schemaUp, v + "_a.down.sql": schemaDown}), "m"); err != nil {
			t.Errorf("%s refused: %v", v, err)
		}
	}
	for _, v := range refused {
		_, err := Parse(mapFS(map[string]string{v + "_a.up.sql": schemaUp, v + "_a.down.sql": schemaDown}), "m")
		if err == nil || !strings.Contains(err.Error(), "outside the reserved ranges") {
			t.Errorf("%s accepted: %v", v, err)
		}
	}
}

func TestMigrationFilesChecksumNormalisesCRLF(t *testing.T) {
	lf, err := Parse(mapFS(map[string]string{"0001_a.up.sql": schemaUp, "0001_a.down.sql": schemaDown}), "m")
	if err != nil {
		t.Fatal(err)
	}
	crlf, err := Parse(mapFS(map[string]string{"0001_a.up.sql": strings.ReplaceAll(schemaUp, "\n", "\r\n"), "0001_a.down.sql": schemaDown}), "m")
	if err != nil {
		t.Fatal(err)
	}
	if string(lf[0].Checksum) != string(crlf[0].Checksum) {
		t.Fatal("CRLF changed the checksum")
	}
	changed, _ := Parse(mapFS(map[string]string{"0001_a.up.sql": schemaUp + "-- edit\n", "0001_a.down.sql": schemaDown}), "m")
	if string(lf[0].Checksum) == string(changed[0].Checksum) {
		t.Fatal("an edit did not change the checksum")
	}
}

func parsed(t *testing.T, files map[string]string) []Migration {
	t.Helper()
	m, err := Parse(mapFS(files), "m")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func appliedFrom(migs []Migration, versions ...int) []Applied {
	var out []Applied
	for i, v := range versions {
		for _, m := range migs {
			if m.Version == v {
				out = append(out, Applied{Version: v, Name: m.Name, Kind: m.Kind, Checksum: m.Checksum, AppliedSeq: int64(i + 1), AppliedAt: time.Unix(int64(i), 0)})
			}
		}
	}
	return out
}

func TestOutOfOrder(t *testing.T) {
	migs := parsed(t, map[string]string{
		"0001_base.up.sql": schemaUp, "0001_base.down.sql": schemaDown,
		"0100_talk.up.sql": "-- sela:kind schema\n-- sela:requires 0001\nSELECT 1;", "0100_talk.down.sql": schemaDown,
		"0150_talk_more.up.sql": "-- sela:kind schema\n-- sela:requires 0210\nSELECT 1;", "0150_talk_more.down.sql": schemaDown,
		"0200_journal.up.sql": schemaUp, "0200_journal.down.sql": schemaDown,
		"0210_journal_more.up.sql": schemaUp, "0210_journal_more.down.sql": schemaDown,
	})
	// 0001 and 0200 applied: 0100 is out of order (lower than 0200), 0150 too.
	st := Plan(migs, appliedFrom(migs, 1, 200))
	states := map[int]State{}
	for _, e := range st.Entries {
		states[e.Version] = e.State
	}
	if states[100] != StateOutOfOrder || states[150] != StateOutOfOrder || states[210] != StatePending || states[1] != StateApplied {
		t.Fatalf("states: %v", states)
	}
	if st.ExitCode() != 3 || st.Current() {
		t.Fatalf("exit %d", st.ExitCode())
	}
	// 0100 requires 0001 (applied): allowed. 0150 requires 0210, which is
	// pending and higher, so the whole run is refused before anything runs.
	if _, err := st.Sequence(0); !errors.Is(err, ErrRequiresUnmet) || !strings.Contains(err.Error(), "0150_talk_more requires 0210") {
		t.Fatalf("want requires error, got %v", err)
	}
	seq, err := st.Sequence(100)
	if err != nil || len(seq) != 1 || seq[0].Version != 100 {
		t.Fatalf("--to 0100: %v %v", seq, err)
	}
	// Once 0210 is applied, 0150 may run out of order.
	st = Plan(migs, appliedFrom(migs, 1, 200, 210))
	seq, err = st.Sequence(0)
	if err != nil || len(seq) != 2 || seq[0].Version != 100 || seq[1].Version != 150 {
		t.Fatalf("after 0210: %v %v", seq, err)
	}
	if _, err := st.Sequence(700); err == nil {
		t.Fatal("--to of an unknown version accepted")
	}
}

func TestOutOfOrderMismatchStates(t *testing.T) {
	migs := parsed(t, map[string]string{"0001_base.up.sql": schemaUp, "0001_base.down.sql": schemaDown})
	applied := appliedFrom(migs, 1)
	applied[0].Checksum = make([]byte, 32)
	st := Plan(migs, applied)
	if st.Entries[0].State != StateChecksumMismatch || st.ExitCode() != 4 || len(st.Problems()) != 1 {
		t.Fatalf("checksum mismatch not reported: %+v", st.Entries)
	}
	applied = append(appliedFrom(migs, 1), Applied{Version: 5, Name: "future", Kind: KindSchema, Checksum: make([]byte, 32), AppliedSeq: 2})
	st = Plan(migs, applied)
	if st.Entries[1].State != StateUnknown || st.ExitCode() != 4 {
		t.Fatalf("unknown version not reported: %+v", st.Entries)
	}
	applied = appliedFrom(migs, 1)
	applied[0].Name = "renamed"
	if st := Plan(migs, applied); st.ExitCode() != 4 {
		t.Fatal("renamed migration not reported")
	}
	if st := Plan(migs, appliedFrom(migs, 1)); st.ExitCode() != 0 || !st.Current() {
		t.Fatal("current schema reported as not current")
	}
}

// R1-6: transaction control in a transactional migration's files, and a
// validate file without a query, are refused at parse time.
func TestMigrationFilesRefuseTransactionControl(t *testing.T) {
	const data = "-- sela:kind data\n"
	up := func(body string) map[string]string {
		return map[string]string{"0001_a.up.sql": data + body, "0001_a.down.sql": "SELECT 1;", "0001_a.validate.sql": "SELECT 1 WHERE false"}
	}
	refused := map[string]map[string]string{
		"COMMIT":               up("UPDATE x SET id = 1;\nCOMMIT;\n"),
		"BEGIN wrapper":        up("BEGIN;\nUPDATE x SET id = 1;\nCOMMIT;\n"),
		"lower-case commit":    up("update x set id = 1; commit"),
		"START TRANSACTION":    up("START TRANSACTION; UPDATE x SET id = 1;"),
		"END":                  up("UPDATE x SET id = 1; END;"),
		"ROLLBACK":             up("ROLLBACK;"),
		"ABORT":                up("ABORT;"),
		"SAVEPOINT":            up("SAVEPOINT s1; UPDATE x SET id = 1;"),
		"RELEASE":              up("RELEASE SAVEPOINT s1;"),
		"PREPARE TRANSACTION":  up("PREPARE TRANSACTION 'gid';"),
		"COMMIT PREPARED":      up("COMMIT PREPARED 'gid';"),
		"ROLLBACK PREPARED":    up("ROLLBACK PREPARED 'gid';"),
		"after a comment":      up("/* rewrite */ -- note\nCOMMIT;"),
		"schema migration":     {"0001_a.up.sql": "-- sela:kind schema\nCREATE TABLE x (id int);\nCOMMIT;\n", "0001_a.down.sql": "DROP TABLE x;"},
		"in the down file":     {"0001_a.up.sql": schemaUp, "0001_a.down.sql": "BEGIN; DROP TABLE x; COMMIT;"},
		"in the validate file": {"0001_a.up.sql": data + "SELECT 1;", "0001_a.down.sql": "SELECT 1;", "0001_a.validate.sql": "COMMIT"},
		"comment-only validate": {"0001_a.up.sql": data + "SELECT 1;", "0001_a.down.sql": "SELECT 1;",
			"0001_a.validate.sql": "-- TODO: write the ownership checks\n"},
		"empty validate":         {"0001_a.up.sql": data + "SELECT 1;", "0001_a.down.sql": "SELECT 1;", "0001_a.validate.sql": " \n\t\n"},
		"two-statement validate": {"0001_a.up.sql": data + "SELECT 1;", "0001_a.down.sql": "SELECT 1;", "0001_a.validate.sql": "SELECT 1 WHERE false; SELECT 2 WHERE false;"},
		"unterminated string":    up("UPDATE x SET name = 'open;\n"),
		"unterminated comment":   up("UPDATE x SET id = 1; /* COMMIT;"),
		"unterminated dollar":    up("DO $$ BEGIN NULL; END;\n"),
		// R2-1: a function named begin is not a BEGIN ATOMIC body, so the
		// statements after it are still seen.
		"function named begin": up("CREATE FUNCTION begin() RETURNS int LANGUAGE sql AS 'SELECT 1';\nCREATE TABLE r2_persisted (id int);\nCOMMIT;\nBEGIN;\nCREATE TABLE r2_after (id int);\n"),
		// CASE and END used as column labels do not move the body's end.
		"case label in a body":        up("CREATE FUNCTION f() RETURNS int LANGUAGE sql BEGIN ATOMIC SELECT 1 AS case; END;\nCOMMIT;\nBEGIN;\n"),
		"bare end label in a body":    up("CREATE FUNCTION f() RETURNS int LANGUAGE sql BEGIN ATOMIC SELECT 1 end; SELECT 2; END;\nCOMMIT; BEGIN;\n"),
		"begin atomic as identifiers": up("SELECT begin atomic FROM (SELECT 1 AS begin) s;\nCOMMIT;\nBEGIN;\nSELECT 1 AS end;\n"),
		"commit inside a body":        up("CREATE PROCEDURE p() LANGUAGE sql BEGIN ATOMIC SELECT 1; COMMIT; END;\n"),
		"unterminated body":           up("CREATE FUNCTION f() RETURNS int LANGUAGE sql BEGIN ATOMIC SELECT 1;\nCOMMIT;\nBEGIN;\n"),
		"unclosed parenthesis":        up("UPDATE x SET id = (1;\nCOMMIT;\nBEGIN;\n"),
	}
	for name, files := range refused {
		t.Run("refused/"+name, func(t *testing.T) {
			if _, err := Parse(mapFS(files), "m"); err == nil {
				t.Fatal("accepted")
			}
		})
	}
	allowed := map[string]map[string]string{
		"plpgsql body": {"0001_a.up.sql": "-- sela:kind schema\nCREATE FUNCTION f() RETURNS trigger LANGUAGE plpgsql AS\n$$ BEGIN RAISE EXCEPTION 'no; COMMIT;'; END $$;\n",
			"0001_a.down.sql": "DROP FUNCTION f();"},
		"tagged dollar body": {"0001_a.up.sql": "-- sela:kind schema\nCREATE FUNCTION f() RETURNS int LANGUAGE plpgsql AS $fn$ BEGIN COMMIT; RETURN 1; END $fn$;\n",
			"0001_a.down.sql": "DROP FUNCTION f();"},
		"SQL-standard body": {"0001_a.up.sql": "-- sela:kind schema\nCREATE OR REPLACE FUNCTION g(i int) RETURNS int LANGUAGE sql\nBEGIN ATOMIC\n  SELECT CASE WHEN i > 0 THEN 1 ELSE 0 END;\n  SELECT 2;\nEND;\nCREATE PROCEDURE p() LANGUAGE sql BEGIN ATOMIC INSERT INTO x VALUES (1); END;\n",
			"0001_a.down.sql": "DROP PROCEDURE p(); DROP FUNCTION g(int);"},
		"words in comments and strings": {"0001_a.up.sql": data + "-- COMMIT;\n/* outer /* COMMIT; */ BEGIN; */\nUPDATE x SET note = 'COMMIT; BEGIN', \"commit\" = E'it\\'s; COMMIT';\n",
			"0001_a.down.sql": "SELECT 1;", "0001_a.validate.sql": "-- must be empty\nSELECT id FROM x WHERE note <> 'COMMIT;'\n"},
		"prepared statement": {"0001_a.up.sql": "-- sela:kind schema\nPREPARE q AS SELECT 1;\nDEALLOCATE q;\n", "0001_a.down.sql": "SELECT 1;"},
		"identifier with $":  {"0001_a.up.sql": "-- sela:kind schema\nCREATE TABLE a$b (id int);\n", "0001_a.down.sql": "DROP TABLE a$b;"},
		"no-transaction":     {"0001_a.up.sql": "-- sela:kind schema\n-- sela:no-transaction\nCOMMIT;\nCREATE INDEX CONCURRENTLY i ON x (id);\n", "0001_a.down.sql": "DROP INDEX CONCURRENTLY i;"},
		"function named begin": {"0001_a.up.sql": "-- sela:kind schema\nCREATE FUNCTION begin() RETURNS int LANGUAGE sql AS 'SELECT 1';\nCREATE TABLE x (id int);\n",
			"0001_a.down.sql": "DROP TABLE x; DROP FUNCTION begin();"},
		"labels and nested body": {"0001_a.up.sql": "-- sela:kind schema\nCREATE FUNCTION f() RETURNS int LANGUAGE sql BEGIN ATOMIC SELECT 1 AS case; SELECT CASE WHEN true THEN 1 END AS end; END;\nCREATE TABLE x (id int);\n",
			"0001_a.down.sql": "DROP TABLE x; DROP FUNCTION f();"},
	}
	for name, files := range allowed {
		t.Run("allowed/"+name, func(t *testing.T) {
			if _, err := Parse(mapFS(files), "m"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSplitStatements(t *testing.T) {
	for sql, want := range map[string]int{
		"":                       0,
		"-- only a comment\n":    0,
		"SELECT 1; ; SELECT 2":   2,
		"SELECT ';'; SELECT 2;":  2,
		"SELECT $$;$$; SELECT 3": 2,
		"SELECT (1; 2)":          1, // a semicolon inside parentheses does not split (as in psql)
		"CREATE FUNCTION f() RETURNS int LANGUAGE sql BEGIN ATOMIC SELECT 1; SELECT CASE WHEN true THEN 1 END; END; SELECT 4": 2,
		// R2-1: what the server sees, not psql's BEGIN/CASE/END counting.
		"CREATE FUNCTION begin() RETURNS int LANGUAGE sql AS 'SELECT 1'; CREATE TABLE t (id int); COMMIT; BEGIN;":                                                    4,
		"CREATE FUNCTION f() RETURNS int LANGUAGE sql BEGIN ATOMIC SELECT 1 AS case; END; COMMIT; BEGIN":                                                             3,
		"CREATE FUNCTION f() RETURNS int LANGUAGE sql BEGIN ATOMIC SELECT 1 end; SELECT x.end FROM t x; END; COMMIT":                                                 2,
		"CREATE OR REPLACE PROCEDURE p() LANGUAGE sql BEGIN ATOMIC; END; SELECT 1":                                                                                   2,
		"CREATE FUNCTION f() RETURNS int LANGUAGE sql BEGIN ATOMIC CREATE FUNCTION g() RETURNS int LANGUAGE sql BEGIN ATOMIC SELECT 1; END; SELECT 2; END; SELECT 3": 2,
		"SELECT begin atomic FROM (SELECT 1 AS begin) s; COMMIT; BEGIN":                                                                                              3,
	} {
		got, err := splitStatements(sql)
		if err != nil || len(got) != want {
			t.Errorf("%q: %d statements (%v), want %d", sql, len(got), err, want)
		}
	}
}

// R2-1: transaction control inside a routine body is reported, and so is
// a file that ends inside a body or a parenthesis.
func TestSplitStatementsNestedAndUnterminated(t *testing.T) {
	stmts, err := splitStatements("CREATE PROCEDURE p() LANGUAGE sql BEGIN ATOMIC INSERT INTO x VALUES (1); COMMIT; END; SELECT 1")
	if err != nil || len(stmts) != 2 || len(stmts[0].nested) != 2 || transactionControl(stmts[0].nested[1]) != "COMMIT" {
		t.Fatalf("nested statements: %+v %v", stmts, err)
	}
	for _, sql := range []string{
		"CREATE FUNCTION f() RETURNS int LANGUAGE sql BEGIN ATOMIC SELECT 1;",
		"CREATE FUNCTION f() RETURNS int LANGUAGE sql BEGIN ATOMIC SELECT 1 AS end; COMMIT; BEGIN",
		"SELECT (1; COMMIT; BEGIN;",
	} {
		if _, err := splitStatements(sql); err == nil {
			t.Errorf("%q: accepted", sql)
		}
	}
}
