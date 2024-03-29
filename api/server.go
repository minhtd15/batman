package api

import (
	"context"
	batman "education-website"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	log "github.com/sirupsen/logrus"
	"go.elastic.co/apm/module/apmlogrus"
	"gopkg.in/yaml.v3"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	XApiKeyHeader string = "X-Api-Key"
)

var (
	userService  batman.UserService
	authService  batman.AuthService
	jwtService   batman.JwtService
	classService batman.ClassService
)

func GetLoggerWithContext(ctx context.Context) *log.Entry {
	log := log.WithFields(apmlogrus.TraceContext(ctx))
	return log
}

// Config struct for webapp config
type Config struct {
	Server struct {
		// Host is the local machine IP Address to bind the HTTP Server to
		Host string `yaml:"host"`
		// Port is the local machine TCP Port to bind the HTTP Server to
		Port    string `yaml:"port"`
		Timeout struct {
			// Server is the general server timeout to use
			// for graceful shutdowns
			Server time.Duration `yaml:"server"`

			// Write is the amount of time to wait until an HTTP server
			// write opperation is cancelled
			Write time.Duration `yaml:"write"`

			// Read is the amount of time to wait until an HTTP server
			// read operation is cancelled
			Read time.Duration `yaml:"read"`

			// Read is the amount of time to wait
			// until an IDLE HTTP session is closed
			Idle time.Duration `yaml:"idle"`
		} `yaml:"timeout"`
	} `yaml:"server"`

	Database struct {
		Port     string `yaml:"port"`
		Host     string `yaml:"host"`
		User     string `yaml:"user"`
		Password string `yaml:"pass"`
		DbName   string `yaml:"dbName"`
	} `yaml:"database"`

	XApiKey string `yaml:"XApiKey"`

	UserService  batman.UserService
	JwtService   batman.JwtService
	AuthService  batman.AuthService
	ClassService batman.ClassService
}

// NewConfig returns a new decoded Config struct
func NewConfig(configPath string) (*Config, error) {
	// Create config structure
	config := &Config{}

	// Open config file
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}

// ValidateConfigPath just makes sure, that the path provided is a file,
// that can be read
func ValidateConfigPath(path string) error {
	s, err := os.Stat(path)
	if err != nil {
		return err
	}
	if s.IsDir() {
		return fmt.Errorf("'%s' is a directory, not a normal file", path)
	}
	return nil
}

// ParseFlags will create and parse the CLI flags
// and return the path to be used elsewhere
func ParseFlags() (string, error) {
	// String that contains the configured configuration path
	configFile := os.Getenv("CONFIG_FILE_PATH")
	// Use configFile in your application

	// Set up a CLI flag called "-config" to allow users
	// to supply the configuration file
	flag.StringVar(&configFile, "config", "conf/config.yml", "path to config file")

	// Actually parse the flags
	flag.Parse()

	// Validate the path first
	if err := ValidateConfigPath(configFile); err != nil {
		log.Infof("error validating config path: %v", err)
		return "", err
	}

	// Return the configuration path
	return configFile, nil
}

