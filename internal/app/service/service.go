package service

import (
	"errors"
	"fmt"
	"log"
	"pcxr/internal/app/models"
	"pcxr/internal/app/repository"
	"pcxr/pkg"
	"regexp"
	"time"

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
}

type service struct {
	repo repository.Repository
}

var (
	ErrInvalidData     = errors.New("invalid data")
	ErrInvalidExpires  = errors.New("token expired")
	ErrSessionNotFound = errors.New("session not found")
)

func NewService(repo repository.Repository) Service {
	return &service{repo: repo}
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
