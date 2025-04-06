package service

import (
	"net/http"
	"postal-system/internal/config"
	"postal-system/internal/handlers"
	"postal-system/internal/middleware"
	"postal-system/internal/repository"

	gohandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PostalService struct {
	db           *gorm.DB
	config       config.Config
	router       *mux.Router
	userRepo     repository.UserRepository
	logRepo      repository.LogRepository
	packageRepo  repository.PackageRepository
	trackingRepo repository.TrackingRepository
	officeRepo   repository.PostOfficeRepository
	tokenManager *middleware.TokenManager
}

func NewPostalService(cfg config.Config) (*PostalService, error) {
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	userRepo := repository.NewUserRepository(db)
	packageRepo := repository.NewPackageRepository(db)
	trackingRepo := repository.NewTrackingRepository(db)
	officeRepo := repository.NewPostOfficeRepository(db)

	router := mux.NewRouter()

	service := &PostalService{
		db:           db,
		config:       cfg,
		router:       router,
		userRepo:     userRepo,
		packageRepo:  packageRepo,
		trackingRepo: trackingRepo,
		officeRepo:   officeRepo,
	}

	service.SetupRoutes()

	return service, nil
}

func (p *PostalService) SetupRoutes() {
	corsHandler := gohandlers.CORS(
		gohandlers.AllowedOrigins([]string{"*"}),
		gohandlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		gohandlers.AllowedHeaders([]string{"Content-Type", "X-Requested-With", "Accept", "Accept-Language", "Content-Language", "Origin", "Authorization"}),
		gohandlers.AllowCredentials(),
	)

	authHandler := handlers.NewAuthHandler(p.userRepo, p.logRepo, p.tokenManager)

	profileHandler := handlers.NewProfileHandler(p.userRepo)
	logHandler := handlers.NewLogHandler(p.userRepo, p.logRepo)

	packageHandler := handlers.NewPackageHandler(p.packageRepo, p.trackingRepo, p.userRepo)
	trackingHandler := handlers.NewTrackingHandler(p.trackingRepo, p.packageRepo)
	officeHandler := handlers.NewPostOfficeHandler(p.officeRepo)

	authMiddleware := middleware.NewAuthMiddleware(p.tokenManager)

	p.router.HandleFunc("/api/register", authHandler.Register).Methods("POST")
	p.router.HandleFunc("/api/login", authHandler.Login).Methods("POST")
	p.router.HandleFunc("/api/refresh", authHandler.RefreshToken).Methods("POST")
	p.router.HandleFunc("/api/verify", authHandler.VerifyToken).Methods("GET")

	p.router.HandleFunc("/api/track/{trackingNumber}", trackingHandler.TrackPackage).Methods("GET")
	p.router.HandleFunc("/api/offices", officeHandler.GetAllOffices).Methods("GET")
	p.router.HandleFunc("/api/offices/{id}", officeHandler.GetOfficeById).Methods("GET")

	protected := p.router.PathPrefix("/api/protected").Subrouter()
	protected.Use(corsHandler, authMiddleware.Authenticate)

	protected.HandleFunc("/profile", profileHandler.GetProfile).Methods("GET")
	protected.HandleFunc("/profile", profileHandler.UpdateProfile).Methods("PUT")
	protected.HandleFunc("/logs", logHandler.GetUserLogs).Methods("GET")

	protected.HandleFunc("/packages", packageHandler.GetUserPackages).Methods("GET")
	protected.HandleFunc("/packages", packageHandler.CreatePackage).Methods("POST")
	protected.HandleFunc("/packages/{id}", packageHandler.GetPackageById).Methods("GET")
	protected.HandleFunc("/packages/{id}", packageHandler.UpdatePackage).Methods("PUT")

	// Маршруты для сотрудников почты (требуют роль EMPLOYEE или ADMIN)
	employee := p.router.PathPrefix("/api/employee").Subrouter()
	employee.Use(corsHandler, authMiddleware.Authenticate, authMiddleware.RequireRole("EMPLOYEE"))

	employee.HandleFunc("/packages", packageHandler.GetAllPackages).Methods("GET")
	employee.HandleFunc("/packages/{id}/status", packageHandler.UpdateStatus).Methods("PUT")
	employee.HandleFunc("/tracking", trackingHandler.CreateTrackingEvent).Methods("POST")

	// Маршруты для администраторов
	admin := p.router.PathPrefix("/api/admin").Subrouter()
	admin.Use(corsHandler, authMiddleware.Authenticate, authMiddleware.RequireRole("ADMIN"))

	admin.HandleFunc("/offices", officeHandler.CreateOffice).Methods("POST")
	admin.HandleFunc("/offices/{id}", officeHandler.UpdateOffice).Methods("PUT")
	admin.HandleFunc("/offices/{id}", officeHandler.DeleteOffice).Methods("DELETE")
	admin.HandleFunc("/users", profileHandler.GetAllUsers).Methods("GET")
	admin.HandleFunc("/users/{id}/role", profileHandler.UpdateUserRole).Methods("PUT")
}

func (p *PostalService) Start() error {
	return http.ListenAndServe(p.config.ServerAddress, p.router)
}

func (p *PostalService) GetRouter() *mux.Router {
	return p.router
}
