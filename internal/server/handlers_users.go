package server

import (
	"log"
	"time"
	"net/http"
	"encoding/json"

	"github.com/lnix1/lift_judge/internal/auth"
	"github.com/lnix1/lift_judge/internal/database"
	resp "github.com/lnix1/lift_judge/internal/responses"
	"github.com/google/uuid"
)

func (apiCfg *ApiCfg) handlerCreateUser(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Password 	string	`json:"password"`
		Email		string	`json:"email"`
	}
	type returnVals struct {
		Id		uuid.UUID 	`json:"id"`
		Created_at 	time.Time	`json:"created_at"`
		Updated_at 	time.Time	`json:"updated_at"`
		Email 		string		`json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		resp.RespondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
		return
	}
	if params.Email == "" || params.Password == "" {
		resp.RespondWithError(w, http.StatusBadRequest, "Password or email missing", err)
		return
	}

	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		resp.RespondWithError(w, http.StatusInternalServerError, "Error generating password hash", err)
		return
	}
	
	user, err := apiCfg.Db.CreateUser(r.Context(), database.CreateUserParams{params.Email, hashedPassword})
	if err != nil {
		resp.RespondWithError(w, http.StatusBadRequest, "Couldn't create new user", err)
	}
	
	resp.RespondWithJSON(w, http.StatusCreated, returnVals{
		Id: user.ID,
		Created_at: user.CreatedAt,
		Updated_at: user.UpdatedAt,
		Email: user.Email,
	})
	return
}

func (apiCfg *ApiCfg) handlerUpdateEmailPassword(w http.ResponseWriter, r *http.Request) {
	bearer, err := auth.GetBearerToken(r.Header)
	if err != nil {
		resp.RespondWithError(w, http.StatusUnauthorized, "Authentication token missing", err)
	}
	
	bearerUserID, err := auth.ValidateJWT(bearer, apiCfg.Secret)
	if err != nil {
		resp.RespondWithError(w, http.StatusUnauthorized, "Invalid authentication token", err)
	}

	type parameters struct {
		Password 	string	`json:"password"`
		Email		string	`json:"email"`
	}
	type returnVals struct {
		Id		uuid.UUID 	`json:"id"`
		Created_at 	time.Time	`json:"created_at"`
		Updated_at 	time.Time	`json:"updated_at"`
		Email 		string		`json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		resp.RespondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
		return
	}
	if params.Email == "" || params.Password == "" {
		resp.RespondWithError(w, http.StatusBadRequest, "New password or email missing", err)
		return
	}

	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		resp.RespondWithError(w, http.StatusInternalServerError, "Error with password creation", err)
		return
	}
	updateArgs := database.UpdateEmailUserParams{Email: params.Email, HashedPassword: hashedPassword, ID: bearerUserID}
	user, err := apiCfg.Db.UpdateEmailUser(r.Context(), updateArgs)
	if err != nil {
		resp.RespondWithError(w, http.StatusBadRequest, "Failed to udpate email and password", err)
		return
	}

	resp.RespondWithJSON(w, http.StatusOK, returnVals{
		Id: user.ID,
		Created_at: user.CreatedAt,
		Updated_at: user.UpdatedAt,
		Email: user.Email,
	})
	return
}
