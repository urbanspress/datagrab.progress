package dbstudio

import (
	"strings"
	"testing"
)

func TestMariaDBWriteStatements(t *testing.T) {
	accepted := map[string]int{
		"UPDATE `books` SET price = '1.00' WHERE id = 9007199254740993":                             1,
		"ALTER TABLE books ADD COLUMN a INT; ALTER TABLE books ADD COLUMN a INT; -- trailing":       2,
		"INSERT INTO t VALUES ('a;b', \"c;d\", 'it''s', 'back\\'slash'); /* note; */ DELETE FROM t": 2,
		"CREATE OR REPLACE ALGORITHM=UNDEFINED VIEW v AS SELECT id FROM books":                      1,
		"CREATE TRIGGER changed BEFORE UPDATE ON books FOR EACH ROW SET NEW.note = 'x'":             1,
		"CREATE UNIQUE INDEX books_price ON books(price)":                                           1,
		"DROP TABLE IF EXISTS later":  1,
		"TRUNCATE books":              1,
		"RENAME TABLE books TO shelf": 1,
	}
	for statements, want := range accepted {
		got, err := mariadbWriteStatements(statements)
		if err != nil || len(got) != want {
			t.Errorf("%q: %d statements, %v", statements, len(got), err)
		}
	}
	got, err := mariadbWriteStatements("INSERT INTO t VALUES ('a;b'); -- ignored; DROP TABLE t\n")
	if err != nil || len(got) != 1 || !strings.Contains(got[0], "'a;b'") {
		t.Errorf("comment or quoted separator changed the statement list: %q, %v", got, err)
	}
	for _, statement := range []string{
		"GRANT ALL ON *.* TO 'x'@'%'", "CREATE USER 'x'@'%'", "DROP DATABASE d", "CREATE DATABASE d", "SET GLOBAL read_only=0",
		"CALL p()", "USE other", "LOAD DATA INFILE 'x' INTO TABLE t", "SELECT 1", "CREATE PROCEDURE p() SELECT 1",
		"CREATE FUNCTION f() RETURNS INT RETURN 1", "CREATE EVENT e ON SCHEDULE EVERY 1 DAY DO DELETE FROM t",
		"INSERT INTO t SELECT authentication_string FROM mysql.user", "INSERT INTO t SELECT * FROM `INFORMATION_SCHEMA`.`TABLES`",
		"UPDATE t SET a = 1 INTO OUTFILE '/tmp/x'", "CREATE TABLE t (a INT) DATA DIRECTORY='/tmp'",
		"/*!50000 GRANT ALL ON *.* TO 'x'@'%' */ DELETE FROM t", "/*M!100100 DROP TABLE t */ DELETE FROM t",
		"DELETE FROM t; GRANT SELECT ON *.* TO x", "CREATE TRIGGER c BEFORE INSERT ON t FOR EACH ROW BEGIN SET NEW.a = 1; END",
		"CREATE SEQUENCE s", "ALTER USER x IDENTIFIED BY 'y'", "INSERT INTO t VALUES ('open", "DELETE FROM t /* open",
		"DO SLEEP(1)", "HANDLER t OPEN", "FLUSH PRIVILEGES", "KILL 1",
	} {
		if got, err := mariadbWriteStatements(statement); err == nil {
			t.Errorf("%q was accepted as %q", statement, got)
		}
	}
	if _, err := mariadbWriteStatements("  ;; -- nothing\n"); err == nil {
		t.Error("an empty statement list was accepted")
	}
}

func TestMariaSchemaPatternAndGrantee(t *testing.T) {
	for stored, want := range map[string]struct {
		literal string
		pattern bool
	}{"desktarget": {"desktarget", false}, `desk\_us`: {"desk_us", false}, "desk_target": {"", true}, "%": {"", true}, `a\%b`: {"a%b", false}} {
		literal, pattern := mariaSchemaPattern(stored)
		if literal != want.literal || pattern != want.pattern {
			t.Errorf("%q: %q %v", stored, literal, pattern)
		}
	}
	if g, ok := parseMariaGrantee("'target_app_user'@'%'"); !ok || g.account() != "'target_app_user'@'%'" {
		t.Errorf("valid account rejected: %v", g)
	}
	for _, bad := range []string{"'a'b'@'%'", `'a\'@'%'`, "PUBLIC", "'a'@'host name'", "''@'%'", "'a';DROP'@'%'"} {
		if g, ok := parseMariaGrantee(bad); ok {
			t.Errorf("%q parsed as %v", bad, g)
		}
	}
}
