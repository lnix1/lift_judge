package server

import (
	"net/http"
	"time"

	"github.com/lnix1/lift_judge/internal/auth"
	resp "github.com/lnix1/lift_judge/internal/responses"
)

func (apiCfg *ApiCfg) handlerRefresh(w http.ResponseWriter, r *http.Request) {
	type returnVals struct {
		Token 	string `json:"token"`
	}

	bearer, err := auth.GetBearerToken(r.Header)
	if err != nil {
		resp.RespondWithError(w, http.StatusUnauthorized, "No valid refresh token", err)
		return
	}

	dbBearer, err := apiCfg.Db.GetRefreshToken(r.Context(), bearer)
	if err != nil || dbBearer.ExpiredBool == false || dbBearer.RevokeCheck == false {
		resp.RespondWithError(w, http.StatusUnauthorized, "No valid refresh token", err)
		return
	}

	accessToken, err := auth.MakeJWT(dbBearer.UserID, apiCfg.Secret, time.Duration(3600) * time.Second)
	if err != nil {
		resp.RespondWithError(w, http.StatusBadRequest, "Error generating bearer token", err)
		return
	}

	resp.RespondWithJSON(w, http.StatusOK, returnVals{
		Token: accessToken,
	})
	return
}

func (apiCfg *ApiCfg) handlerRevoke(w http.ResponseWriter, r *http.Request) {
	bearer, err := auth.GetBearerToken(r.Header)
	if err != nil {
		resp.RespondWithError(w, http.StatusUnauthorized, "No valid refresh token", err)
		return
	}

	err = apiCfg.Db.RevokeRefreshToken(r.Context(), bearer)
	if err != nil {
		resp.RespondWithError(w, http.StatusUnauthorized, "No refresh token with provided value in database", err)
		return
	}

	resp.RespondWithJSON(w, http.StatusNoContent, nil)
	return
}
