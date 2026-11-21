package dbstudio

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"groxy-desk/internal/security"

	"github.com/go-sql-driver/mysql"
)

// MariaDB has no database-level switch that closes admission. Desk therefore
// closes it by privilege: one principal holds every privilege on exactly the
// target database (with GRANT OPTION), but holds no global write privilege.
// While a replay runs, that principal revokes every other account's write
// access to the target, ends their live sessions, and restores the recorded
// grants afterwards. Accounts that Desk cannot isolate to the target database
// make the operation unavailable before any approval is issued.

const mariaSessionMode = "NO_AUTO_VALUE_ON_ZERO"

// The mode used for reviewed statements is MariaDB's default.
const mariaUserMode = "STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_ENGINE_SUBSTITUTION"

var (
	// Privileges whose global grant lets an account change data or schema in
	// the target database or change who may.
	mariaGlobalWrite = []string{"INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER", "INDEX", "TRIGGER", "CREATE VIEW",
		"CREATE ROUTINE", "ALTER ROUTINE", "EVENT", "SUPER", "FILE", "SET USER", "DELETE HISTORY", "CREATE USER", "REFERENCES",
		"BINLOG REPLAY", "FEDERATED ADMIN", "READ_ONLY ADMIN"}
	// Privileges that let a principal write inside a database when granted on it.
	mariaScopedWrite = []string{"INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER", "INDEX", "TRIGGER", "CREATE VIEW",
		"CREATE ROUTINE", "ALTER ROUTINE", "EVENT", "DELETE HISTORY", "REFERENCES", "EXECUTE"}
	// The administrator must itself hold these on the target to grant them.
	mariaRequiredScoped = []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER", "INDEX", "TRIGGER",
		"CREATE VIEW", "SHOW VIEW", "REFERENCES", "LOCK TABLES", "CREATE TEMPORARY TABLES"}
	mariaScopedTypes = []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "REFERENCES", "INDEX", "ALTER",
		"CREATE TEMPORARY TABLES", "LOCK TABLES", "EXECUTE", "CREATE VIEW", "SHOW VIEW", "CREATE ROUTINE", "ALTER ROUTINE",
		"EVENT", "TRIGGER", "DELETE HISTORY", "SHOW CREATE ROUTINE"}

	mariaUserName   = regexp.MustCompile(`^[A-Za-z0-9_.$-]{1,80}$`)
	mariaHostName   = regexp.MustCompile(`^[A-Za-z0-9_.:%-]{1,255}$`)
	mariaGranteeRE  = regexp.MustCompile(`^'([^'\\]*)'@'([^'\\]*)'$`)
	errMariaKeepOff = errors.New("keep MariaDB admission closed")
)

type mariaGrantee struct {
	user, host string
	schema     string   // stored schema pattern, exactly as it appears in the grant tables
	types      []string // every schema-level privilege held on the target
	grantable  bool
}

func (g mariaGrantee) account() string { return "'" + g.user + "'@'" + g.host + "'" }

type mariaAdmission struct {
	self     mariaGrantee
	grantees []mariaGrantee
	selfID   uint64
}

func mariaFenceError(message string, err error) error { return writeFenceUnavailable(message, err) }

func quoteMariaList(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = "'" + value + "'"
	}
	return strings.Join(quoted, ",")
}

// mariaSchemaPattern reports whether the stored schema text applies to more
// than one database name, and returns the unescaped literal.
func mariaSchemaPattern(stored string) (string, bool) {
	var literal strings.Builder
	for i := 0; i < len(stored); i++ {
		switch c := stored[i]; {
		case c == '\\' && i+1 < len(stored):
			i++
			literal.WriteByte(stored[i])
		case c == '%' || c == '_':
			return "", true
		default:
			literal.WriteByte(c)
		}
	}
	return literal.String(), false
}

func parseMariaGrantee(raw string) (mariaGrantee, bool) {
	match := mariaGranteeRE.FindStringSubmatch(raw)
	if match == nil || !mariaUserName.MatchString(match[1]) || !mariaHostName.MatchString(match[2]) {
		return mariaGrantee{}, false
	}
	return mariaGrantee{user: match[1], host: match[2]}, true
}

