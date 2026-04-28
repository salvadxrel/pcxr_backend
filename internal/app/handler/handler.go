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
	"strconv"
	"strings"
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
				logger.Log.Debug("session expired")
			} else {
				logger.Log.Debug("invalid session")
			}
			next.ServeHTTP(w, r)
			return
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

func (s *handler) CatalogTables(w http.ResponseWriter, r *http.Request) {
	filter, err := parseFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	userID, ok := r.Context().Value("userID").(int)
	if !ok {
		tables, err := s.serv.LoadCatalogTablesGuestService(filter, 9)
		if err != nil {
			log.Printf("CatalogTablesGuest service error: %v", err)
			http.Error(w, "iternal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-type", "application/json")
		json.NewEncoder(w).Encode(tables)
	} else {
		tables, err := s.serv.LoadCatalogTablesAuthorizedService(filter, userID, 9)
		if err != nil {
			log.Printf("CatalogTablesAuthorized service error: %v", err)
			http.Error(w, "iternal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-type", "application/json")
		json.NewEncoder(w).Encode(tables)
	}
}

func (s *handler) CatalogUnderframe(w http.ResponseWriter, r *http.Request) {
	filter, err := parseFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	userID, ok := r.Context().Value("userID").(int)
	if !ok {
		underframes, err := s.serv.LoadCatalogUnderframeGuestService(filter, 9)
		if err != nil {
			log.Printf("CatalogUnderframeGuest service error: %v", err)
			http.Error(w, "iternal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-type", "application/json")
		json.NewEncoder(w).Encode(underframes)
	} else {
		tables, err := s.serv.LoadCatalogUnderframeAuthorizedService(filter, userID, 9)
		if err != nil {
			log.Printf("CatalogUnderframeAuthorized service error: %v", err)
			http.Error(w, "iternal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-type", "application/json")
		json.NewEncoder(w).Encode(tables)
	}
}

func parseFilter(r *http.Request) (*models.FilterModel, error) {
	q := r.URL.Query()
	filter := new(models.FilterModel)
	if cats := q["category"]; len(cats) > 0 {
		filter.Categories = cats
	}
	if lift := q.Get("lift"); lift != "" {
		filter.Lift = strings.Split(lift, ",")
	}
	if panel := q.Get("panel"); panel != "" {
		filter.Panel = strings.Split(panel, ",")
	}
	if type_support := q.Get("support"); type_support != "" {
		filter.Type_Support = strings.Split(type_support, ",")
	}
	if pMin := q.Get("pmin"); pMin != "" {
		if val, err := strconv.ParseFloat(pMin, 64); err == nil {
			filter.Price_min = &val
		}
	}
	if pMax := q.Get("pmax"); pMax != "" {
		if val, err := strconv.ParseFloat(pMax, 64); err == nil {
			filter.Price_max = &val
		}
	}
	if fMin := q.Get("fmin"); fMin != "" {
		if val, err := strconv.Atoi(fMin); err == nil {
			filter.Frame_min = &val
		}
	}
	if fMax := q.Get("fmax"); fMax != "" {
		if val, err := strconv.Atoi(fMax); err == nil {
			filter.Frame_max = &val
		}
	}
	if lcMin := q.Get("lcmin"); lcMin != "" {
		if val, err := strconv.Atoi(lcMin); err == nil {
			filter.Load_capacity_min = &val
		}
	}
	if lcMax := q.Get("lcmax"); lcMax != "" {
		if val, err := strconv.Atoi(lcMax); err == nil {
			filter.Load_capacity_max = &val
		}
	}
	if fwMin := q.Get("fwmin"); fwMin != "" {
		if val, err := strconv.Atoi(fwMin); err == nil {
			filter.Frame_width_min = &val
		}
	}
	if fwMax := q.Get("fwmax"); fwMax != "" {
		if val, err := strconv.Atoi(fwMax); err == nil {
			filter.Frame_width_max = &val
		}
	}
	if order := q.Get("order"); order != "" {
		if val, err := strconv.Atoi(order); err == nil {
			filter.Order = &val
		}
	}
	page := 1
	if p := q.Get("page"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 0 {
			page = val
		}
	}
	filter.Page = page
	return filter, nil
}
