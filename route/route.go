package route

import (
	"api-students/app/service"
	"api-students/helper"
	"api-students/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Dependencies struct {
	Pool *pgxpool.Pool

	JWT *helper.JWTManager

	Permissions *helper.PermissionSet

	StudentService *service.StudentService

	UserService *service.UserService

	AuthService *service.AuthService

	SupervisionService *service.SupervisionService
}

func Register(
	app *fiber.App,
	deps Dependencies,
) {

	api := app.Group(
		"/api/v1",
	)

	// ========================================================
	// PUBLIC
	// ========================================================

	api.Get(
		"/health",
		func(c *fiber.Ctx) error {

			if err := deps.Pool.Ping(
				c.UserContext(),
			); err != nil {

				return helper.ServiceUnavailable("database tidak dapat dihubungi")
			}

			return helper.Success(
				c,
				fiber.StatusOK,
				"server dan database berjalan",
				fiber.Map{
					"status": "ok",
				},
			)
		},
	)

	// ========================================================
	// AUTHENTICATION
	// ========================================================

	auth := api.Group(
		"/auth",
		middleware.RequireJSON,
	)

	auth.Post(
		"/register",
		deps.AuthService.Register,
	)

	auth.Post(
		"/login",
		middleware.LoginRateLimiter(),
		deps.AuthService.Login,
	)

	auth.Post(
		"/refresh",
		deps.AuthService.Refresh,
	)

	auth.Post(
		"/logout",
		deps.AuthService.Logout,
	)

	auth.Get(
		"/me",
		middleware.RequireAuth(deps.JWT),
		deps.AuthService.Me,
	)

	// ========================================================
	// USERS
	// ========================================================

	users := api.Group(
		"/users",
		middleware.RequireJSON,
		middleware.RequireAuth(deps.JWT),
	)

	// Bisa diputuskan langsung berdasarkan permission.
	users.Get(
		"/",
		middleware.RequirePermission(
			deps.Permissions,
			"user:list",
		),
		deps.UserService.List,
	)

	users.Post(
		"/",
		middleware.RequirePermission(
			deps.Permissions,
			"user:update:any",
		),
		deps.UserService.Create,
	)

	users.Delete(
		"/:id",
		middleware.RequirePermission(
			deps.Permissions,
			"user:delete",
		),
		deps.UserService.Delete,
	)

	users.Patch(
		"/:id/role",
		middleware.RequirePermission(
			deps.Permissions,
			"role:assign",
		),
		deps.UserService.AssignRole,
	)

	// Ownership diperiksa di service.
	users.Get(
		"/:id",
		deps.UserService.Get,
	)

	users.Put(
		"/:id",
		deps.UserService.Replace,
	)

	users.Patch(
		"/:id",
		deps.UserService.Patch,
	)

	// ========================================================
	// STUDENTS
	// ========================================================

	students := api.Group(
		"/students",
		middleware.RequireAuth(deps.JWT),
	)

	// Permission dapat diputuskan tanpa membaca data.
	students.Get(
		"/",
		middleware.RequirePermission(
			deps.Permissions,
			"student:list",
		),
		deps.StudentService.List,
	)

	students.Post(
		"/",
		middleware.RequireJSON,
		middleware.RequirePermission(
			deps.Permissions,
			"student:create",
		),
		deps.StudentService.Create,
	)

	students.Delete(
		"/:id",
		middleware.RequirePermission(
			deps.Permissions,
			"student:delete",
		),
		deps.StudentService.Delete,
	)

	// Ownership diperiksa di service.
	students.Get(
		"/:id",
		deps.StudentService.Get,
	)

	students.Put(
		"/:id",
		middleware.RequireJSON,
		deps.StudentService.Replace,
	)

	students.Patch(
		"/:id",
		middleware.RequireJSON,
		deps.StudentService.Patch,
	)

	supervisors := api.Group(
		"/supervisors",
		middleware.RequireAuth(deps.JWT),
	)

	supervisors.Get(
		"/",
		middleware.RequirePermission(
			deps.Permissions,
			"supervision:manage",
		),
		deps.SupervisionService.List,
	)

	supervisors.Post(
		"/:id/assign",
		middleware.RequireJSON,
		middleware.RequirePermission(
			deps.Permissions,
			"supervision:manage",
		),
		deps.SupervisionService.Assign,
	)

	supervisors.Patch(
		"/:id",
		middleware.RequireJSON,
		middleware.RequirePermission(
			deps.Permissions,
			"supervision:manage",
		),
		deps.SupervisionService.Update,
	)

	supervisions := api.Group(
		"/supervisions",
		middleware.RequireAuth(deps.JWT),
		middleware.RequireJSON,
		middleware.RequirePermission(
			deps.Permissions,
			"supervision:manage",
		),
	)

	supervisions.Post(
		"/switch",
		deps.SupervisionService.Switch,
	)
}