"""Verify native dumps and Desk's pinned HTTP recovery on disposable databases."""

import argparse
import hashlib
import os
import secrets
import subprocess
import time
from contextlib import ExitStack, contextmanager
from pathlib import Path


PASSWORD = "desk-fixture-only"
DATABASE = "desk_fixture"
SEED = """
CREATE TABLE authors (id BIGINT PRIMARY KEY, name VARCHAR(80) NOT NULL);
CREATE TABLE books (
    id BIGINT PRIMARY KEY,
    author_id BIGINT REFERENCES authors(id),
    price DECIMAL(12,2) NOT NULL,
    note TEXT,
    cover BLOB_TYPE
);
CREATE INDEX books_by_author ON books(author_id);
CREATE VIEW priced_books AS SELECT id, price FROM books;
INSERT INTO authors VALUES (1, 'Ada');
INSERT INTO books VALUES (9007199254740993, 1, 0.00, NULL, BLOB_VALUE);
INSERT INTO books VALUES (9007199254740994, 1, 9.00, '', NULL);
"""


def docker(*args, data=None, check=True):
    return subprocess.run(
        ["docker", *args], input=data, capture_output=True, timeout=60, check=check,
    )


@contextmanager
def database(engine, publish=False):
    name = f"groxy-desk-{engine}-recovery-{secrets.token_hex(6)}"
    if engine == "postgres":
        image = "postgres:18"
        config = ("POSTGRES_PASSWORD", "POSTGRES_DB")
        client_env = "PGPASSWORD"
        client = ("psql", "-X", "-U", "postgres", "--set=ON_ERROR_STOP=1")
    elif engine == "mariadb":
        image = "mariadb:11.8"
        config = ("MARIADB_ROOT_PASSWORD", "MARIADB_DATABASE")
        client_env = "MYSQL_PWD"
        client = ("mariadb", "--no-defaults", "-u", "root", "--batch")
    else:
        assert engine == "mysql", engine
        image = "mysql:8.4"
        config = ("MYSQL_ROOT_PASSWORD", "MYSQL_DATABASE")
        client_env = "MYSQL_PWD"
        client = ("mysql", "--no-defaults", "-u", "root", "--batch")
    if publish:
        engine_port = 5432 if engine == "postgres" else 3306
        network = ("--publish", f"127.0.0.1::{engine_port}")
    else:
        network = ("--network", "none")
    fresh_root = ("--env", "MYSQL_ROOT_HOST=%") if engine == "mysql" else ()
    docker("run", "--detach", "--rm", *network, "--name", name, *fresh_root,
           "--env", f"{config[0]}={PASSWORD}", "--env", f"{config[1]}={DATABASE}", image)

    def run(*args, data=None, check=True):
        return docker("exec", "-i", "--env", f"{client_env}={PASSWORD}", name, *args, data=data, check=check)

    try:
        for _ in range(60):
            # Initialization temporarily launches a server without TCP before
            # switching to the real instance. Wait for TCP inside the fixture.
            host = ("-h", "127.0.0.1") if engine == "postgres" else ("--protocol=TCP", "-h", "127.0.0.1")
            probe = run(*client, *host, DATABASE, data=b"SELECT 1;\n", check=False)
            if probe.returncode == 0:
                break
            time.sleep(0.5)
        else:
            raise RuntimeError(f"{engine} fixture did not start: {probe.stderr.decode(errors='replace')}")
        host_port = None
        if publish:
            address = docker("port", name, f"{engine_port}/tcp").stdout.decode().strip()
            host, host_port = address.split(":")
            assert host == "127.0.0.1", f"unexpected published database address {address}"
        version = run(*client, "--skip-column-names" if engine != "postgres" else "--tuples-only",
                      DATABASE, data=b"SELECT VERSION();\n").stdout.decode().strip()
        print(f"{engine} fixture: {version}", flush=True)
        yield run, client, host_port
    finally:
        docker("stop", "--time", "5", name)


