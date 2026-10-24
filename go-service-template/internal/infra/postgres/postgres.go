package postgres

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/joserafaelSH/golang-software-engineer/go-service-template/internal/config"
	_ "github.com/lib/pq"
)

func NewPostgresClient(cfg config.Config) *sql.DB {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		cfg.Postgres.Host, cfg.Postgres.Port, cfg.Postgres.Username, cfg.Postgres.Password, cfg.Postgres.Database)

	// 3. Open connection (does not verify connectivity yet)
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	db.SetMaxOpenConns(cfg.Postgres.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Postgres.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Postgres.ConnMaxLifetime)

	return db
}

// Job mirrors a row of the jobs table (see foundation/terraform/modules/infra/init.sql).
type Job struct {
	ID      int64
	Payload json.RawMessage
	Status  string
}

// FetchPendingJobs returns up to limit jobs currently in "pending" status,
// oldest first.
func FetchPendingJobs(db *sql.DB, limit int) ([]Job, error) {
	rows, err := db.Query(
		`SELECT id, payload, status FROM jobs WHERE status = 'pending' ORDER BY id LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("fetching pending jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.Payload, &j.Status); err != nil {
			return nil, fmt.Errorf("scanning job row: %w", err)
		}
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading pending jobs: %w", err)
	}

	return jobs, nil
}

// UpdateJobStatus transitions job id's status from -> to. The WHERE clause
// only matches rows still in the "from" status, so it's safe to call from
// multiple workers: whoever's update actually affects a row (ok == true)
// won the transition; everyone else gets false and should back off.
func UpdateJobStatus(db *sql.DB, id int64, to string) (bool, error) {
	res, err := db.Exec(
		`UPDATE jobs SET status = $1 WHERE id = $2`,
		to, id,
	)
	if err != nil {
		return false, fmt.Errorf("updating job %d status %s : %w", id, to, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("reading rows affected for job %d: %w", id, err)
	}

	return affected > 0, nil
}

func BulkUpdateJobStatus(db *sql.DB, ids []int64, to string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res, err := db.Exec(
		`UPDATE jobs SET status = $1 WHERE id = ANY($2)`,
		to, ids,
	)
	if err != nil {
		return 0, fmt.Errorf("bulk updating job status %s: %w", to, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("reading rows affected for bulk update: %w", err)
	}
	return affected, nil
}
