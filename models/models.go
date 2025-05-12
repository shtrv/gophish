package models

import (
	"crypto/rand"
	"fmt"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"io"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/jinzhu/gorm"
	"github.com/shtrv/gophish/auth"
	"github.com/shtrv/gophish/config"
	log "github.com/shtrv/gophish/logger"
)

var db *gorm.DB
var conf *config.Config

const (
	MaxDatabaseConnectionAttempts = 10
	DefaultAdminUsername          = "admin"
	InitialAdminPassword          = "GOPHISH_INITIAL_ADMIN_PASSWORD"
	InitialAdminApiToken          = "GOPHISH_INITIAL_ADMIN_API_TOKEN"
)

const (
	CampaignInProgress string = "In progress"
	CampaignQueued     string = "Queued"
	CampaignCreated    string = "Created"
	CampaignEmailsSent string = "Emails Sent"
	CampaignComplete   string = "Completed"
	EventSent          string = "Email Sent"
	EventSendingError  string = "Error Sending Email"
	EventOpened        string = "Email Opened"
	EventClicked       string = "Clicked Link"
	EventDataSubmit    string = "Submitted Data"
	EventReported      string = "Email Reported"
	EventProxyRequest  string = "Proxied request"
	StatusSuccess      string = "Success"
	StatusQueued       string = "Queued"
	StatusSending      string = "Sending"
	StatusUnknown      string = "Unknown"
	StatusScheduled    string = "Scheduled"
	StatusRetry        string = "Retrying"
	Error              string = "Error"
)

// Flash is used to hold flash information for use in templates.
type Flash struct {
	Type    string
	Message string
}

// Response contains the attributes found in an API response
type Response struct {
	Message string      `json:"message"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

func generateSecureKey() string {
	k := make([]byte, 32)
	io.ReadFull(rand.Reader, k)
	return fmt.Sprintf("%x", k)
}

func createTemporaryPassword(u *User) error {
	var temporaryPassword string
	if envPassword := os.Getenv(InitialAdminPassword); envPassword != "" {
		temporaryPassword = envPassword
	} else {
		temporaryPassword = auth.GenerateSecureKey(auth.MinPasswordLength)
	}
	hash, err := auth.GeneratePasswordHash(temporaryPassword)
	if err != nil {
		return err
	}
	u.Hash = hash
	u.PasswordChangeRequired = true
	err = db.Save(u).Error
	if err != nil {
		return err
	}
	log.Infof("Please login with the username admin and the password %s", temporaryPassword)
	return nil
}

func Setup(c *config.Config) error {
	conf = c

	if conf.DBSSLCaPath != "" {
		switch conf.DBName {
		case "mysql":
			log.Warn("DBSSLCaPath not implemented")
			//rootCertPool := x509.NewCertPool()
			//pem, err := ioutil.ReadFile(conf.DBSSLCaPath)
			//if err != nil {
			//	log.Error(err)
			//	return err
			//}
			//if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
			//	log.Error("Failed to append PEM.")
			//	return err
			//}
			//
		}
	}

	var err error
	i := 0
	for {
		db, err = gorm.Open(conf.DBName, conf.DBPath)
		if err == nil {
			break
		}
		if i >= MaxDatabaseConnectionAttempts {
			log.Error(err)
			return err
		}
		i++
		log.Warn("waiting for database to be up...")
		time.Sleep(5 * time.Second)
	}
	db.LogMode(false)
	db.SetLogger(log.Logger)
	db.DB().SetMaxOpenConns(1)

	// Run migrations using golang-migrate
	sqlDB := db.DB()
	var m *migrate.Migrate

	switch conf.DBName {
	case "mysql":
		driver, err := mysql.WithInstance(sqlDB, &mysql.Config{})
		if err != nil {
			log.Error(err)
			return err
		}
		m, err = migrate.NewWithDatabaseInstance(
			"file://"+conf.MigrationsPath,
			"mysql",
			driver,
		)
	case "sqlite3":
		driver, err := sqlite3.WithInstance(sqlDB, &sqlite3.Config{})
		if err != nil {
			log.Error(err)
			return err
		}
		m, err = migrate.NewWithDatabaseInstance(
			"file://"+conf.MigrationsPath,
			"sqlite3",
			driver,
		)
	default:
		return fmt.Errorf("unsupported database type: %s", conf.DBName)
	}
	if err != nil {
		log.Error(err)
		return err
	}

	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		log.Error(err)
		return err
	}

	// Setup default admin user
	var userCount int64
	var adminUser User
	db.Model(&User{}).Count(&userCount)

	adminRole, err := GetRoleBySlug(RoleAdmin)
	if err != nil {
		log.Error(err)
		return err
	}

	if userCount == 0 {
		adminUser = User{
			Username:               DefaultAdminUsername,
			Role:                   adminRole,
			RoleID:                 adminRole.ID,
			PasswordChangeRequired: true,
		}
		if envToken := os.Getenv(InitialAdminApiToken); envToken != "" {
			adminUser.ApiKey = envToken
		} else {
			adminUser.ApiKey = auth.GenerateSecureKey(auth.APIKeyLength)
		}
		err = db.Save(&adminUser).Error
		if err != nil {
			log.Error(err)
			return err
		}
	} else {
		adminUser, err = GetUserByUsername(DefaultAdminUsername)
		if err != nil {
			log.Error(err)
			return err
		}
	}

	if adminUser.PasswordChangeRequired {
		err = createTemporaryPassword(&adminUser)
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return nil
}
