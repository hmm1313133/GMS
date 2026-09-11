package main

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"

	_ "github.com/go-sql-driver/mysql"

	"GMS/internal/dropgen"
)

// dropKey identifies one (monster, item) pair. A pair can legitimately appear
// several times (multipleDropsIncrement inserts one row per copy), so both
// sides are aggregated to row count + summed chance.
type dropKey struct {
	Mob  int
	Item int
}

type dropAgg struct {
	Rows   int
	Chance int
}

func runDiff(dsn string, res *dropgen.Result, diffOut string) error {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return fmt.Errorf("diff: connect: %w", err)
	}

	live, err := loadLiveDrops(db)
	if err != nil {
		return err
	}
	gen := aggregate(res.Entries)

	var onlyDB, onlyWZ, differs []dropKey
	dbRows, wzRows := 0, 0
	for k, a := range live {
		dbRows += a.Rows
		if _, ok := gen[k]; !ok {
			onlyDB = append(onlyDB, k)
		}
	}
	for k, a := range gen {
		wzRows += a.Rows
		la, ok := live[k]
		switch {
		case !ok:
			onlyWZ = append(onlyWZ, k)
		case la.Chance != a.Chance || la.Rows != a.Rows:
			differs = append(differs, k)
		}
	}
	sortKeys(onlyDB)
	sortKeys(onlyWZ)
	sortKeys(differs)

	fmt.Printf("diff vs %s\n", dsn)
	fmt.Printf("  database : %6d rows over %4d monsters / %5d pairs\n",
		dbRows, len(monstersOf(live)), len(live))
	fmt.Printf("  wz build : %6d rows over %4d monsters / %5d pairs\n",
		wzRows, len(monstersOf(gen)), len(gen))
	fmt.Printf("  only in database : %d pairs\n", len(onlyDB))
	fmt.Printf("  only in wz build : %d pairs\n", len(onlyWZ))
	fmt.Printf("  same pair, different chance/count : %d pairs\n", len(differs))
	fmt.Println("  (the database is authoritative - this is a drift report, not an error list)")

	if diffOut == "" {
		return nil
	}
	f, err := os.Create(diffOut)
	if err != nil {
		return err
	}
	defer f.Close()

	var b strings.Builder
	b.WriteString("# wz rebuild vs live drop_data\n")
	b.WriteString("# columns: mob item | database(rows,chance) | wz(rows,chance)\n\n")
	b.WriteString("## only in the database (hand-tuned rows the wz build cannot see)\n")
	for _, k := range onlyDB {
		a := live[k]
		fmt.Fprintf(&b, "%d %d | %d %d | -\n", k.Mob, k.Item, a.Rows, a.Chance)
	}
	b.WriteString("\n## only in the wz build\n")
	for _, k := range onlyWZ {
		a := gen[k]
		fmt.Fprintf(&b, "%d %d | - | %d %d\n", k.Mob, k.Item, a.Rows, a.Chance)
	}
	b.WriteString("\n## different chance / row count\n")
	for _, k := range differs {
		l, g := live[k], gen[k]
		fmt.Fprintf(&b, "%d %d | %d %d | %d %d\n", k.Mob, k.Item, l.Rows, l.Chance, g.Rows, g.Chance)
	}
	if _, err := f.WriteString(b.String()); err != nil {
		return err
	}
	fmt.Printf("  details -> %s\n", diffOut)
	return nil
}

func loadLiveDrops(db *sql.DB) (map[dropKey]dropAgg, error) {
	rows, err := db.Query("SELECT dropperid, itemid, chance FROM drop_data")
	if err != nil {
		return nil, fmt.Errorf("diff: query drop_data: %w", err)
	}
	defer rows.Close()

	out := map[dropKey]dropAgg{}
	for rows.Next() {
		var mob, item, chance int
		if err := rows.Scan(&mob, &item, &chance); err != nil {
			return nil, fmt.Errorf("diff: scan drop_data: %w", err)
		}
		k := dropKey{mob, item}
		a := out[k]
		a.Rows++
		a.Chance += chance
		out[k] = a
	}
	return out, rows.Err()
}

func aggregate(entries []dropgen.Entry) map[dropKey]dropAgg {
	out := map[dropKey]dropAgg{}
	for _, e := range entries {
		k := dropKey{e.MonsterID, e.ItemID}
		a := out[k]
		a.Rows++
		a.Chance += e.Chance
		out[k] = a
	}
	return out
}

func monstersOf(m map[dropKey]dropAgg) map[int]bool {
	out := map[int]bool{}
	for k := range m {
		out[k.Mob] = true
	}
	return out
}

func sortKeys(keys []dropKey) {
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Mob != keys[j].Mob {
			return keys[i].Mob < keys[j].Mob
		}
		return keys[i].Item < keys[j].Item
	})
}
