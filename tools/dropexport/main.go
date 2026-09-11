// dropexport exports the monster drop tables out of the authoritative
// 079-max2 MySQL database (K:\079MAX2服务端\mysql\MySQL\data\079@002dmax2)
// into a portable SQL file that both backends can replay:
//
//	dropexport -dsn "root:root@tcp(127.0.0.1:3306)/079-max2" -out migrations/0002_drop_data.sql
//	dropexport -stats                     # row counts only, no file written
//
// Why this tool exists (GMS-P3.5, docs/PLAN.md):
//
//	this server's drop tables were seeded by a third party, they are NOT
//	generated from the wz files (tools/wztosql/MonsterDropCreator.java is
//	the generator the upstream project used, but the 079MAX2 distribution
//	shipped its own hand-tuned drop_data rows). The database is therefore
//	the source of truth and this tool snapshots it, so the Go tree can be
//	provisioned without the MySQL 5.5 distribution running.
//
// Output is INSERT-only (no DDL): migrations/0001_base.sql already carries
// CREATE TABLE for every backend, and the SQLite path provisions its own
// translated schema in code (internal/database/db.go), so a data-only file
// replays identically under MySQL and SQLite.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// tableSpec describes one table to snapshot. skip lists surrogate columns
// that must not be exported (the target re-assigns its own AUTO_INCREMENT
// ids, exactly like MonsterDropCreator's "(DEFAULT, ...)" rows). orderBy
// makes the output byte-stable so re-exporting produces a clean diff.
type tableSpec struct {
	name    string
	skip    map[string]bool
	orderBy string
}

var specs = []tableSpec{
	{
		name:    "drop_data",
		skip:    map[string]bool{"id": true},
		orderBy: "dropperid, itemid, chance, minimum_quantity, maximum_quantity, questid",
	},
	{
		name:    "drop_data_global",
		skip:    map[string]bool{"id": true},
		orderBy: "continent, dropType, itemid, chance, questid",
	},
}

func main() {
	dsn := flag.String("dsn", "root:root@tcp(127.0.0.1:3306)/079-max2", "source MySQL DSN (authoritative 079-max2)")
	out := flag.String("out", "migrations/0002_drop_data.sql", "output SQL file ('-' = stdout)")
	only := flag.String("tables", "", "comma separated subset of tables (default: all)")
	batch := flag.Int("batch", 500, "rows per INSERT statement")
	stats := flag.Bool("stats", false, "print row counts only")
	flag.Parse()

	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		fail(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		fail(fmt.Errorf("connect %s: %w", *dsn, err))
	}

	want := specs
	if *only != "" {
		keep := map[string]bool{}
		for _, t := range strings.Split(*only, ",") {
			keep[strings.TrimSpace(t)] = true
		}
		want = nil
		for _, s := range specs {
			if keep[s.name] {
				want = append(want, s)
			}
		}
		if len(want) == 0 {
			fail(fmt.Errorf("no known table in -tables %q", *only))
		}
	}

	src := strings.TrimSpace(sourceLabel(*dsn))

	if *stats {
		for _, s := range want {
			n, err := countRows(db, s.name)
			if err != nil {
				fail(err)
			}
			fmt.Printf("%-18s %d rows\n", s.name, n)
		}
		return
	}

	var w io.Writer = os.Stdout
	var f *os.File
	if *out != "-" {
		f, err = os.Create(*out)
		if err != nil {
			fail(err)
		}
		defer f.Close()
		w = f
	}

	total := 0
	counts := make([]string, 0, len(want))
	for _, s := range want {
		n, err := dumpTable(db, s, w, *batch)
		if err != nil {
			fail(err)
		}
		total += n
		counts = append(counts, fmt.Sprintf("%s=%d", s.name, n))
	}

	// Header first: the file must be self-describing, but the counts are only
	// known after dumping, so stdout is written here too (the file already
	// carries them, see dumpTable's deferred header buffer below).
	if f != nil {
		if err := writeHeader(f, src, counts, total); err != nil {
			fail(err)
		}
	}
	fmt.Printf("exported %d rows (%s) -> %s\n", total, strings.Join(counts, ", "), *out)
}