def postgres_round_trip():
    with database("postgres") as (run, client, _):
        def dump():
            return run(
                "pg_dump", "-U", "postgres", "--create", "--clean", "--if-exists",
                "--restrict-key=GroxyDeskRecoveryFixture110", DATABASE,
            ).stdout

        seed = SEED.replace("BLOB_TYPE", "BYTEA").replace("BLOB_VALUE", "decode('00ff','hex')")
        run(*client, DATABASE, data=(seed + "CREATE SEQUENCE book_number START 97; SELECT nextval('book_number');").encode())
        snapshot = dump()
        digest = hashlib.sha256(snapshot).hexdigest()
        run(*client, DATABASE, data=b"ALTER TABLE books ADD COLUMN changed TEXT; DELETE FROM books; SELECT nextval('book_number');")
        assert dump() != snapshot, "fixture did not change"
        assert hashlib.sha256(snapshot).hexdigest() == digest
        run(*client, "postgres", data=snapshot)
        assert hashlib.sha256(dump()).hexdigest() == digest, "PostgreSQL full restore differs from its snapshot"
        print(f"PostgreSQL full schema, data and sequence round trip verified ({digest})")


def mariadb_partial_ddl_round_trip():
    with database("mariadb") as (run, client, _):
        def dump():
            return run(
                "mariadb-dump", "--no-defaults", "-u", "root", "--single-transaction",
                "--order-by-primary", "--hex-blob", "--routines", "--events",
                "--skip-dump-date", "--add-drop-database", "--databases", DATABASE,
            ).stdout

        seed = SEED.replace("BLOB_TYPE", "BLOB").replace("BLOB_VALUE", "X'00ff'")
        run(*client, DATABASE, data=(seed + "CREATE PROCEDURE book_count() SELECT COUNT(*) FROM books;").encode())
        snapshot = dump()
        digest = hashlib.sha256(snapshot).hexdigest()
        failed = run(*client, DATABASE, check=False, data=b"""
START TRANSACTION;
ALTER TABLE books ADD COLUMN partial_change INT;
UPDATE books SET price=1.25 WHERE id=9007199254740993;
ALTER TABLE books ADD COLUMN partial_change INT;
COMMIT;
""")
        assert failed.returncode != 0 and b"ERROR 1060" in failed.stderr, failed.stderr
        columns = run(*client, "--skip-column-names", DATABASE, data=b"SHOW COLUMNS FROM books LIKE 'partial_change';")
        price = run(*client, "--skip-column-names", DATABASE, data=b"SELECT price FROM books WHERE id=9007199254740993;")
        assert columns.stdout.startswith(b"partial_change\t"), "first DDL did not survive the second DDL failure"
        assert price.stdout.strip() == b"1.25", "implicit commit did not persist the intervening DML"
        assert dump() != snapshot, "fixture did not change"
        assert hashlib.sha256(snapshot).hexdigest() == digest
        run(*client, data=snapshot)
        assert hashlib.sha256(dump()).hexdigest() == digest, "MariaDB partial DDL restore differs from its snapshot"
        print(f"MariaDB recovered exact schema, routines and data after partially committed DDL ({digest})")


def desk_api_round_trip():
    with ExitStack() as stack:
        _, _, pg_port = stack.enter_context(database("postgres", publish=True))
        _, _, maria_port = stack.enter_context(database("mariadb", publish=True))
        _, _, mysql_port = stack.enter_context(database("mysql", publish=True))
        env = dict(os.environ, GROXY_DESK_TEST_POSTGRES_PORT=pg_port, GROXY_DESK_TEST_MARIADB_PORT=maria_port,
                   GROXY_DESK_TEST_MYSQL_PORT=mysql_port, GROXY_DESK_TEST_PG_LOGICAL="1")
        subprocess.run(
            ["go", "test", "-p=1", "-count=1", "-v", "./internal/dbstudio", "./internal/server",
             "-run", "^Test(PinnedDatabaseCatalogs|PinnedPostgresLogicalReplay|PinnedNetworkRecoveryAPI)$"],
            cwd=Path(__file__).resolve().parents[1], env=env, timeout=300, check=True,
        )


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--desk-api", action="store_true", help="also exercise the Desk recovery endpoints (requires PostgreSQL 18 pg_dump on PATH)")
    args = parser.parse_args()
    postgres_round_trip()
    mariadb_partial_ddl_round_trip()
    if args.desk_api:
        desk_api_round_trip()