// NewRouter generates the router used in the HTTP Server
func NewRouter(config Config) http.Handler {
	// Create router and define routes and return that router
	r := mux.NewRouter()
	r.Use(AuthMiddleware())

	// APIs that require token
	internalRouter := r.PathPrefix("/i/v1").Subrouter()
	internalRouter.HandleFunc("/user-verification", handlerUserAccount).Methods(http.MethodPost)
	internalRouter.HandleFunc("/change-password", handleChangePassword).Methods(http.MethodPut)
	internalRouter.HandleFunc("/salary-info", handlerSalaryInformation).Methods(http.MethodGet)
	internalRouter.HandleFunc("/modify-salary-configuration", handleModifySalaryConfiguration).Methods(http.MethodPut)
	internalRouter.HandleFunc("/new-course", handleInsertNewClass).Methods(http.MethodPost)
	internalRouter.HandleFunc("/insert-students", handleInsertStudents).Methods(http.MethodPost)
	internalRouter.HandleFunc("/user-schedule", handleClassFromToDateById).Methods(http.MethodGet)
	internalRouter.HandleFunc("/modify-user-info", handleModifyUserInformation).Methods(http.MethodPut)
	internalRouter.HandleFunc("/check-in-class", handleCheckInAttendanceClass).Methods(http.MethodPost)
	internalRouter.HandleFunc("/add-student", handleInsertOneNewStudent).Methods(http.MethodPost)
	internalRouter.HandleFunc("/sort-role", handleGetUserByRole).Methods(http.MethodGet)
	internalRouter.HandleFunc("/delete-course", handleDeleteCourse).Methods(http.MethodDelete)
	//internalRouter.HandleFunc("/delete-user", handleDeleteUser).Methods(http.MethodDelete)
	internalRouter.HandleFunc("/delete-class", handleDeleteSessionByClassIs).Methods(http.MethodDelete)
	internalRouter.HandleFunc("/my-schedule", handleGetMySchedule).Methods(http.MethodGet)
	internalRouter.HandleFunc("/fix-course-information", handleFixCourseInformation).Methods(http.MethodPut)
	internalRouter.HandleFunc("/note", AddNoteByClassId).Methods(http.MethodPost)
	internalRouter.HandleFunc("/check-in-history", handleGetCheckInWorkerHistory).Methods(http.MethodGet)
	internalRouter.HandleFunc("/delete-student", handleDeleteStudent).Methods(http.MethodDelete)
	internalRouter.HandleFunc("/fix-student-info", handleFixStudentInformation).Methods(http.MethodPut)
	internalRouter.HandleFunc("/add-user-by-position", handleAddUserByPosition).Methods(http.MethodPost)
	internalRouter.HandleFunc("/add-sub-class", handlePostSubClass).Methods(http.MethodPost)
	internalRouter.HandleFunc("/delete-sub-class", handleDeleteSubClass).Methods(http.MethodDelete)

	// APIs that does not require token
	externalRouter := r.PathPrefix("/e/v1").Subrouter()
	externalRouter.HandleFunc("/login", handlerLoginUser).Methods(http.MethodPost)
	externalRouter.HandleFunc("/register", handlerRegisterUser).Methods(http.MethodPost)
	externalRouter.HandleFunc("/excel-export", handleExcelSalary).Methods(http.MethodPost)
	externalRouter.HandleFunc("/class-info", handleGetClassInformation).Methods(http.MethodGet)
	externalRouter.HandleFunc("/all-courses", handleGetAllCourseInformation).Methods(http.MethodGet)
	//internalRouter.HandleFunc("/class-information", handleGetClassInformation).Methods(http.MethodGet)
	externalRouter.HandleFunc("/send-email", handleEmailJobs).Methods(http.MethodPost)
	//externalRouter.HandleFunc("/send-daily-email", handleSendDailyEmail).Methods(http.MethodGet)
	externalRouter.HandleFunc("/student-check-in", checkInStudentAttendance).Methods(http.MethodGet)
	externalRouter.HandleFunc("/students", getStudentsByCourse).Methods(http.MethodGet)
	externalRouter.HandleFunc("/check-attendance-student", handlePostStudentAttendance).Methods(http.MethodPost)
	externalRouter.HandleFunc("/fix-attendance-status", updateStudentAttendanceStatus).Methods(http.MethodPut)
	externalRouter.HandleFunc("/course-sessions", handleGetAllSessionsByCourseId).Methods(http.MethodGet)
	externalRouter.HandleFunc("/forgot-password", handleSendEmailForgotPassword).Methods(http.MethodPost)
	externalRouter.HandleFunc("/send-digit", handleCheckDigitCodeForgotPassword).Methods(http.MethodPost)
	externalRouter.HandleFunc("/new-password", handleSetNewPassword).Methods(http.MethodPost)
	externalRouter.HandleFunc("/get-sub-class", handleGetSubClass).Methods(http.MethodGet)

	// Serving static files from the "./static" directory
	r.PathPrefix("/web/").Handler(http.StripPrefix("/web/", http.FileServer(http.Dir("dist"))))

	// Set up cron job to run sendDailyEmail at 7 AM daily
	//con := cron.New()
	//con.AddFunc("0 7 * * *", user.SendDailyEmail)
	//con.Start()

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"http://localhost:8081"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders: []string{"*"},
	})

	handler := c.Handler(r)
	return handler
}

// Run will run the HTTP Server
func (config Config) Run() {
	// Set up a channel to listen to for interrupt signals
	var runChan = make(chan os.Signal, 1)

	// Set up a context to allow for graceful server shutdowns in the event
	// of an OS interrupt (defers the cancel just in case)
	ctx, cancel := context.WithTimeout(
		context.Background(),
		config.Server.Timeout.Server,
	)
	defer cancel()

	// Define server options
	server := &http.Server{
		Addr:         config.Server.Host + ":" + config.Server.Port,
		Handler:      NewRouter(config),
		ReadTimeout:  config.Server.Timeout.Read * time.Second,
		WriteTimeout: config.Server.Timeout.Write * time.Second,
		IdleTimeout:  config.Server.Timeout.Idle * time.Second,
	}

	// Handle ctrl+c/ctrl+x interrupt
	signal.Notify(runChan, os.Interrupt, syscall.SIGTSTP)

	// Alert the user that the server is starting
	log.Printf("Server is starting on %s\n", server.Addr)

	// Run the server on a new goroutine
	go func() {
		log.Infof("Before Listen and Serve")
		if err := server.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				// Normal interrupt operation, ignore
			} else {
				log.Fatalf("Server failed to start due to err: %v", err)
			}
		}
	}()

	log.Printf("After ListenAndServe")
	// Block on this channel listeninf for those previously defined syscalls assign
	// to variable so we can let the user know why the server is shutting down
	interrupt := <-runChan

	// If we get one of the pre-prescribed syscalls, gracefully terminate the server
	// while alerting the user
	log.Printf("Server is shutting down due to %+v\n", interrupt)
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server was unable to gracefully shutdown due to err: %+v", err)
	}
}

func respondWithJSON(w http.ResponseWriter, httpStatusCode int, data interface{}) {
	resp, err := json.Marshal(data)
	if err != nil {
		log.WithError(err).WithField("data", data).Error("failed to marshal data")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatusCode)
	w.Write(resp)
	return
}

func Init(c Config) {
	userService = c.UserService
	authService = c.AuthService
	jwtService = c.JwtService
	classService = c.ClassService
}
