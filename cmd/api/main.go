package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"pcxr/internal/app/database"
	"pcxr/internal/app/email"
	"pcxr/internal/app/handler"
	"pcxr/internal/app/logger"
	"pcxr/internal/app/models"
	"pcxr/internal/app/repository"
	"pcxr/internal/app/service"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	logger.Log.Info("скидыщ")
	logger.Log.Info("Program has started")
	err := godotenv.Load(".env")
	ctx := context.Background()
	if err != nil {
		log.Fatalf("gg: %v", err)
	}
	cfgRedis, err := database.RedisPool(ctx, models.Redis_Config_Model{
		Addr:        os.Getenv("REDIS_ADDR"),
		Password:    os.Getenv("REDIS_PASSWORD"),
		DB:          0,
		User:        os.Getenv("REDIS_NAME"),
		MaxRetries:  5,
		DialTimeout: 10 * time.Second,
		Timeout:     5 * time.Second,
	})
	if err != nil {
		panic(err)
	}
	db, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("gg: %v", err)
	}
	if err := db.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to ping database: %v\n", err)
		panic(err)
	}
	defer db.Close()
	emailSender := email.NewSMTPSender(
		os.Getenv("SMTP_HOST"),
		587,
		os.Getenv("EMAIL"),
		os.Getenv("PASSWORD_MAIL"),
	)
	repository := repository.NewRepository(db, cfgRedis)
	service := service.NewService(repository, *emailSender, db)
	handler := handler.NewHandler(service /*, cfgRedis*/)

	r := chi.NewRouter()
	r.Use(handler.CheckSessionToken)
	r.Post("/logout", handler.Logout)
	r.Post("/reg", handler.CreateUser)
	r.Post("/login", handler.LoginUser)
	r.Post("/forgot_password", handler.RequestPasswordReset)
	r.Post("/reset_password", handler.ConfrimPasswordReset)
	r.Post("/change_password", handler.ChangePasswordProfile)
	r.Post("/change_data", handler.ChangeUserData)
	r.Post("/save_point", handler.SavePickUpPoint)
	r.Post("/get_info_order", handler.GetInfoOrder)
	r.Post("/add_order", handler.AddOrder)
	r.Get("/get_all_orders", handler.GetOrders)
	r.Get("/get_points", handler.GetPickUpPoint)
	r.Get("/catalog/tables", handler.CatalogTables)
	r.Get("/catalog/underframe", handler.CatalogUnderframe)
	r.Get("/cart", handler.CartLoads)
	r.Get("/addcart", handler.AddProductToCart)
	r.Get("/removecart", handler.RemoveProductFromCart)
	r.Get("/profile", handler.LoadProfile)
	http.ListenAndServe(":1337", r)
}
