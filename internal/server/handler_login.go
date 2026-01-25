package server

import (
	"net/http"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lnix1/lift_judge/internal/auth"
	"github.com/lnix1/lift_judge/internal/database"
	resp "github.com/lnix1/lift_judge/internal/responses"
)

func (apiCfg *ApiCfg) handlerLogin(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Password 	string	`json:"password"`
		Email		string	`json:"email"`
	}
	type returnVals struct {
		Id		uuid.UUID 	`json:"id"`
		Created_at 	time.Time	`json:"created_at"`
		Updated_at 	time.Time	`json:"updated_at"`
		Email 		string		`json:"email"`
		Token		string		`json:"token"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		resp.RespondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
		return
	}
	
	user, err := apiCfg.Db.GetUser(r.Context(), params.Email)
	if err != nil {
		resp.RespondWithError(w, http.StatusUnauthorized, "Incorrect email or password", err)
		return
	}

	passCheck, err := auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if passCheck == false || err != nil {
		resp.RespondWithError(w, http.StatusUnauthorized, "Incorrect email or password", err)
		return
	}
	
	accessToken, err := auth.MakeJWT(user.ID, apiCfg.Secret, time.Duration(3600) * time.Second)
	if err != nil {
		resp.RespondWithError(w, http.StatusBadRequest, "Error generating bearer token", err)
		return
	}

	refreshToken, _ := auth.MakeRefreshToken()
	_, err = apiCfg.Db.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{Token: refreshToken, UserID: user.ID})
	if err != nil {
		resp.RespondWithError(w, http.StatusInternalServerError, "Error creating user refresh token", err)
		return
	}

	http.SetCookie(w, &http.Cookie{
    		Name:     "RefreshToken",
    		Value:    refreshToken,   // the new token string
    		Path:     "/",
    		HttpOnly: true,
    		//Secure:   true,           // if using HTTPS, which you should in prod
    		SameSite: http.SameSiteStrictMode, // or Lax, depending on your needs
    		Expires:  time.Now().Add(1 * time.Hour),
	})
	resp.RespondWithJSON(w, http.StatusOK, returnVals{
		Id: user.ID,
		Created_at: user.CreatedAt,
		Updated_at: user.UpdatedAt,
		Email: user.Email,
		Token: accessToken,
	})
	return
}
