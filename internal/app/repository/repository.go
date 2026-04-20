package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"pcxr/internal/app/logger"
	"pcxr/internal/app/models"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type repository_struct struct {
	db  *pgxpool.Pool
	red *redis.Client
}

type Repository interface {
	GetUserByID(user_id int) (*models.User, error)
	CreateUser(reg *models.Register_Model) (err error)
	CartLoads(user_id int) ([]models.Cart_Config_Model, error)
	AddProductToCart(user_id, product_id int) error
	RemoveProductFromCart(user_id, product_id int) error
	CheckSessionTokenDB(token string) (*models.Session, error)
	UpdateSessionExpiry(token string, expiresAt time.Time) error
	LoginUser(email string) (*models.Login_User_Model, error)
	CreateSessionDB(token string, userID int) (string, error)
	DisableSession(token string) (*models.Session, error)
	CheckSessionRedis(token string) (*models.Session, time.Duration, error)
	CreateSessionRedis(session *models.Session, ttl time.Duration) error
	UpdateTTLRedis(token string, ttl time.Duration) error
	DeleteSessionRedis(token string) error
}

func NewRepository(db *pgxpool.Pool, red *redis.Client) Repository {
	return &repository_struct{db: db, red: red}
}

var (
	UserExist          = errors.New("this user already exists")
	ErrSessionNotFound = errors.New("session not found")
)

