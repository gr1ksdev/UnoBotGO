package postgres

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/malbs/UnoGoBot/internal/rankingimport"
)

var _ rankingimport.Repository = (*Store)(nil)

// StageRankingImport stores parsed audit entries only. It never modifies ranking.
func (s *Store) StageRankingImport(ctx context.Context, source rankingimport.Import) (rankingimport.Import, error) {
	// Reparse the raw source so callers cannot smuggle different entries/hash into
	// an idempotent import. Candidate linking is a separate future authorized flow.
	parsed, err := rankingimport.New(source.ChatID, source.CreatedBy, source.RawText)
	if err != nil {
		return rankingimport.Import{}, err
	}
	if source.ID == "" || source.CreatedAt.IsZero() {
		return rankingimport.Import{}, rankingimport.ErrInvalid
	}
	parsed.ID = source.ID
	parsed.CreatedAt = source.CreatedAt
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return rankingimport.Import{}, operationError(ctx, "begin import")
	}
	defer tx.Rollback(context.Background())
	tag, err := tx.Exec(ctx, `INSERT INTO ranking_imports(import_id,chat_id,created_by,created_at,source_hash,raw_text) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(chat_id,source_hash) DO NOTHING`, parsed.ID, parsed.ChatID, parsed.CreatedBy, parsed.CreatedAt, parsed.Hash, parsed.RawText)
	if err != nil {
		return rankingimport.Import{}, operationError(ctx, "stage import")
	}
	if tag.RowsAffected() == 0 {
		if err = tx.QueryRow(ctx, `SELECT import_id,created_by,created_at FROM ranking_imports WHERE chat_id=$1 AND source_hash=$2`, parsed.ChatID, parsed.Hash).Scan(&parsed.ID, &parsed.CreatedBy, &parsed.CreatedAt); err != nil {
			return rankingimport.Import{}, operationError(ctx, "read existing import")
		}
		rows, err := tx.Query(ctx, `SELECT entry_id,line_number,imported_name,source_score_units,linked_user_id,status,COALESCE(parse_error,'') FROM ranking_import_entries WHERE import_id=$1 ORDER BY line_number`, parsed.ID)
		if err != nil {
			return rankingimport.Import{}, operationError(ctx, "read existing import entries")
		}
		parsed.Entries = nil
		for rows.Next() {
			var e rankingimport.Entry
			if err = rows.Scan(&e.ID, &e.Line, &e.ImportedName, &e.Score, &e.LinkedUserID, &e.Status, &e.Error); err != nil {
				rows.Close()
				return rankingimport.Import{}, operationError(ctx, "read existing import entries")
			}
			parsed.Entries = append(parsed.Entries, e)
		}
		rows.Close()
		if rows.Err() != nil {
			return rankingimport.Import{}, operationError(ctx, "read import entries")
		}
	} else {
		for _, e := range parsed.Entries {
			var parseError any
			if e.Error != "" {
				parseError = e.Error
			}
			if _, err = tx.Exec(ctx, `INSERT INTO ranking_import_entries(entry_id,import_id,line_number,imported_name,source_score_units,status,parse_error) VALUES($1,$2,$3,$4,$5,$6,$7)`, e.ID, parsed.ID, e.Line, e.ImportedName, int64(e.Score), e.Status, parseError); err != nil {
				return rankingimport.Import{}, operationError(ctx, "stage import entry")
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return rankingimport.Import{}, operationError(ctx, "commit import")
	}
	return parsed, nil
}
