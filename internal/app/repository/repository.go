package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"pcxr/internal/app/logger"
	"pcxr/internal/app/models"
	"pcxr/pkg"
	"strings"
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
	GetUserByEmail(email string) (*models.User, error)
	UpdatePassword(tx pgx.Tx, password string, userID int) error
	UpdatePasswordProfile(password string, userID int) error
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
	CreateSessionRedis(session *models.Session, ttl time.Duration, token string) error
	UpdateTTLRedis(token string, ttl time.Duration) error
	DeleteSessionRedis(token string) error
	LoadCatalogTablesAuthorized(filter *models.FilterModel, userID, limit int) ([]models.Response_Tables_Authorized, error)
	LoadCatalogTablesGuest(filter *models.FilterModel, limit int) ([]models.Response_Tables_Guest, error)
	LoadCatalogUnderframeAuthorized(filter *models.FilterModel, userID, limit int) ([]models.Response_Underframe_Authorized, error)
	LoadCatalogUnderframeGuest(filter *models.FilterModel, limit int) ([]models.Response_Underframe_Guest, error)
	LoadProfile(userID int) (*models.Response_Profile, error)
	CreateResetToken(tx pgx.Tx, userID int, token string, expiresAt time.Time) error
	FindValid(tx pgx.Tx, token string) (*models.Reset_Token, error)
	MarkUsed(tx pgx.Tx, token string) (bool, error)
	CleanExpiredResetTokens() error
	GetAllPickUpPoints(userID int) ([]models.PickUpPoint_Model, error)
	SavePickUpPoint(userID, pickupID int) error
	ChangeUserData(data *models.ChangeUserData, userID int) error
	GetAllOrders(userID int) ([]models.OrderRequest, error)
	GetInfoOrder(userID int, orderToken string) ([]models.OrderInfoRequest, error)
	AddOrder(userID, point_id int, orderToken string) error
}

func NewRepository(db *pgxpool.Pool, red *redis.Client) Repository {
	return &repository_struct{db: db, red: red}
}