func (s *repository_struct) GetUserByID(id int) (*models.User, error) {
	user := new(models.User)
	err := s.db.QueryRow(context.Background(),
		`SELECT id, email, password, first_name, last_name, patronymic, phone, photo, created_at
	FROM users
	WHERE id = $1`, id).Scan(
		&user.ID,
		&user.Email,
		&user.Password,
		&user.First_name,
		&user.Last_name,
		&user.Patronymic,
		&user.Phone,
		&user.Photo,
		&user.Created_at,
	)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *repository_struct) CreateUser(reg *models.Register_Model) (err error) {
	ctx := context.Background()
	tx, err := s.db.Begin(ctx)
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()
	var userID int
	err = tx.QueryRow(ctx, `INSERT INTO users (email, password, first_name, last_name, patronymic, phone, photo, created_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING id`,
		reg.Email,
		reg.Password,
		reg.First_Name,
		reg.Last_Name,
		reg.Patronymic,
		reg.Phone,
		reg.Photo,
		time.Now().UTC(),
	).Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return UserExist
		}
		log.Printf("Scan error: %v", err)
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO cart (user_id) VALUES ($1)`, userID)
	if err != nil {
		return err
	}
	err = tx.Commit(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *repository_struct) CartLoads(user_id int) ([]models.Cart_Config_Model, error) {
	var cart []models.Cart_Config_Model
	rows, err := s.db.Query(context.Background(),
		`SELECT
    cc.id AS id,
    cc.quantity,
    COALESCE(t.name, tt.name, uf.name) AS product_name,
    COALESCE(t.photo, tt.photo, uf.photo) AS product_photo,
    COALESCE(t.description, tt.description, uf.description) AS product_description,
    COALESCE(t.price, tt.price, uf.price) AS product_price,
    pcat.name AS category_name,
    CASE 
        WHEN p.tables_id IS NOT NULL AND p.tables_id != 0 THEN 'table'
        WHEN p.tabletop_id IS NOT NULL AND p.tabletop_id != 0 THEN 'tabletop'
        WHEN p.underframe_id IS NOT NULL AND p.underframe_id != 0 THEN 'underframe'
        ELSE 'unknown'
    END AS product_type,
    t.id AS table_id,
    tt.id AS tabletop_id,
    uf.id AS underframe_id
	FROM cart_config cc
	JOIN cart c ON cc.cart_id = c.id
	JOIN products p ON cc.product_id = p.id
	JOIN category pcat ON p.products_category_id = pcat.id
	LEFT JOIN tables t ON p.tables_id = t.id
	LEFT JOIN tabletop tt ON p.tabletop_id = tt.id
	LEFT JOIN underframe uf ON p.underframe_id = uf.id
	WHERE c.user_id = $1 
	ORDER BY cc.id ASC;`, user_id)
	if err != nil {
		return nil, fmt.Errorf("db query error: %w", err)
	}
	for rows.Next() {
		var item models.Cart_Config_Model
		err := rows.Scan(
			&item.ID,
			&item.Quantity,
			&item.Product_Name,
			&item.Product_Photo,
			&item.Product_Description,
			&item.Product_Price,
			&item.Category_Name,
			&item.Product_Type,
			&item.Table_ID,
			&item.Tabletop_ID,
			&item.Underframe_ID,
		)
		if err != nil {
			log.Printf("Scan error: %v", err)
			return nil, fmt.Errorf("scan error: %w", err)
		}
		cart = append(cart, item)
	}
	defer rows.Close()
	return cart, nil
}
func (s *repository_struct) AddProductToCart(user_id, product_id int) error {
	ctx := context.Background()
	tx, err := s.db.Begin(ctx)
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()
	_, err = tx.Exec(ctx,
		`WITH user_cart AS (SELECT id FROM cart WHERE user_id = $1)
    INSERT INTO cart_config (cart_id, product_id, quantity)
    SELECT id, $2, COALESCE($3, 1)
    FROM user_cart
    ON CONFLICT (cart_id, product_id) DO UPDATE
    SET quantity = cart_config.quantity + 1`,
		user_id, product_id, 1)
	if err != nil {
		return err
	}
	err = tx.Commit(ctx)
	return nil
}
func (s *repository_struct) RemoveProductFromCart(user_id, product_id int) error {
	ctx := context.Background()
	tx, err := s.db.Begin(ctx)
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()
	_, err = tx.Exec(ctx,
		`DELETE FROM cart_config cc
		USING cart c
		WHERE cc.cart_id = c.id
			AND c.user_id = $1
			AND cc.product_id = $2
			AND cc.quantity <= 1`,
		user_id,
		product_id)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`UPDATE cart_config cc
		SET quantity = cc.quantity - 1
		FROM cart c
		WHERE cc.cart_id = c.id
			AND c.user_id = $1
			AND cc.product_id = $2
			AND cc.quantity >= 1`,
		user_id,
		product_id)
	if err != nil {
		return err
	}
	err = tx.Commit(ctx)
	return nil
}

func (s *repository_struct) CheckSessionTokenDB(token string) (*models.Session, error) {
	session := new(models.Session)
	err := s.db.QueryRow(context.Background(),
		`SELECT user_id, expires_at
	FROM session WHERE token = $1 AND is_active = true`, token).Scan(
		&session.User_ID,
		&session.Expires_At)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("сессия не найдена")
		}
		fmt.Println(err)
		return nil, err
	}
	return session, nil
}

func (r *repository_struct) CheckSessionRedis(token string) (*models.Session, time.Duration, error) {
	session := new(models.Session)
	key := fmt.Sprintf("session:%s", token)
	fmt.Printf("🔑 Redis key: %s\n", key)
	data, err := r.red.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return nil, 0, ErrSessionNotFound
	}
	if err != nil {
		return nil, 0, fmt.Errorf("redis error: %w", err)
	}
	ttl, err := r.red.TTL(context.Background(), key).Result()
	if err != nil {
		return nil, 0, err
	}
	if err := json.Unmarshal([]byte(data), session); err != nil {
		return nil, 0, fmt.Errorf("unmarshal error: %w", err)
	}
	return session, ttl, err
}

func (s *repository_struct) UpdateSessionExpiry(token string, expiresAt time.Time) error {
	_, err := s.db.Exec(context.Background(),
		`UPDATE session SET expires_at = $1 WHERE token = $2`,
		expiresAt, token,
	)
	return err
}

func (r *repository_struct) UpdateTTLRedis(token string, ttl time.Duration) error {
	key := fmt.Sprintf("session:%s", token)
	return r.red.Expire(context.Background(), key, ttl).Err()
}

func (s *repository_struct) CreateSessionDB(token string, userID int) (string, error) {
	ctx := context.Background()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()
	_, err = tx.Exec(ctx,
		`WITH 
	active_session AS (
        SELECT token FROM session WHERE user_id = $2 AND is_active = TRUE LIMIT 1),
    updated_active AS (
        UPDATE session
        SET token = $1, expires_at = NOW() + INTERVAL '7 days', updated_at = NOW(), is_active = TRUE
        WHERE token = (SELECT token FROM active_session)
        RETURNING token),
    inactive_session AS (
        SELECT token FROM session WHERE is_active = FALSE LIMIT 1 FOR UPDATE SKIP LOCKED),
    updated_inactive AS (
        UPDATE session
        SET token = $1, user_id = $2, expires_at = NOW() + INTERVAL '7 days', updated_at = NOW(), is_active = TRUE
        WHERE token = (SELECT token FROM inactive_session)
        AND NOT EXISTS (SELECT 1 FROM updated_active)
        RETURNING token)
    INSERT INTO session (token, user_id, expires_at, created_at, is_active)
    SELECT $1, $2, NOW() + INTERVAL '7 days', NOW(), TRUE
    WHERE NOT EXISTS (SELECT 1 FROM updated_active)
    AND NOT EXISTS (SELECT 1 FROM updated_inactive)`, token, userID)
	if err != nil {
		return "", err
	}
	err = tx.Commit(ctx)
	if err != nil {
		return "", err
	}
	return "", nil
}

func (r *repository_struct) CreateSessionRedis(session *models.Session, ttl time.Duration) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	key := fmt.Sprintf("session:%s", session.Token)
	fmt.Printf("🔑 Redis key: %s\n", key)
	if err := r.red.Set(context.Background(), key, data, ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	return nil
}

func (s *repository_struct) DisableSession(token string) (*models.Session, error) {
	_, err := s.db.Exec(context.Background(),
		`UPDATE session SET is_active = $2 WHERE token = $1`, token, "false")
	if err != nil {
		return nil, err
	}
	return nil, err
}

func (s *repository_struct) LoginUser(email string) (*models.Login_User_Model, error) {
	var login models.Login_User_Model
	if err := s.db.QueryRow(context.Background(),
		`SELECT id, email, password
	FROM users WHERE email = $1`, email).Scan(
		&login.User_ID,
		&login.Email,
		&login.Password); err != nil {
		logger.Log.Info(err.Error())
		return nil, err
	}
	return &login, nil
}

func (r *repository_struct) DeleteSessionRedis(token string) error {
	key := fmt.Sprintf("session:%s", token)
	return r.red.Del(context.Background(), key).Err()
}
