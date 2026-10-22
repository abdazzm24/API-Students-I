package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api-students/app/repository"
	"api-students/app/service"
	"api-students/config"
	"api-students/database"
	"api-students/helper"
	"api-students/route"
)

const minSecretLength = 32

func main() {

	// ========================================================
	// 1. LOAD ENV
	// ========================================================

	config.LoadEnv()

	// ========================================================
	// 2. LOGGER
	// ========================================================

	logger := config.NewLogger()

	// ========================================================
	// 3. VALIDATE JWT SECRET
	// ========================================================

	jwtSecret := config.GetEnv(
		"JWT_SECRET",
		"",
	)

	if len(jwtSecret) < minSecretLength {

		logger.Error(
			"JWT_SECRET tidak diisi atau terlalu pendek",
			slog.Int(
				"minimal_karakter",
				minSecretLength,
			),
		)

		os.Exit(1)
	}

	// ========================================================
	// 4. DATABASE
	// ========================================================

	pool, err := database.NewPool(
		context.Background(),
	)

	if err != nil {

		logger.Error(
			"gagal terhubung ke database",
			slog.String(
				"error",
				err.Error(),
			),
		)

		os.Exit(1)
	}

	defer pool.Close()

	// ========================================================
	// 5. JWT MANAGER
	// ========================================================

	jwtManager := helper.NewJWTManager(
		jwtSecret,

		config.GetEnv(
			"JWT_ISSUER",
			"praktikum-backend",
		),

		time.Duration(
			config.GetEnvInt(
				"JWT_ACCESS_TTL_MINUTES",
				15,
			),
		) * time.Minute,
	)

	// ========================================================
	// 6. REPOSITORY
	// ========================================================

	studentRepository :=
		repository.NewStudentRepository(pool)

	userRepository :=
		repository.NewUserRepository(pool)

	tokenRepository :=
		repository.NewTokenRepository(pool)

	roleRepository :=
	repository.NewRoleRepository(pool)

	supervisionRepository :=
		repository.NewSupervisionRepository(pool)

	txManager :=
		repository.NewTxManager(pool)

	// ========================================================
	// 7. LOAD RBAC PERMISSIONS
	// ========================================================

	permissionsByRole, err :=
		roleRepository.LoadPermissions(
			context.Background(),
		)

	if err != nil {

		logger.Error(
			"gagal memuat permission RBAC",
			slog.String(
				"error",
				err.Error(),
			),
		)

		os.Exit(1)
	}

	permissions :=
		helper.NewPermissionSet(
			permissionsByRole,
		)

	logger.Info(
		"permission RBAC berhasil dimuat",
		slog.Any(
			"roles",
			permissions.KnownRoles(),
		),
	)

	// ========================================================
	// 8. SERVICE
	// ========================================================

	studentService :=
		service.NewStudentService(
			studentRepository,
			permissions,
		)

	userService :=
		service.NewUserService(
			userRepository,
			permissions,
		)

	authService :=
		service.NewAuthService(
			userRepository,
			tokenRepository,
			jwtManager,
			permissions,

			time.Duration(
				config.GetEnvInt(
					"JWT_REFRESH_TTL_DAYS",
					7,
				),
			) * 24 * time.Hour,
		)

	supervisionService :=
		service.NewSupervisionService(
			supervisionRepository,
			txManager,
		)

	// ========================================================
	// 9. APP
	// ========================================================

	app := config.NewApp(
		logger,
		route.Dependencies{
			Pool: pool,

			JWT: jwtManager,

			Permissions: permissions,

			StudentService:
				studentService,

			UserService:
				userService,

			AuthService:
				authService,

			SupervisionService:
				supervisionService,
		},
	)

	// ========================================================
	// 10. SERVER
	// ========================================================

	port := config.GetEnv(
		"APP_PORT",
		"3000",
	)

	go func() {

		if err := app.Listen(
			":" + port,
		); err != nil {

			logger.Error(
				"server berhenti",
				slog.String(
					"error",
					err.Error(),
				),
			)

			os.Exit(1)
		}
	}()

	logger.Info(
		"server berjalan",
		slog.String(
			"port",
			port,
		),
	)

	// ========================================================
	// 11. GRACEFUL SHUTDOWN
	// ========================================================

	quit := make(
		chan os.Signal,
		1,
	)

	signal.Notify(
		quit,
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	<-quit

	logger.Info(
		"sinyal berhenti diterima, menutup server",
	)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)

	defer cancel()

	if err := app.ShutdownWithContext(
		ctx,
	); err != nil {

		logger.Error(
			"gagal menutup server dengan rapi",
			slog.String(
				"error",
				err.Error(),
			),
		)
	}

	logger.Info(
		"server berhenti dengan rapi",
	)
}