// mariadbAdmission verifies that Desk can isolate the target database and
// returns the accounts whose access Desk must close. It changes nothing.
func mariadbAdmission(ctx context.Context, conn *sql.Conn, scope networkScope) (mariaAdmission, error) {
	var result mariaAdmission
	fail := func(message string, err error) (mariaAdmission, error) { return result, mariaFenceError(message, err) }
	var version, current string
	if err := conn.QueryRowContext(ctx, "SELECT VERSION(), CURRENT_USER()").Scan(&version, &current); err != nil {
		return fail("Desk could not inspect the MariaDB principal.", err)
	}
	if !strings.HasPrefix(version, "11.8.") {
		return fail("Full MariaDB recovery requires MariaDB 11.8.", nil)
	}
	name, host, ok := strings.Cut(current, "@")
	if !ok {
		return fail("Desk could not identify the MariaDB administrator account.", nil)
	}
	self, ok := parseMariaGrantee("'" + name + "'@'" + host + "'")
	if !ok {
		return fail("The MariaDB administrator account name is outside the supported character set.", nil)
	}
	result.self = self
	if err := conn.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&result.selfID); err != nil {
		return fail("Desk could not identify its MariaDB session.", err)
	}

	// The audit must see every account, not only its own.
	var accounts, visible int
	if err := conn.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM mysql.global_priv),
		(SELECT COUNT(DISTINCT GRANTEE) FROM information_schema.USER_PRIVILEGES)`).Scan(&accounts, &visible); err != nil || accounts == 0 || accounts != visible {
		return fail("The MariaDB administrator needs SELECT on the mysql database so Desk can audit every account.", err)
	}
	var admin int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(DISTINCT PRIVILEGE_TYPE) FROM information_schema.USER_PRIVILEGES
		WHERE GRANTEE = ? AND PRIVILEGE_TYPE IN ('CONNECTION ADMIN','PROCESS')`, self.account()).Scan(&admin); err != nil || admin != 2 {
		return fail("The MariaDB administrator needs the global CONNECTION ADMIN and PROCESS privileges to end other sessions on the target.", err)
	}
	global := quoteMariaList(mariaGlobalWrite)
	rows, err := conn.QueryContext(ctx, `SELECT DISTINCT GRANTEE FROM information_schema.USER_PRIVILEGES
		WHERE PRIVILEGE_TYPE IN (`+global+`) OR IS_GRANTABLE = 'YES' ORDER BY GRANTEE`)
	if err != nil {
		return fail("Desk could not audit global MariaDB privileges.", err)
	}
	var global2 []string
	for rows.Next() {
		var grantee string
		if err := rows.Scan(&grantee); err != nil {
			rows.Close()
			return fail("Desk could not audit global MariaDB privileges.", err)
		}
		global2 = append(global2, grantee)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return fail("Desk could not audit global MariaDB privileges.", err)
	}
	if len(global2) > 0 {
		return fail(fmt.Sprintf("Desk cannot exclude all writers to this MariaDB database: %d account(s) hold global write privileges. "+
			"The request requires target-only isolation.", len(global2)), nil)
	}

	// Schema-level grants on the target, including wildcard patterns that match it.
	schemas, err := conn.QueryContext(ctx, `SELECT GRANTEE, TABLE_SCHEMA, PRIVILEGE_TYPE, IS_GRANTABLE
		FROM information_schema.SCHEMA_PRIVILEGES WHERE ? LIKE TABLE_SCHEMA ORDER BY GRANTEE, TABLE_SCHEMA, PRIVILEGE_TYPE`, scope.database)
	if err != nil {
		return fail("Desk could not audit MariaDB database privileges.", err)
	}
	byGrantee := map[string]*mariaGrantee{}
	var order []string
	for schemas.Next() {
		var raw, schema, privilege, grantable string
		if err := schemas.Scan(&raw, &schema, &privilege, &grantable); err != nil {
			schemas.Close()
			return fail("Desk could not audit MariaDB database privileges.", err)
		}
		if !slices.Contains(mariaScopedTypes, privilege) {
			schemas.Close()
			return fail("The MariaDB target grants a privilege Desk cannot restore.", nil)
		}
		literal, pattern := mariaSchemaPattern(schema)
		grantee, ok := parseMariaGrantee(raw)
		write := slices.Contains(mariaScopedWrite, privilege)
		switch {
		case !ok && write:
			schemas.Close()
			return fail("A MariaDB account with write access to the target has an unsupported name.", nil)
		case !ok:
			continue
		case pattern && (write || grantee.account() == self.account()):
			schemas.Close()
			return fail("A MariaDB grant covering the target also covers other databases. Desk requires exact database-scoped grants.", nil)
		case pattern:
			continue
		case literal != scope.database:
			schemas.Close()
			return fail("Desk could not match a MariaDB grant to the target database.", nil)
		}
		key := grantee.account()
		if byGrantee[key] == nil {
			grantee.schema = schema
			byGrantee[key] = &grantee
			order = append(order, key)
		}
		entry := byGrantee[key]
		entry.types = append(entry.types, privilege)
		entry.grantable = entry.grantable || grantable == "YES"
	}
	if err := errors.Join(schemas.Err(), schemas.Close()); err != nil {
		return fail("Desk could not audit MariaDB database privileges.", err)
	}
	selfEntry := byGrantee[self.account()]
	if selfEntry == nil || !selfEntry.grantable {
		return fail("The MariaDB administrator must hold all privileges on exactly the target database with GRANT OPTION.", nil)
	}
	for _, required := range mariaRequiredScoped {
		if !slices.Contains(selfEntry.types, required) {
			return fail("The MariaDB administrator must hold all privileges on exactly the target database with GRANT OPTION.", nil)
		}
	}
	result.self = *selfEntry
	for _, key := range order {
		entry := byGrantee[key]
		if key == self.account() || !slices.ContainsFunc(entry.types, func(t string) bool { return slices.Contains(mariaScopedWrite, t) }) {
			continue
		}
		var users int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM mysql.global_priv WHERE User = ? AND Host = ?
			AND COALESCE(JSON_VALUE(Priv, '$.is_role'), 'false') <> 'true'`, entry.user, entry.host).Scan(&users); err != nil || users != 1 {
			return fail("A MariaDB role or unknown account has write access to the target. Grant access to named accounts instead.", err)
		}
		sort.Strings(entry.types)
		result.grantees = append(result.grantees, *entry)
	}

	// Narrower grants cannot be closed and restored target-only.
	var other int
	err = conn.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM information_schema.TABLE_PRIVILEGES WHERE TABLE_SCHEMA = ? AND GRANTEE <> ?
			AND PRIVILEGE_TYPE IN (`+quoteMariaList(mariaScopedWrite)+`))
		+ (SELECT COUNT(*) FROM information_schema.COLUMN_PRIVILEGES WHERE TABLE_SCHEMA = ? AND GRANTEE <> ?
			AND PRIVILEGE_TYPE IN ('INSERT','UPDATE'))
		+ (SELECT COUNT(*) FROM mysql.procs_priv WHERE Db = ?)
		+ (SELECT COUNT(*) FROM mysql.proxies_priv WHERE Proxied_user = ? AND Proxied_host = ?)
		+ (SELECT COUNT(*) FROM mysql.roles_mapping WHERE User = ? AND Host = ?)`,
		scope.database, self.account(), scope.database, self.account(), scope.database, self.user, self.host, self.user, self.host).Scan(&other)
	if err != nil || other != 0 {
		return fail("A MariaDB table, column, routine, proxy or role grant gives access to the target outside the verified database-wide grants.", err)
	}
	var objects int
	if err := conn.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = ?)
		+ (SELECT COUNT(*) FROM information_schema.EVENTS WHERE EVENT_SCHEMA = ?)`, scope.database, scope.database).Scan(&objects); err != nil || objects != 0 {
		return fail("The MariaDB target contains stored routines or events, which full recovery does not support.", err)
	}
	return result, nil
}

func (g mariaGrantee) revokeStatements(database string) []string {
	target := "`" + g.schema + "`.*"
	statements := []string{"REVOKE " + strings.Join(g.types, ", ") + " ON " + target + " FROM " + g.account()}
	if g.grantable {
		statements = append(statements, "REVOKE GRANT OPTION ON "+target+" FROM "+g.account())
	}
	return statements
}

func (g mariaGrantee) grantStatement() string {
	statement := "GRANT " + strings.Join(g.types, ", ") + " ON `" + g.schema + "`.* TO " + g.account()
	if g.grantable {
		statement += " WITH GRANT OPTION"
	}
	return statement
}

// closeMariaAdmission revokes every other writer's access and ends the live
// sessions that may still hold cached database privileges or open
// transactions. Sessions of unrelated accounts are never touched.
func closeMariaAdmission(ctx context.Context, conn *sql.Conn, scope networkScope, adm mariaAdmission, revoked *[]mariaGrantee) error {
	for _, grantee := range adm.grantees {
		for _, statement := range grantee.revokeStatements(scope.database) {
			if _, err := conn.ExecContext(ctx, statement); err != nil {
				return err
			}
		}
		*revoked = append(*revoked, grantee)
	}
	users := []string{adm.self.user}
	for _, grantee := range adm.grantees {
		users = append(users, grantee.user)
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.Now().Add(10 * time.Second)
	for {
		ids, err := mariaWriterSessions(ctx, conn, users)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			if _, err := conn.ExecContext(ctx, "KILL CONNECTION "+strconv.FormatUint(id, 10)); err != nil {
				// The session may have exited between the list and the kill.
				var unknown *mysql.MySQLError
				if errors.As(err, &unknown) && unknown.Number == 1094 {
					continue
				}
				return err
			}
		}
		if time.Now().After(deadline) {
			return errors.New("MariaDB sessions remained connected to the target")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	return verifyMariaGrants(ctx, conn, scope, adm.grantees, true)
}

// mariaWriterSessions lists every other live session belonging to the
// administrator or an account that held write privileges on the target.
// Terminating all of their sessions prevents cached database privileges,
// open transactions, consistent-read snapshots, or LOCK TABLES metadata locks
// from surviving revocation or blocking DDL replay. Accounts without grants
// on the target are never listed or disturbed.
func mariaWriterSessions(ctx context.Context, conn *sql.Conn, users []string) ([]uint64, error) {
	rows, err := conn.QueryContext(ctx, `SELECT p.ID, p.USER FROM information_schema.PROCESSLIST p WHERE p.ID <> CONNECTION_ID()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uint64
	for rows.Next() {
		var id uint64
		var user string
		if err := rows.Scan(&id, &user); err != nil {
			return nil, err
		}
		if slices.Contains(users, user) {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

// verifyMariaGrants checks that each account's target grants are either all
// absent (closed) or exactly the recorded set (restored).
func verifyMariaGrants(ctx context.Context, conn *sql.Conn, scope networkScope, grantees []mariaGrantee, closed bool) error {
	for _, grantee := range grantees {
		rows, err := conn.QueryContext(ctx, `SELECT PRIVILEGE_TYPE, IS_GRANTABLE FROM information_schema.SCHEMA_PRIVILEGES
			WHERE GRANTEE = ? AND TABLE_SCHEMA = ? ORDER BY PRIVILEGE_TYPE`, grantee.account(), grantee.schema)
		if err != nil {
			return err
		}
		var types []string
		var grantable bool
		for rows.Next() {
			var privilege, flag string
			if err := rows.Scan(&privilege, &flag); err != nil {
				rows.Close()
				return err
			}
			types = append(types, privilege)
			grantable = grantable || flag == "YES"
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return err
		}
		switch {
		case closed && len(types) != 0:
			return fmt.Errorf("MariaDB admission is still open to %s", grantee.account())
		case !closed && (!slices.Equal(types, grantee.types) || grantable != grantee.grantable):
			return fmt.Errorf("MariaDB grants for %s differ from the recorded set", grantee.account())
		}
	}
	return nil
}

func openMariaAdmission(ctx context.Context, conn *sql.Conn, scope networkScope, grantees []mariaGrantee) error {
	for _, grantee := range grantees {
		if _, err := conn.ExecContext(ctx, grantee.grantStatement()); err != nil {
			return err
		}
	}
	return verifyMariaGrants(ctx, conn, scope, grantees, false)
}

func mariaSessionSettings(ctx context.Context, conn *sql.Conn, mode string) error {
	for _, statement := range []string{
		"SET SESSION sql_mode = '" + mode + "'", "SET SESSION autocommit = 1", "SET SESSION foreign_key_checks = 1",
		"SET SESSION lock_wait_timeout = 10", "SET SESSION sql_notes = 0", "SET SESSION innodb_lock_wait_timeout = 10",
		"SET SESSION time_zone = '+00:00'",
	} {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

// withMariaDBFence audits the target, closes admission to every other writer,
// runs the operation on one reserved administrator session, and reopens
// admission. When reopening cannot be verified, or run asks to keep admission
// closed, Desk keeps the maintenance lock so an operator can inspect the target.
func (m *Manager) withMariaDBFence(ctx context.Context, target TargetSpec, scope networkScope, removeLock *bool, run func(context.Context, *sql.Conn) error) (err error) {
	db, _, _, err := m.openTargetOnTunnel(target, scope.tunnel)
	if err != nil {
		return mariaFenceError("Could not open the MariaDB recovery connection.", err)
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return mariaFenceError("Could not reserve the MariaDB recovery connection.", err)
	}
	defer conn.Close()
	if err := mariaSessionSettings(ctx, conn, mariaSessionMode); err != nil {
		return mariaFenceError("Could not prepare the MariaDB recovery session.", err)
	}
	adm, err := mariadbAdmission(ctx, conn, scope)
	if err != nil {
		return err
	}
	var revoked []mariaGrantee
	keepClosed := false
	defer func() {
		if keepClosed {
			*removeLock = false
			return
		}
		if len(revoked) == 0 {
			return
		}
		// A canceled request must still reopen what it closed.
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		reopen, reopenErr := db.Conn(cleanup)
		if reopenErr == nil {
			defer reopen.Close()
			reopenErr = openMariaAdmission(cleanup, reopen, scope, revoked)
		}
		if reopenErr != nil {
			*removeLock = false
			err = security.NewError(security.CodeDeskE502, 502, "Desk could not restore MariaDB account access and retained the maintenance lock. Inspect the grants before retrying.", errors.Join(err, reopenErr))
		}
	}()
	if err := closeMariaAdmission(ctx, conn, scope, adm, &revoked); err != nil {
		return mariaFenceError("MariaDB could not exclude all other writers from the recovery database.", err)
	}
	if err := run(ctx, conn); err != nil {
		if errors.Is(err, errMariaKeepOff) {
			keepClosed = true
		}
		return err
	}
	return nil
}