// dumpTable writes the INSERT statements for one table. Because the file
// header needs the row counts, the body is buffered and prefixed afterwards
// (the header is written by writeHeader on the same file handle, at offset 0,
// which works because we build the body in memory first).
func dumpTable(db *sql.DB, s tableSpec, w io.Writer, batch int) (int, error) {
	q := fmt.Sprintf("SELECT * FROM `%s` ORDER BY %s", s.name, s.orderBy)
	rows, err := db.Query(q)
	if err != nil {
		return 0, fmt.Errorf("query %s: %w", s.name, err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return 0, err
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		return 0, err
	}

	keep := make([]int, 0, len(cols))
	for i, c := range cols {
		if s.skip[c] {
			continue
		}
		keep = append(keep, i)
	}
	numeric := make([]bool, len(types))
	for i, t := range types {
		numeric[i] = isNumeric(t.DatabaseTypeName())
	}

	vals := make([]sql.NullString, len(cols))
	ptrs := make([]interface{}, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}

	var body strings.Builder
	fmt.Fprintf(&body, "\n-- \n-- 表 %s\n-- \n", s.name)
	colList := make([]string, 0, len(keep))
	for _, i := range keep {
		colList = append(colList, "`"+cols[i]+"`")
	}

	head := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES\n", s.name, strings.Join(colList, ", "))
	n := 0
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return n, fmt.Errorf("scan %s: %w", s.name, err)
		}
		if n%batch == 0 {
			if n > 0 {
				body.WriteString(";\n")
			}
			body.WriteString(head)
		} else {
			body.WriteString(",\n")
		}
		body.WriteString("(")
		for k, i := range keep {
			if k > 0 {
				body.WriteString(", ")
			}
			body.WriteString(literal(vals[i], numeric[i]))
		}
		body.WriteString(")")
		n++
	}
	if err := rows.Err(); err != nil {
		return n, fmt.Errorf("rows %s: %w", s.name, err)
	}
	if n > 0 {
		body.WriteString(";\n")
	} else {
		body.WriteString("-- (empty table, no INSERT emitted)\n")
	}

	if _, err := io.WriteString(w, body.String()); err != nil {
		return n, err
	}
	return n, nil
}

// writeHeader prepends the provenance header. It re-opens the file and
// rewrites it (header + captured body) because the counts are only known
// after the dump.
func writeHeader(f *os.File, src string, counts []string, total int) error {
	body, err := os.ReadFile(f.Name())
	if err != nil {
		return err
	}
	var h strings.Builder
	h.WriteString("-- GMS-P3.5 掉落表种子数据 (generated by tools/dropexport, do not hand-edit)\n")
	h.WriteString("--\n")
	fmt.Fprintf(&h, "-- source : %s\n", src)
	fmt.Fprintf(&h, "-- rows   : %s (total %d)\n", strings.Join(counts, ", "), total)
	h.WriteString("--\n")
	h.WriteString("-- 本服掉落数据是第三方预置的，不是由 wz 生成：以本库的 drop_data /\n")
	h.WriteString("-- drop_data_global 为准（tools/wztosql 只做 wz 侧重建与对比，不覆盖）。\n")
	h.WriteString("-- 纯数据文件（无 DDL）：建表见 migrations/0001_base.sql（MySQL）与\n")
	h.WriteString("-- internal/database/db.go 的 sqliteSchema（SQLite，启动时自动建表并导入本数据）。\n")
	h.WriteString("--\n")
	h.WriteString("-- 重新导出：dropexport -dsn \"root:root@tcp(127.0.0.1:3306)/079-max2\" -out migrations/0002_drop_data.sql\n\n")
	h.Write(body)
	return os.WriteFile(f.Name(), []byte(h.String()), 0o644)
}

func countRows(db *sql.DB, table string) (int, error) {
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM `" + table + "`").Scan(&n); err != nil {
		return 0, fmt.Errorf("count %s: %w", table, err)
	}
	return n, nil
}

// literal renders one value for the INSERT. NULL is emitted as the bare
// NULL keyword; numeric columns stay unquoted so both MySQL and SQLite keep
// the column's declared type.
func literal(v sql.NullString, numeric bool) string {
	if !v.Valid {
		return "NULL"
	}
	s := v.String
	if numeric {
		s = strings.TrimSpace(s)
		if s == "" {
			return "0"
		}
		return s
	}
	return "'" + escapeLiteral(s) + "'"
}

// escapeLiteral quotes a string for MySQL by default (backslash escapes on),
// which SQLite reads literally. Drop data carries no backslashes or control
// characters, so the two dialects agree here; documented rather than
// silently ignored.
func escapeLiteral(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString("\\\\")
		case '\'':
			b.WriteString("\\'")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case 0:
			b.WriteString("\\0")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isNumeric(typeName string) bool {
	switch strings.ToUpper(typeName) {
	case "TINYINT", "SMALLINT", "MEDIUMINT", "INT", "INTEGER", "BIGINT",
		"YEAR", "FLOAT", "DOUBLE", "DECIMAL", "NEWDECIMAL":
		return true
	}
	return false
}

// sourceLabel strips the password out of the DSN for the file header.
func sourceLabel(dsn string) string {
	at := strings.LastIndex(dsn, "@")
	if at < 0 {
		return dsn
	}
	return "***" + dsn[at:]
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "dropexport:", err)
	os.Exit(1)
}
