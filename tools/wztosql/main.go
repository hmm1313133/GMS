// wztosql is the Go port of tools/wztosql/MonsterDropCreator.java
// (GMS-P3.5): it rebuilds a monster drop table from the wz export.
//
// Usage:
//
//	wztosql -out mobDrop.sql              # write the wz-derived drop_data
//	wztosql -stats                        # counts only
//	wztosql -diff "root:root@tcp(127.0.0.1:3306)/079-max2"
//	                                      # compare against the live database
//	wztosql -diff <dsn> -diffout diff.txt # … and dump the per-row differences
//
// It never writes to a database. This server's drop_data was tuned by hand by
// a third party and lives in the 079MAX2 MySQL database
// (K:\079MAX2服务端\mysql\MySQL\data\079@002dmax2); that database, not the wz
// files, defines what actually drops. Use -diff to see how far the two have
// drifted, and tools/dropexport to snapshot the authoritative rows.
package main

import (
	"flag"
	"fmt"
	"os"

	"GMS/internal/config"
	"GMS/internal/dropgen"
	"GMS/internal/wzs"
)

func main() {
	cfgPath := flag.String("config", "configs/gms.toml", "gms.toml path (for [wz].path)")
	wzPath := flag.String("wz", "", "wz export directory (default: [wz].path from config, else \"wz\")")
	out := flag.String("out", "mobDrop.sql", "output SQL file ('-' = stdout)")
	table := flag.String("table", "drop_data", "target table name written into the INSERTs")
	statsOnly := flag.Bool("stats", false, "print counts only")
	cardSuffix := flag.String("card-suffix", dropgen.DefaultCardSuffix,
		"monster-card name suffix stripped before matching a monster name")
	cardSuffixCN := flag.Bool("card-suffix-cn", false,
		"shortcut for -card-suffix 卡片 (the literal cannot be typed on this "+
			"box: PowerShell mangles non-ASCII argv, see docs/SESSION_STATE.md §三.9)")
	diffDSN := flag.String("diff", "", "MySQL DSN to diff the generated rows against")
	diffOut := flag.String("diffout", "", "write the per-row diff to this file too")
	flag.Parse()

	dir := resolveWZPath(*wzPath, *cfgPath)
	root, err := wzs.OpenRoot(dir)
	if err != nil {
		fail(err)
	}

	fmt.Fprintf(os.Stderr, "wztosql: reading %s\n", dir)
	opt := dropgen.Options{CardSuffix: *cardSuffix}
	if *cardSuffixCN {
		opt.CardSuffix = "卡片"
	}
	res, err := dropgen.GenerateWithOptions(root, opt)
	if err != nil {
		fail(err)
	}
	fmt.Fprintf(os.Stderr, "wztosql: %s\n", res.Summary())
	if len(res.Cards) < 10 && !*cardSuffixCN {
		fmt.Fprintln(os.Stderr, "wztosql: hint: almost no monster cards matched - this "+
			"distribution names them in Chinese, retry with -card-suffix-cn")
	}
	for _, e := range res.Errors {
		fmt.Fprintf(os.Stderr, "wztosql: warning: %s\n", e)
	}

	if *diffDSN != "" {
		if err := runDiff(*diffDSN, res, *diffOut); err != nil {
			fail(err)
		}
		if *statsOnly {
			return
		}
	}
	if *statsOnly {
		return
	}

	w := os.Stdout
	if *out != "-" {
		f, err := os.Create(*out)
		if err != nil {
			fail(err)
		}
		defer f.Close()
		w = f
	}
	if err := dropgen.WriteSQL(w, res, *table); err != nil {
		fail(err)
	}
	if *out != "-" {
		fmt.Printf("wrote %d entries -> %s\n", len(res.Entries), *out)
	}
}

func resolveWZPath(flagPath, cfgPath string) string {
	if flagPath != "" {
		return flagPath
	}
	if cfg, err := config.Load(cfgPath); err == nil && cfg.WZ.Path != "" {
		return cfg.WZ.Path
	}
	return "wz"
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "wztosql:", err)
	os.Exit(1)
}
