package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"pcxr/internal/app/logger"
	"pcxr/internal/app/models"
	"pcxr/internal/app/service"
	"time"

	"go.uber.org/zap"
)

type handler struct {
	serv service.Service
	//red  *redis.Client
}

func NewHandler(serv service.Service /*, red *redis.Client*/) *handler {
	return &handler{serv: serv /*red: red*/}
}

func (s *handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var reg models.Register_Model
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		logger.Log.Error("пуки каки", zap.Error(err))
		return
	}
	err := s.serv.RegisterUser(&reg)
	if err != nil {
		if errors.Is(err, service.ErrInvalidData) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		logger.Log.Error("пуки каки", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]int{
		"status": http.StatusOK,
	})
	w.Header().Set("Content-type", "application/json")
	defer r.Body.Close()
}

func (s *handler) CartLoads(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("userID").(int)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	result, err := s.serv.CartLoadsService(userID)
	if err != nil {
		return
	}
	json.NewEncoder(w).Encode(result)
	w.Header().Set("Content-type", "application/json")
	defer r.Body.Close()
}

func (s *handler) AddProductToCart(w http.ResponseWriter, r *http.Request) {
	var addCart models.Add_Remove_Cart_Model
	userID, ok := r.Context().Value("userID").(int)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := json.NewDecoder(r.Body).Decode(&addCart); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	err := s.serv.AddProductToCartService(userID, addCart.Product_id)
	if err != nil {
		fmt.Println("ERROR:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(map[string]int{
		"status": http.StatusOK,
	})
	w.Header().Set("Content-type", "application/json")
	defer r.Body.Close()
}

func (s *handler) RemoveProductFromCart(w http.ResponseWriter, r *http.Request) {
	var removeCart models.Add_Remove_Cart_Model
	userID, ok := r.Context().Value("userID").(int)
	if !ok {
		http.Error(w, "", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-type", "application/json")
	if err := json.NewDecoder(r.Body).Decode(&removeCart); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err := s.serv.RemoveProductFromCartService(userID, removeCart.Product_id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(map[string]int{
		"status": http.StatusOK,
	})
	defer r.Body.Close()
}

func (s *handler) CheckSessionToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Log.Info("handler CheckSession launched")
		cookie, err := r.Cookie("session_token")
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		sess, err := s.serv.CheckSessionTokenService(cookie.Value)
		if err != nil {
			if errors.Is(err, service.ErrInvalidExpires) {
				http.Error(w, `{"error":"invadil_expired"}`, http.StatusUnauthorized)
				return
			} else {
				http.Error(w, `{"error":"invalid_session"}`, http.StatusUnauthorized)
				return
			}
		}
		if err := s.serv.UpdateSessionExpiryService(cookie.Value); err != nil {
			log.Printf("failed %s: %v", cookie.Value, err)
		}
		ctx := context.WithValue(r.Context(), "userID", sess.User_ID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *handler) Logout(w http.ResponseWriter, r *http.Request) {
	token, err := r.Cookie("session_token")
	if err != nil {
		return
	}
	if err := s.serv.DeleteSession(token.Value); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *handler) LoginUser(w http.ResponseWriter, r *http.Request) {
	var data models.Login_Model
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		log.Printf("JSON decode error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	token, err := s.serv.LoginService(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   int(24 * time.Hour.Seconds()),
	})
	logger.Log.Info("welcUm to gym")
	w.Header().Set("Content-type", "application/json")
	defer r.Body.Close()
}
