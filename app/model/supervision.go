package model

type Supervisor struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Quota           int    `json:"quota"`
	SupervisedCount int    `json:"supervised_count"`
	Version         int    `json:"version"`
}

type AssignSupervisionRequest struct {
	StudentID int `json:"student_id" validate:"required,min=1"`
}

type SwitchSupervisionRequest struct {
	StudentID       int `json:"student_id" validate:"required,min=1"`
	NewSupervisorID int `json:"new_supervisor_id" validate:"required,min=1"`
}

type UpdateSupervisorRequest struct {
	Name    *string `json:"name,omitempty" validate:"omitnil,notblank,min=2,max=120"`
	Quota   int     `json:"quota" validate:"required,min=1,max=500"`
	Version int     `json:"version" validate:"required,min=1"`
}