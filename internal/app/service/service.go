package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"pcxr/internal/app/email"
	"pcxr/internal/app/models"
	"pcxr/internal/app/repository"
	"pcxr/pkg"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Service interface {
	RegisterUser(reg *models.Register_Model) error
	CartLoadsService(user_id int) ([]models.Cart_Config_Model, error)
	AddProductToCartService(user_id, product_id int) error
	RemoveProductFromCartService(user_id, product_id int) error
	CheckSessionTokenService(token string) (*models.Session, error)
	UpdateSessionExpiryService(token string) error
	LoginService(login *models.Login_Model) (string, error)
	DeleteSession(token string) error
	LoadCatalogTablesAuthorizedService(filter *models.FilterModel, userID, limit int) ([]models.Response_Tables_Authorized, error)
	LoadCatalogTablesGuestService(filter *models.FilterModel, limit int) ([]models.Response_Tables_Guest, error)
	LoadCatalogUnderframeGuestService(filter *models.FilterModel, limit int) ([]models.Response_Underframe_Guest, error)
	LoadCatalogUnderframeAuthorizedService(filter *models.FilterModel, userID, limit int) ([]models.Response_Underframe_Authorized, error)
	LoadProfileService(userID int) (*models.Response_Profile, error)
	RequestResetPassword(email string) error
	ConfrimResetPassword(token, newPassword string) error
}

type service struct {
	repo         repository.Repository
	email_sender email.SMTPSender
	pool         *pgxpool.Pool
}

var (
	ErrInvalidData     = errors.New("invalid data")
	ErrInvalidExpires  = errors.New("token expired")
	ErrSessionNotFound = errors.New("session not found")
	ErrTokenUsed       = errors.New("token already used")
)

func NewService(repo repository.Repository, email_sender email.SMTPSender, pool *pgxpool.Pool) Service {
	return &service{repo: repo, email_sender: email_sender, pool: pool}
}

func (s *service) RegisterUser(reg *models.Register_Model) error {
	if len(reg.Password) <= 8 {
		fmt.Println("Ошибка: длина пароля менее 8 символов")
		return ErrInvalidData
	}
	password_hash, err := bcrypt.GenerateFromPassword([]byte(reg.Password), 12)
	if err != nil {
		return err
	}
	firstNameRe := regexp.MustCompile(`^[a-zA-Zа-яА-ЯёЁ]+$`)
	lastNameRe := regexp.MustCompile(`^[a-zA-Zа-яА-ЯёЁ]+$`)
	patronymicRe := regexp.MustCompile(`^[a-zA-Zа-яА-ЯёЁ]+$`)
	emailRe := regexp.MustCompile(`[a-zA-Z0-9._-]+@[a-zA-Z0-9._-]+\.[a-zA-Z0-9_-]+`)
	phoneRe := regexp.MustCompile(`^[0-9+]+$`)
	if !firstNameRe.MatchString(reg.First_Name) {
		log.Println("Ошибка: неправильно введено имя")
		return ErrInvalidData
	}
	if !lastNameRe.MatchString(reg.Last_Name) {
		log.Println("Ошибка: неправильно введена фамилия")
		return ErrInvalidData
	}
	if !patronymicRe.MatchString(reg.Patronymic) {
		log.Println("Ошибка: неправильно введено имя")
		return ErrInvalidData
	}
	if !emailRe.MatchString(reg.Email) {
		log.Println("Ошибка: неправильная почта")
		return ErrInvalidData
	}
	if !phoneRe.MatchString(reg.Phone) && len(reg.Phone) > 12 {
		log.Println("Ошибка: неправильно введён телефон")
		return ErrInvalidData
	}
	reg.Password = string(password_hash)
	err = s.repo.CreateUser(reg)
	if err != nil {
		return err
	}
	return nil
}

func (s *service) CartLoadsService(user_id int) ([]models.Cart_Config_Model, error) {
	log.Printf("Loading cart for user_id: %d", user_id)

	load, err := s.repo.CartLoads(user_id)
	if err != nil {
		log.Printf("Repository error: %v", err)
		return nil, err
	}

	log.Printf("Loaded %d items", len(load))
	return load, nil
}

func (s *service) AddProductToCartService(user_id, product_id int) error {
	err := s.repo.AddProductToCart(user_id, product_id)
	if err != nil {
		return err
	}
	return nil
}

func (s *service) RemoveProductFromCartService(user_id, product_id int) error {
	err := s.repo.RemoveProductFromCart(user_id, product_id)
	if err != nil {
		return err
	}
	return nil
}

