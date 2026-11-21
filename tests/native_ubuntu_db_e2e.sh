#!/usr/bin/env bash
set -euo pipefail

# Build both Linux amd64 binaries with go test -c.
test_binary=$(realpath -e "${1:?Pass the Linux amd64 dbstudio test binary}")
server_test_binary=""
if [[ -n "${2:-}" ]]; then server_test_binary=$(realpath -e "$2"); fi
fixture=$(mktemp -d /var/tmp/groxy-desk-native-db.XXXXXXXX)
printf 'Native fixture: %s\n' "$fixture"

cleanup() {
    if [[ -n "${maria_lp_pid:-}" ]]; then kill "$maria_lp_pid" 2>/dev/null || true; wait "$maria_lp_pid" || true; fi
    if [[ -n "${maria_pid:-}" ]]; then kill "$maria_pid" 2>/dev/null || true; wait "$maria_pid" || true; fi
    if [[ -n "${pg_pid:-}" ]]; then kill "$pg_pid" 2>/dev/null || true; wait "$pg_pid" || true; fi
    if [[ $(realpath -e "$fixture") = /var/tmp/groxy-desk-native-db.* ]]; then
        rm -rf -- "$fixture"
    fi
}
trap cleanup EXIT
trap 'exit 130' INT TERM

cd "$fixture"
apt download postgresql-18 postgresql-client-18 mariadb-server mariadb-server-core mariadb-client \
    mariadb-client-core mariadb-common libicu78 liburing2 libpq5 libxslt1.1 libaio1t64 libnuma1 libncurses6
mkdir pkg
for package in ./*.deb; do dpkg-deb -x "$package" pkg; done

export LD_LIBRARY_PATH="$fixture/pkg/usr/lib/x86_64-linux-gnu"
pg="$fixture/pkg/usr/lib/postgresql/18/bin"
maria="$fixture/pkg/usr"
printf '%s\n' desk-fixture-only > "$fixture/pg-password"
"$pg/initdb" --no-locale --encoding=UTF8 --auth-local=trust --auth-host=scram-sha-256 \
    --username=postgres --pwfile="$fixture/pg-password" --pgdata="$fixture/pgdata"
"$maria/bin/mariadb-install-db" --no-defaults --basedir="$maria" --auth-root-authentication-method=socket \
    --skip-test-db --datadir="$fixture/mariadata"
"$maria/bin/mariadb-install-db" --no-defaults --basedir="$maria" --auth-root-authentication-method=socket \
    --skip-test-db --datadir="$fixture/mariadata_lp"
"$pg/postgres" -D "$fixture/pgdata" -h 127.0.0.1 -p 51918 -k "$fixture" >pg.log 2>&1 &
pg_pid=$!
"$maria/sbin/mariadbd" --no-defaults --basedir="$maria" --datadir="$fixture/mariadata" \
    --bind-address=127.0.0.1 --port=51919 --socket="$fixture/maria.sock" --pid-file="$fixture/maria.pid" >maria.log 2>&1 &
maria_pid=$!
"$maria/sbin/mariadbd" --no-defaults --basedir="$maria" --datadir="$fixture/mariadata_lp" \
    --bind-address=127.0.0.1 --port=51920 --socket="$fixture/maria_lp.sock" --pid-file="$fixture/maria_lp.pid" >maria_lp.log 2>&1 &
maria_lp_pid=$!

for attempt in {1..100}; do
    if [[ -S "$fixture/maria.sock" && -S "$fixture/maria_lp.sock" ]] && "$pg/pg_isready" -h "$fixture" -p 51918 -U postgres >/dev/null; then
        break
    fi
    if ! kill -0 "$pg_pid" "$maria_pid" "$maria_lp_pid" 2>/dev/null; then cat pg.log maria.log maria_lp.log; exit 1; fi
    sleep 0.1
done
"$pg/createdb" -h "$fixture" -p 51918 -U postgres desk_fixture
"$pg/psql" -h "$fixture" -p 51918 -U postgres -d desk_fixture -Atc 'SELECT version();'
"$maria/bin/mariadb" --no-defaults --socket="$fixture/maria.sock" -u "$(id -un)" -e \
    "CREATE DATABASE desk_fixture; ALTER USER 'root'@'localhost' IDENTIFIED BY 'desk-fixture-only'; SELECT VERSION();"
"$maria/bin/mariadb" --no-defaults --socket="$fixture/maria_lp.sock" -u "$(id -un)" -e \
    "CREATE USER IF NOT EXISTS 'root'@'%' IDENTIFIED BY 'desk-fixture-only'; GRANT ALL PRIVILEGES ON *.* TO 'root'@'%' WITH GRANT OPTION; ALTER USER 'root'@'localhost' IDENTIFIED BY 'desk-fixture-only';"
printf 'systemd control group: %s\n' "$(cat /proc/self/cgroup)"
export GROXY_DESK_TEST_POSTGRES_PORT=51918 GROXY_DESK_TEST_MARIADB_PORT=51919 GROXY_DESK_TEST_MARIADB_LEAST_PRIV_PORT=51920
export PATH="$pg:$PATH"
export GROXY_DESK_TEST_PG_LOGICAL=1
"$test_binary" -test.count=1 -test.v -test.run '^TestPinned(DatabaseCatalogs|PostgresLogicalReplay)$'
if [[ -n "$server_test_binary" ]]; then
    "$server_test_binary" -test.count=1 -test.v -test.run '^TestPinnedNetworkRecoveryAPI$'
fi
