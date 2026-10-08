package main

// ============================================================================
// TEMPORARY — one-off DB data export for Railway account migration.
// Hapus file ini segera setelah export selesai; JANGAN biarkan di main.
// Endpoint: GET /api/admin/tmp-export?token=<EXPORT_TOKEN>[&table=<nama>][&sequences=1]
// Format: data-only dump ala pg_dump (COPY ... FROM stdin; + setval).
// ============================================================================

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gofiber/fiber/v3"
)

var tmpExportTables = []string{
	"subjects", "cps", "users", "students", "rombel", "materials",
	"assignments", "submissions", "attendance", "news", "exams",
	"questions", "attempts", "answers", "cheat_signals", "ai_keys",
}

func (s *Server) tmpExport(c fiber.Ctx) error {
	tok := os.Getenv("EXPORT_TOKEN")
	if tok == "" || c.Query("token") != tok {
		return c.SendStatus(fiber.StatusNotFound)
	}
	tables := tmpExportTables
	if t := strings.ToLower(strings.TrimSpace(c.Query("table"))); t != "" {
		tables = []string{t}
	}
	wantSeq := c.Query("sequences") != ""
	c.Set("Content-Type", "application/sql")
	c.Set("Content-Disposition", `attachment; filename="elearning-data.sql"`)
	return c.SendStreamWriter(func(w *bufio.Writer) {
		ctx := context.Background()
		conn, err := s.pool.Acquire(ctx)
		if err != nil {
			fmt.Fprintf(w, "-- ACQUIRE ERROR: %v\n", err)
			w.Flush()
			return
		}
		defer conn.Release()
		defer w.Flush()

		for _, t := range tables {
			var cols string
			err := conn.QueryRow(ctx,
				`SELECT string_agg(quote_ident(column_name), ', ' ORDER BY ordinal_position)
				 FROM information_schema.columns
				 WHERE table_schema='public' AND table_name=$1`, t).Scan(&cols)
			if err != nil || cols == "" {
				fmt.Fprintf(w, "-- SKIP %s: %v\n", t, err)
				continue
			}
			fmt.Fprintf(w, "COPY public.%s (%s) FROM stdin;\n", t, cols)
			if _, err := conn.Conn().PgConn().CopyTo(ctx, w, "COPY public."+t+" TO STDOUT"); err != nil {
				fmt.Fprintf(w, "-- COPY ERROR %s: %v\n", t, err)
				return
			}
			fmt.Fprintf(w, "\\.\n")
		}
		if wantSeq {
			rows, err := conn.Query(ctx, `
				SELECT format('SELECT pg_catalog.setval(%L, COALESCE((SELECT MAX(%I) FROM %I.%I), 1), true);',
					n.nspname||'.'||c.relname, a.attname, n.nspname, t.relname)
				FROM pg_class c
				JOIN pg_namespace n ON n.oid = c.relnamespace
				JOIN pg_depend d ON d.objid = c.oid AND d.deptype = 'i'
				JOIN pg_attribute a ON a.attrelid = d.refobjid AND a.attnum = d.refobjsubid
				JOIN pg_class t ON t.oid = d.refobjid
				WHERE c.relkind = 'S' AND n.nspname = 'public'`)
			if err != nil {
				fmt.Fprintf(w, "-- SEQ QUERY ERROR: %v\n", err)
				return
			}
			defer rows.Close()
			for rows.Next() {
				var stmt string
				if err := rows.Scan(&stmt); err != nil {
					continue
				}
				fmt.Fprintln(w, stmt)
			}
		}
	})
}