func (s *service) CheckSessionTokenService(token string) (*models.Session, error) {
	session, ttl, err := s.repo.CheckSessionRedis(token)
	if err == ErrSessionNotFound {
		session, err = s.repo.CheckSessionTokenDB(token)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	const minTTL = 120 * time.Hour
	const newTTL = 240 * time.Hour

	if ttl < minTTL {
		if err := s.repo.UpdateTTLRedis(token, newTTL); err != nil {
			log.Printf("failed to update ttl: %v", err)
			return nil, nil
		}
	}
	//if session.ttl
	/*if !session.Expires_At.After(time.Now().UTC()) {
		_, err = s.repo.DisableSession(token)
		if err != nil {
			return nil, err
		}
		return nil, ErrInvalidExpires
	} */
	return session, nil
}

func (s *service) UpdateSessionExpiryService(token string) error {
	newExpires := time.Now().Add(168 * time.Hour)
	return s.repo.UpdateSessionExpiry(token, newExpires)
}

func (s *service) LoginService(login *models.Login_Model) (string, error) {
	log.Printf("Login attempt for email: %s", login.Email)
	user, err := s.repo.LoginUser(login.Email)
	if err != nil {
		log.Printf("LoginUser error: %v", err)
		return "", err
	}
	log.Printf("User found: ID=%d", user.User_ID)
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(login.Password)); err != nil {
		log.Printf("Password comparison failed: %v", err)
		return "", ErrInvalidData
	}
	token, err := pkg.GenerateSecureToken()
	if err != nil {
		log.Printf("Token generation error: %v", err)
		return "", err
	}
	ttl := 240 * time.Hour
	session := &models.Session{
		//Token.token
		User_ID: user.User_ID,
		//Expires_At: time.Now().UTC(),
		Created_At: time.Now().UTC(),
		Is_Active:  true,
	}
	if err := s.repo.CreateSessionRedis(session, ttl, token); err != nil {
		log.Printf("Token generated, calling CreateSession for user %d", user.User_ID)
		return "", err
	}
	return token, err
}

func (s *service) DeleteSession(token string) error {
	if err := s.repo.DeleteSessionRedis(token); err != nil {
		return err
	}
	return nil
}

func (s *service) LoadCatalogTablesAuthorizedService(filter *models.FilterModel, userID, limit int) ([]models.Response_Tables_Authorized, error) {
	tables, err := s.repo.LoadCatalogTablesAuthorized(filter, userID, limit)
	if err != nil {
		return nil, err
	}
	return tables, nil
}

func (s *service) LoadCatalogTablesGuestService(filter *models.FilterModel, limit int) ([]models.Response_Tables_Guest, error) {
	tables, err := s.repo.LoadCatalogTablesGuest(filter, limit)
	if err != nil {
		return nil, err
	}
	return tables, nil
}

func (s *service) LoadCatalogUnderframeGuestService(filter *models.FilterModel, limit int) ([]models.Response_Underframe_Guest, error) {
	underframes, err := s.repo.LoadCatalogUnderframeGuest(filter, limit)
	if err != nil {
		return nil, err
	}
	return underframes, nil
}

func (s *service) LoadCatalogUnderframeAuthorizedService(filter *models.FilterModel, userID, limit int) ([]models.Response_Underframe_Authorized, error) {
	underframes, err := s.repo.LoadCatalogUnderframeAuthorized(filter, userID, limit)
	if err != nil {
		return nil, err
	}
	return underframes, nil
}

func (s *service) LoadProfileService(userID int) (*models.Response_Profile, error) {
	profile, err := s.repo.LoadProfile(userID)
	if err != nil {
		return nil, err
	}
	return profile, nil
}

func (s *service) RequestResetPassword(email string) error {
	user, err := s.repo.GetUserByEmail(email)
	if err != nil {
		return err
	}

	token, err := pkg.GenerateSecureToken()
	if err != nil {
		return fmt.Errorf("invalid generateSecureToken:%w", err)
	}
	tx, err := s.pool.Begin(context.Background())
	defer func() {
		if err != nil {
			tx.Rollback(context.Background())
		}
	}()
	expiresAt := time.Now().Add(15 * time.Minute)
	fmt.Printf("User found: ID=%v, Email=%s\n", user.ID, user.Email)
	if err := s.repo.CreateResetToken(tx, user.ID, token, expiresAt); err != nil {
		return fmt.Errorf("iternal CreateResetToken error: %w", err)
	}
	resetLink := os.Getenv("APP_URL") + "/reset_password/?token=" + token
	/*err = s.email_sender.SendPasswordRest(email, resetLink)
	if err != nil {
		log.Printf("SMTP error: %v", err)
	} */
	if err := tx.Commit(context.Background()); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	go s.email_sender.SendPasswordRest(email, resetLink)
	return nil
}

func (s *service) ConfrimResetPassword(token, newPassword string) error {
	if len(newPassword) < 6 {
		return errors.New("password too weak")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), 10)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(context.Background())
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback(context.Background())
		}
	}()
	rec, err := s.repo.FindValid(tx, token)
	if err != nil {
		return err
	}
	if err := s.repo.UpdatePassword(tx, string(hashed), rec.UserID); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	used, err := s.repo.MarkUsed(tx, token)
	if err != nil {
		return err
	}
	if !used {
		return errors.New("token already used")
	}

	if err := tx.Commit(context.Background()); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	err = s.repo.CleanExpiredResetTokens()
	if err != nil {
		return err
	}
	return nil
}
