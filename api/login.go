package api

import (
	api_request "education-website/api/request"
	api_response "education-website/api/response"
	"education-website/entity/user"
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"go.elastic.co/apm"
	"io/ioutil"
	"net/http"
)

func handlerLoginUser(w http.ResponseWriter, r *http.Request) {
	ctx := apm.DetachedContext(r.Context())
	logger := GetLoggerWithContext(ctx).WithField("METHOD", "handleRetryUserAccount")
	logger.Infof("Handle user account")

	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.WithError(err).Warningf("Error when reading from request")
		http.Error(w, "Invalid format", 252001)
		return
	}

	json.NewDecoder(r.Body)
	defer r.Body.Close()

	// this is the information that the user type in front end
	var userRequest api_request.LoginRequest
	err = json.Unmarshal(bodyBytes, &userRequest)
	if err != nil {
		log.WithError(err).Warningf("Error when unmarshaling data from request")
		http.Error(w, "Status internal Request", http.StatusInternalServerError) // Return a internal server error
		return
	}

	// get the user from request from database
	userEntityInfo, err := userService.GetUserNamePassword(userRequest, ctx)
	if err != nil {
		log.WithError(err).Warningf("Error verify Username and Password for user")
		http.Error(w, "Status internal Request", http.StatusInternalServerError) // Return a internal server error
		return
	}

	// verify the user password
	checkPasswordSimilarity, err := authService.VerifyUser(userRequest, *userEntityInfo)
	if err != nil {
		log.WithError(err).Errorf("Invalid username or password for user: %s", userEntityInfo.UserName)
		return
	}

	// if the username exist and the password is true, generate token to send back to backend
	if checkPasswordSimilarity != nil {
		generatedToken := jwtService.GenerateToken(user.UserEntity{
			UserName: userEntityInfo.UserName,
			Role:     userEntityInfo.Role,
		})

		// Gán token vào đối tượng model.User
		userInfoWithToken := map[string]interface{}{
			"user": api_response.UserDto{
				UserId:       userEntityInfo.UserId,
				UserName:     userEntityInfo.UserName,
				Email:        userEntityInfo.Email,
				Role:         userEntityInfo.Role,
				DOB:          userEntityInfo.DOB,
				JobPosition:  userEntityInfo.JobPosition,
				StartingDate: userEntityInfo.StartDate,
			},
			"token": generatedToken,
		}

		// Trả về thông tin người dùng cùng với token
		// Trả về phản hồi JSON
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(userInfoWithToken)
	} else {
		// return cannot find user
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode("Invalid password or username")
	}
}