var (
	UserExist          = errors.New("this user already exists")
	ErrSessionNotFound = errors.New("session not found")
	ErrTokenNotFound   = errors.New("reset token not found")
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

func (s *repository_struct) GetUserByEmail(email string) (*models.User, error) {
	user := new(models.User)
	err := s.db.QueryRow(context.Background(),
		`SELECT id, email, password, first_name, last_name, patronymic, phone, photo, created_at
	FROM users
	WHERE email = $1`, email).Scan(
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

func (s *repository_struct) UpdatePassword(tx pgx.Tx, password string, userID int) error {
	_, err := tx.Exec(
		context.Background(),
		`UPDATE users SET password = $1 WHERE id = $2`, password, userID)
	if err != nil {
		return fmt.Errorf("error update password:%w", err)
	}
	return nil
}

func (s *repository_struct) UpdatePasswordProfile(password string, userID int) error {
	_, err := s.db.Exec(
		context.Background(),
		`UPDATE users SET password = $1 WHERE id = $2`, password, userID)
	if err != nil {
		return fmt.Errorf("error update password:%w", err)
	}
	return nil
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
	fmt.Printf("Redis key: %s\n", key)
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

func (r *repository_struct) CreateSessionRedis(session *models.Session, ttl time.Duration, token string) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	key := fmt.Sprintf("session:%s", token)
	fmt.Printf("Redis key: %s\n", key)
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

func (s *repository_struct) LoadCatalogTablesAuthorized(filter *models.FilterModel, userID, limit int) ([]models.Response_Tables_Authorized, error) {
	args := []any{userID}
	wcondition, wArgs, idx := pkg.BuildFilter(filter, 2)
	args = append(args, wArgs...)
	query := `SELECT 
    t.id, p.id, t.name, t.photo, t.description,
    t.price, t.min_height, 
    t.max_height, 
    t.load_capacity,
    t.lifting_mechanism, 
    t.height_storage_console,
    t.type_support, 
    t.category_id,
    CASE WHEN cc.id IS NOT NULL THEN true ELSE false END as in_cart 
FROM tables t
LEFT JOIN products p ON p.tables_id = t.id
LEFT JOIN cart c ON c.user_id = $1
LEFT JOIN cart_config cc ON cc.cart_id = c.id AND cc.product_id = p.id`
	if len(wcondition) > 0 {
		query += " WHERE " + strings.Join(wcondition, " AND ")
	}
	order := "ASC"
	if filter.Order != nil && *filter.Order == 1 {
		order = "DESC"
	}
	query += fmt.Sprintf("\nORDER BY t.price %s", order)
	offset := (filter.Page - 1) * limit
	query += fmt.Sprintf("\nLIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)
	rows, err := s.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("error with loaded tables: %w", err)
	}
	defer rows.Close()
	var tables []models.Response_Tables_Authorized
	for rows.Next() {
		var table models.Response_Tables_Authorized
		if err := rows.Scan(
			&table.ID,
			&table.Product_ID,
			&table.Name,
			&table.Photo,
			&table.Description,
			&table.Price,
			&table.Min_height,
			&table.Max_height,
			&table.Load_capacity,
			&table.Lifting_mechanism,
			&table.Height_storage_console,
			&table.Type_support,
			&table.Category_id,
			&table.In_Cart,
		); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, nil
}

func (s *repository_struct) LoadCatalogTablesGuest(filter *models.FilterModel, limit int) ([]models.Response_Tables_Guest, error) {
	wcondition, args, idx := pkg.BuildFilter(filter, 1)
	query := `SELECT 
    t.id, t.name, t.photo, t.description,
    t.price, t.min_height, 
    t.max_height, 
    t.load_capacity,
    t.lifting_mechanism, 
    t.height_storage_console,
    t.type_support, 
    t.category_id
FROM tables t`
	if len(wcondition) > 0 {
		query += " WHERE " + strings.Join(wcondition, " AND ")
	}
	order := "ASC"
	if filter.Order != nil && *filter.Order == 1 {
		order = "DESC"
	}
	query += fmt.Sprintf("\nORDER BY t.price %s", order)
	offset := (filter.Page - 1) * limit
	query += fmt.Sprintf("\nLIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)
	rows, err := s.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("error with loaded tables: %w", err)
	}
	defer rows.Close()
	var tables []models.Response_Tables_Guest
	for rows.Next() {
		var table models.Response_Tables_Guest
		if err := rows.Scan(
			&table.ID,
			&table.Name,
			&table.Photo,
			&table.Description,
			&table.Price,
			&table.Min_height,
			&table.Max_height,
			&table.Load_capacity,
			&table.Lifting_mechanism,
			&table.Height_storage_console,
			&table.Type_support,
			&table.Category_id,
		); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, nil
}

func (s *repository_struct) LoadCatalogUnderframeAuthorized(filter *models.FilterModel, userID, limit int) ([]models.Response_Underframe_Authorized, error) {
	args := []any{userID}
	wcondition, wArgs, idx := pkg.BuildFilter(filter, 2)
	args = append(args, wArgs...)
	query := `SELECT 
    u.id, p.id, u.name, u.photo, u.description,
    u.price, u.min_height, 
    u.max_height, 
    u.load_capacity,
    u.lifting_mechanism, 
	u.type_support,
	u.frame_width,
	u.category_id, 
    u.height_storage_console,
	CASE WHEN cc.id IS NOT NULL THEN true ELSE false END as in_cart 
FROM underframe u
LEFT JOIN products p ON p.underframe_id = u.id
LEFT JOIN cart c ON c.user_id = $1
LEFT JOIN cart_config cc ON cc.cart_id = c.id AND cc.product_id = p.id
`
	if len(wcondition) > 0 {
		query += " WHERE " + strings.Join(wcondition, " AND ")
	}
	order := "ASC"
	if filter.Order != nil && *filter.Order == 1 {
		order = "DESC"
	}
	query += fmt.Sprintf("\nORDER BY u.price %s", order)
	offset := (filter.Page - 1) * limit
	query += fmt.Sprintf("\nLIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)
	rows, err := s.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("error with loaded tables: %w", err)
	}
	defer rows.Close()
	var underframes []models.Response_Underframe_Authorized
	for rows.Next() {
		var u models.Response_Underframe_Authorized
		if err := rows.Scan(
			&u.ID,
			&u.Product_ID,
			&u.Name,
			&u.Photo,
			&u.Description,
			&u.Price,
			&u.Min_height,
			&u.Max_height,
			&u.Load_capacity,
			&u.Lifting_mechanism,
			&u.Type_support,
			&u.Frame_width,
			&u.Category_id,
			&u.Height_storage_console,
			&u.In_Cart,
		); err != nil {
			return nil, err
		}
		underframes = append(underframes, u)
	}
	return underframes, nil
}

func (s *repository_struct) LoadCatalogUnderframeGuest(filter *models.FilterModel, limit int) ([]models.Response_Underframe_Guest, error) {
	wcondition, args, idx := pkg.BuildFilter(filter, 1)
	query := `SELECT 
    u.id, u.name, u.photo, u.description,
    u.price, u.min_height, 
    u.max_height, 
    u.load_capacity,
    u.lifting_mechanism, 
	u.type_support,
	u.frame_width,
	u.category_id, 
    u.height_storage_console
FROM underframe u`
	if len(wcondition) > 0 {
		query += " WHERE " + strings.Join(wcondition, " AND ")
	}
	order := "ASC"
	if filter.Order != nil && *filter.Order == 1 {
		order = "DESC"
	}
	query += fmt.Sprintf("\nORDER BY u.price %s", order)
	offset := (filter.Page - 1) * limit
	query += fmt.Sprintf("\nLIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)
	rows, err := s.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("error with loaded tables: %w", err)
	}
	defer rows.Close()
	var underframes []models.Response_Underframe_Guest
	for rows.Next() {
		var u models.Response_Underframe_Guest
		if err := rows.Scan(
			&u.ID,
			&u.Name,
			&u.Photo,
			&u.Description,
			&u.Price,
			&u.Min_height,
			&u.Max_height,
			&u.Load_capacity,
			&u.Lifting_mechanism,
			&u.Type_support,
			&u.Frame_width,
			&u.Category_id,
			&u.Height_storage_console,
		); err != nil {
			return nil, err
		}
		underframes = append(underframes, u)
	}
	return underframes, nil
}

func (s *repository_struct) LoadProfile(userID int) (*models.Response_Profile, error) {
	profile := new(models.Response_Profile)
	err := s.db.QueryRow(
		context.Background(),
		`SELECT 
    	u.email,
		u.first_name,
		u.last_name,
		u.patronymic,
		u.phone,
		u.photo,
		u.role,
		u.pick_up_point_id,
    (SELECT COUNT(*)
     		FROM cart c
     		JOIN cart_config cf ON c.id = cf.cart_id
     		WHERE c.user_id = u.id) AS cart_items
		FROM users u
		WHERE u.id = $1;`,
		userID).Scan(
		&profile.Email,
		&profile.First_name,
		&profile.Last_name,
		&profile.Patronymic,
		&profile.Phone,
		&profile.Photo,
		&profile.Role,
		&profile.Pick_up_point_ID,
		&profile.Cart_Items,
	)
	if err != nil {
		logger.Log.Error(err.Error())
		return nil, err
	}
	return profile, nil
}

func (s *repository_struct) CreateResetToken(tx pgx.Tx, userID int, token string, expiresAt time.Time) error {
	_, err := tx.Exec(context.Background(),
		`UPDATE reset_token SET used = true WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("invalidate old tokens: %w", err)
	}
	_, err = tx.Exec(context.Background(),
		`INSERT INTO reset_token (user_id, token, expires_at, used) 
        VALUES ($1, $2, $3, false)`,
		userID, token, expiresAt)

	if err != nil {
		return fmt.Errorf("create reset token: %w", err)
	}
	return nil
}

func (s *repository_struct) FindValid(tx pgx.Tx, token string) (*models.Reset_Token, error) {
	t := new(models.Reset_Token)
	err := tx.QueryRow(context.Background(),
		`SELECT id, user_id, token, expires_at, used
		FROM reset_token
		WHERE token = $1 AND expires_at > NOW() AND used = false
		LIMIT 1`, token).Scan(
		&t.ID,
		&t.UserID,
		&t.Token,
		&t.ExpiresAt,
		&t.Used,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenNotFound
		}
		return nil, fmt.Errorf("find reset token: %w", err)
	}
	return t, nil
}

func (s *repository_struct) MarkUsed(tx pgx.Tx, token string) (bool, error) {
	mark, err := tx.Exec(context.Background(),
		`UPDATE reset_token SET used = true WHERE token = $1 AND used = false`, token)
	if err != nil {
		return false, fmt.Errorf("mark token user :%w", err)
	}
	return mark.RowsAffected() > 0, nil
}

func (s *repository_struct) CleanExpiredResetTokens() error {
	_, err := s.db.Exec(context.Background(),
		`DELETE FROM reset_token WHERE expires_at < NOW() OR used = true`)
	if err != nil {
		log.Printf("Cleanup error: %v", err)
	}
	return nil
}

func (s *repository_struct) GetAllPickUpPoints(userID int) ([]models.PickUpPoint_Model, error) {
	var items []models.PickUpPoint_Model
	req, err := s.db.Query(context.Background(),
		`SELECT
			p.id,
			p.name,
			p.address,
			p.openning_hours,
			(SELECT pick_up_point_id FROM users WHERE id = $1) AS default_point
		FROM pick_up_point p
		ORDER BY p.id ASC`,
		userID)
	if err != nil {
		return nil, err
	}
	defer req.Close()
	for req.Next() {
		var item models.PickUpPoint_Model
		err := req.Scan(
			&item.ID,
			&item.Name,
			&item.Address,
			&item.OpeningHours,
			&item.DefaultPoint,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *repository_struct) SavePickUpPoint(userID, pickupID int) error {
	_, err := s.db.Exec(context.Background(), `UPDATE users SET pick_up_point_id = $1 WHERE id = $2`, pickupID, userID)
	if err != nil {
		return err
	}
	return nil
}

func (s *repository_struct) ChangeUserData(data *models.ChangeUserData, userID int) error {
	_, err := s.db.Exec(context.Background(),
		`UPDATE users SET email = $1, first_name = $2, last_name = $3, patronymic = $4, phone = $5 WHERE id = $6`,
		data.Email, data.First_name, data.Last_name, data.Patronymic, data.Phone, userID)
	if err != nil {
		return err
	}
	return nil
}

func (s *repository_struct) GetAllOrders(userID int) ([]models.OrderRequest, error) {
	var items []models.OrderRequest
	req, err := s.db.Query(context.Background(),
		`SELECT o.order_token, s.name, o.date, o.sum FROM "order" o
		JOIN status_order s ON o.status_order_id = s.id
		WHERE user_id = $1
		ORDER BY date DESC`,
		userID)
	if err != nil {
		return nil, err
	}
	defer req.Close()
	for req.Next() {
		var item models.OrderRequest
		err := req.Scan(
			&item.Order_token,
			&item.Status,
			&item.Date,
			&item.Sum,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *repository_struct) GetInfoOrder(userID int, orderToken string) ([]models.OrderInfoRequest, error) {
	var items []models.OrderInfoRequest
	rows, err := r.db.Query(
		context.Background(),
		`SELECT 
			t.photo as table_photo,
			tt.photo as tabletop_photo,
			u.photo as underframe_photo,
			p.name, 
			p.price, 
			op.quantity, 
			o.sum, 
			o.date 
		FROM order_products op
		JOIN "order" o ON op.order_id = o.id
		JOIN products p ON op.product_id = p.id
		LEFT JOIN tables t ON p.tables_id = t.id
		LEFT JOIN tabletop tt ON p.tabletop_id = tt.id
		LEFT JOIN underframe u ON p.underframe_id = u.id
		WHERE o.user_id = $1 AND o.order_token = $2`,
		userID, orderToken,
	)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var item models.OrderInfoRequest
		err := rows.Scan(
			&item.Photo,
			&item.Name,
			&item.Price,
			&item.Quantity,
			&item.Sum,
			&item.Date,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *repository_struct) AddOrder(userID, point_id int, orderToken string) error {
	var order_id int
	ctx := context.Background()
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	fmt.Println(point_id)
	err = tx.QueryRow(
		ctx,
		`WITH cartid AS (
    SELECT id FROM cart WHERE user_id = $1
),
cart_total AS (
    SELECT COALESCE(SUM(
        COALESCE(t.price, tb.price, u.price, 0) * cc.quantity
    ), 0) as total_sum
    FROM cart_config cc
    JOIN products p ON cc.product_id = p.id
    LEFT JOIN tables t ON p.tables_id = t.id
    LEFT JOIN tabletop tb ON p.tabletop_id = tb.id
    LEFT JOIN underframe u ON p.underframe_id = u.id
    WHERE cc.cart_id = (SELECT id FROM cartid)
)
INSERT INTO "order" (order_token, status_order_id, user_id, date, sum, pick_up_point_id)
VALUES ($2, $3, $1, NOW(), (SELECT total_sum FROM cart_total), $4) RETURNING id;`,
		userID,
		orderToken,
		1,
		point_id,
	).Scan(&order_id)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}
	_, err = tx.Exec(
		ctx,
		`INSERT INTO order_products (order_id, product_id, quantity, price)
		SELECT $1,
			cc.product_id,
			cc.quantity,
			COALESCE(t.price, tb.price, u.price, 0)
		FROM cart_config cc
		JOIN products p ON cc.product_id = p.id
		LEFT JOIN tables t ON p.tables_id = t.id
		LEFT JOIN tabletop tb ON p.tabletop_id = tb.id
		LEFT JOIN underframe u ON p.underframe_id = u.id
		WHERE cc.cart_id = (SELECT id FROM cart WHERE user_id = $2);`,
		order_id, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to insert order products: %w", err)
	}
	err = tx.Commit(ctx)
	if err != nil {
		return err
	}
	return nil
}
