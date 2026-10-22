package service

import (
	"errors"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"

	"github.com/gofiber/fiber/v2"
)

type SupervisionService struct {
	repo repository.SupervisionRepository
	tx   *repository.TxManager
}

func NewSupervisionService(
	repo repository.SupervisionRepository,
	tx *repository.TxManager,
) *SupervisionService {
	return &SupervisionService{
		repo: repo,
		tx:   tx,
	}
}

func (s *SupervisionService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	rows, err := s.repo.ListSupervisors(ctx)
	if err != nil {
		return helper.Internal(err)
	}

	return helper.Success(
		c,
		fiber.StatusOK,
		"daftar dosen pembimbing",
		rows,
	)
}

func (s *SupervisionService) Assign(c *fiber.Ctx) error {
	id, ok := helper.ParamID(c)
	if !ok {
		return helper.BadRequest("id harus berupa angka positif")
	}

	var req model.AssignSupervisionRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}

	if fields := helper.ValidateStruct(req); fields != nil {
		return helper.Validation(fields)
	}

	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	err := s.tx.WithTx(ctx, func(tx repository.DBTX) error {
		repo := s.repo.WithTx(tx)

		if err := repo.LockStudent(ctx, req.StudentID); err != nil {
			return err
		}

		if _, err := repo.CurrentSupervisor(ctx, req.StudentID); err == nil {
			return repository.ErrAlreadySupervised
		} else if !errors.Is(err, repository.ErrNoSupervision) {
			return err
		}

		if err := repo.LockSupervisors(ctx, id); err != nil {
			return err
		}

		if err := repo.TakeQuota(ctx, id); err != nil {
			return err
		}

		return repo.Insert(ctx, req.StudentID, id)
	})

	if err != nil {
		return supervisionError(err)
	}

	return helper.Success(
		c,
		fiber.StatusCreated,
		"dosen pembimbing berhasil ditetapkan",
		fiber.Map{
			"student_id":    req.StudentID,
			"supervisor_id": id,
		},
	)
}

func (s *SupervisionService) Switch(c *fiber.Ctx) error {
	var req model.SwitchSupervisionRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}

	if fields := helper.ValidateStruct(req); fields != nil {
		return helper.Validation(fields)
	}

	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	err := s.tx.WithTx(ctx, func(tx repository.DBTX) error {
		repo := s.repo.WithTx(tx)

		if err := repo.LockStudent(ctx, req.StudentID); err != nil {
			return err
		}

		oldID, err := repo.CurrentSupervisor(ctx, req.StudentID)
		if err != nil {
			return err
		}

		if oldID == req.NewSupervisorID {
			return repository.ErrAlreadySupervised
		}

		if err := repo.LockSupervisors(
			ctx,
			oldID,
			req.NewSupervisorID,
		); err != nil {
			return err
		}

		if err := repo.TakeQuota(ctx, req.NewSupervisorID); err != nil {
			return err
		}

		if err := repo.Delete(ctx, req.StudentID); err != nil {
			return err
		}

		if err := repo.ReleaseQuota(ctx, oldID); err != nil {
			return err
		}

		return repo.Insert(
			ctx,
			req.StudentID,
			req.NewSupervisorID,
		)
	})

	if err != nil {
		return supervisionError(err)
	}

	return helper.Success(
		c,
		fiber.StatusOK,
		"dosen pembimbing berhasil diganti",
		fiber.Map{
			"student_id":    req.StudentID,
			"supervisor_id": req.NewSupervisorID,
		},
	)
}

func (s *SupervisionService) Update(c *fiber.Ctx) error {
	id, ok := helper.ParamID(c)
	if !ok {
		return helper.BadRequest("id harus berupa angka positif")
	}

	var req model.UpdateSupervisorRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}

	if fields := helper.ValidateStruct(req); fields != nil {
		return helper.Validation(fields)
	}

	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	updated, err := s.repo.Update(ctx, id, req)
	if err != nil {
		return supervisionError(err)
	}

	return helper.Success(
		c,
		fiber.StatusOK,
		"data dosen pembimbing diperbarui",
		updated,
	)
}

func supervisionError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return helper.NotFound(
			"mahasiswa atau dosen pembimbing tidak ditemukan",
		)

	case errors.Is(err, repository.ErrSupervisorFull):
		return helper.Conflict("kuota pembimbing sudah penuh")

	case errors.Is(err, repository.ErrAlreadySupervised):
		return helper.Conflict(
			"mahasiswa sudah memiliki pembimbing atau tidak ada perubahan",
		)

	case errors.Is(err, repository.ErrNoSupervision):
		return helper.Conflict(
			"mahasiswa belum memiliki pembimbing",
		)

	case errors.Is(err, repository.ErrVersionConflict):
		return helper.Conflict(
			"data pembimbing berubah; muat ulang lalu coba lagi",
		)

	case errors.Is(err, repository.ErrQuotaBelowAssigned):
		return helper.Conflict(
			"kuota tidak boleh lebih kecil dari jumlah mahasiswa bimbingan",
		)

	default:
		return helper.Internal(err)
	}
}