package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"api-students/app/model"

	"github.com/jackc/pgx/v5"
)

var (
	ErrSupervisorFull     = errors.New("supervisor quota full")
	ErrAlreadySupervised  = errors.New("student already supervised")
	ErrNoSupervision      = errors.New("student has no supervision")
	ErrVersionConflict    = errors.New("version conflict")
	ErrQuotaBelowAssigned = errors.New("quota below current supervised count")
)

type SupervisionRepository interface {
	WithTx(DBTX) SupervisionRepository

	ListSupervisors(context.Context) ([]model.Supervisor, error)
	FindSupervisor(context.Context, int) (model.Supervisor, error)

	LockStudent(context.Context, int) error
	CurrentSupervisor(context.Context, int) (int, error)
	LockSupervisors(context.Context, ...int) error

	TakeQuota(context.Context, int) error
	ReleaseQuota(context.Context, int) error
	Insert(context.Context, int, int) error
	Delete(context.Context, int) error

	Update(
		context.Context,
		int,
		model.UpdateSupervisorRequest,
	) (model.Supervisor, error)
}

type supervisionRepository struct {
	db DBTX
}

func NewSupervisionRepository(db DBTX) SupervisionRepository {
	return &supervisionRepository{db: db}
}

func (r *supervisionRepository) WithTx(tx DBTX) SupervisionRepository {
	return &supervisionRepository{db: tx}
}

func (r *supervisionRepository) ListSupervisors(
	ctx context.Context,
) ([]model.Supervisor, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT id, name, quota, supervised_count, version
		 FROM supervisors
		 ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []model.Supervisor{}

	for rows.Next() {
		var supervisor model.Supervisor

		if err := rows.Scan(
			&supervisor.ID,
			&supervisor.Name,
			&supervisor.Quota,
			&supervisor.SupervisedCount,
			&supervisor.Version,
		); err != nil {
			return nil, err
		}

		result = append(result, supervisor)
	}

	return result, rows.Err()
}

func (r *supervisionRepository) FindSupervisor(
	ctx context.Context,
	id int,
) (model.Supervisor, error) {
	var supervisor model.Supervisor

	err := r.db.QueryRow(
		ctx,
		`SELECT id, name, quota, supervised_count, version
		 FROM supervisors
		 WHERE id = $1`,
		id,
	).Scan(
		&supervisor.ID,
		&supervisor.Name,
		&supervisor.Quota,
		&supervisor.SupervisedCount,
		&supervisor.Version,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return supervisor, ErrNotFound
	}

	return supervisor, err
}

func (r *supervisionRepository) LockStudent(
	ctx context.Context,
	id int,
) error {
	var studentID int

	err := r.db.QueryRow(
		ctx,
		`SELECT id
		 FROM students
		 WHERE id = $1
		 FOR UPDATE`,
		id,
	).Scan(&studentID)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}

	return err
}

func (r *supervisionRepository) CurrentSupervisor(
	ctx context.Context,
	studentID int,
) (int, error) {
	var supervisorID int

	err := r.db.QueryRow(
		ctx,
		`SELECT supervisor_id
		 FROM supervisions
		 WHERE student_id = $1`,
		studentID,
	).Scan(&supervisorID)

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNoSupervision
	}

	return supervisorID, err
}

func (r *supervisionRepository) LockSupervisors(
	ctx context.Context,
	ids ...int,
) error {
	ids = append([]int(nil), ids...)
	sort.Ints(ids)

	unique := ids[:0]
	for _, id := range ids {
		if len(unique) == 0 || unique[len(unique)-1] != id {
			unique = append(unique, id)
		}
	}

	rows, err := r.db.Query(
		ctx,
		`SELECT id
		 FROM supervisors
		 WHERE id = ANY($1)
		 ORDER BY id
		 FOR UPDATE`,
		unique,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	found := 0
	for rows.Next() {
		found++
	}

	if err := rows.Err(); err != nil {
		return err
	}

	if found != len(unique) {
		return ErrNotFound
	}

	return nil
}

func (r *supervisionRepository) TakeQuota(
	ctx context.Context,
	id int,
) error {
	tag, err := r.db.Exec(
		ctx,
		`UPDATE supervisors
		 SET supervised_count = supervised_count + 1
		 WHERE id = $1
		   AND supervised_count < quota`,
		id,
	)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		if _, err := r.FindSupervisor(ctx, id); err != nil {
			return err
		}

		return ErrSupervisorFull
	}

	return nil
}

func (r *supervisionRepository) ReleaseQuota(
	ctx context.Context,
	id int,
) error {
	tag, err := r.db.Exec(
		ctx,
		`UPDATE supervisors
		 SET supervised_count = supervised_count - 1
		 WHERE id = $1
		   AND supervised_count > 0`,
		id,
	)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf(
			"release supervisor quota: %w",
			ErrNotFound,
		)
	}

	return nil
}

func (r *supervisionRepository) Insert(
	ctx context.Context,
	studentID int,
	supervisorID int,
) error {
	_, err := r.db.Exec(
		ctx,
		`INSERT INTO supervisions (student_id, supervisor_id)
		 VALUES ($1, $2)`,
		studentID,
		supervisorID,
	)

	return err
}

func (r *supervisionRepository) Delete(
	ctx context.Context,
	studentID int,
) error {
	tag, err := r.db.Exec(
		ctx,
		`DELETE FROM supervisions
		 WHERE student_id = $1`,
		studentID,
	)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return ErrNoSupervision
	}

	return nil
}

func (r *supervisionRepository) Update(
	ctx context.Context,
	id int,
	req model.UpdateSupervisorRequest,
) (model.Supervisor, error) {
	var name any

	if req.Name != nil {
		name = *req.Name
	}

	var supervisor model.Supervisor

	err := r.db.QueryRow(
		ctx,
		`UPDATE supervisors
		 SET name = COALESCE($1, name),
		     quota = $2,
		     version = version + 1
		 WHERE id = $3
		   AND version = $4
		   AND $2 >= supervised_count
		 RETURNING id, name, quota, supervised_count, version`,
		name,
		req.Quota,
		id,
		req.Version,
	).Scan(
		&supervisor.ID,
		&supervisor.Name,
		&supervisor.Quota,
		&supervisor.SupervisedCount,
		&supervisor.Version,
	)

	if err == nil {
		return supervisor, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return supervisor, err
	}

	current, findErr := r.FindSupervisor(ctx, id)
	if findErr != nil {
		return supervisor, findErr
	}

	if current.Version != req.Version {
		return supervisor, ErrVersionConflict
	}

	return supervisor, ErrQuotaBelowAssigned
}