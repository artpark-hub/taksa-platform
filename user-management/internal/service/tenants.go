package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	v1 "user-management/api/tenants/v1"
	"user-management/internal/biz"
	"user-management/internal/data"

	"net/url"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

type TenantsService struct {
	v1.UnimplementedTenantsServiceServer

	uc              *biz.TenantsUsecase
	log             *log.Helper
	data            *data.Data
	kratosPublicURL string
	kratosAdminURL  string
}

type JWTDetails struct {
	OrganizationName string
	Role             string
	Email            string
	Subject          string
}

func NewTenantsService(uc *biz.TenantsUsecase, d *data.Data, logger log.Logger) *TenantsService {
	return &TenantsService{
		uc:              uc,
		data:            d,
		log:             log.NewHelper(logger),
		kratosPublicURL: d.KratosPublicURL(),
		kratosAdminURL:  d.KratosAdminURL(),
	}
}

// ---------------------------------------------------------
// 1. RegisterMasterUser
// ---------------------------------------------------------
func (s *TenantsService) RegisterMasterUser(ctx context.Context, req *v1.RegisterMasterUserRequest) (*v1.RegisterMasterUserResponse, error) {
	s.log.WithContext(ctx).Infof("RegisterMasterUser: %s (Org: %s)", req.Email, req.OrganizationName)

	data, err := s.uc.RegisterMasterUser(ctx, req.Email, req.Password, req.FirstName, req.LastName, req.OrganizationName)
	if err != nil {
		return nil, err
	}

	return &v1.RegisterMasterUserResponse{
		Data: &v1.TenantData{
			IdentityId:       data.IdentityID,
			OrganizationName: data.OrganizationName,
			Status:           "active",
		},
	}, nil
}

// ---------------------------------------------------------
// 2. LoginUser - Proxy to Kratos API flow
// ---------------------------------------------------------
func (s *TenantsService) LoginUser(ctx context.Context, req *v1.LoginUserRequest) (*v1.LoginUserResponse, error) {
	s.log.WithContext(ctx).Infof("LoginUser: %s", req.Email)

	flowURL := fmt.Sprintf("%s/self-service/login/api", s.kratosPublicURL)
	flowResp, err := http.Get(flowURL)
	if err != nil {
		s.log.Errorf("Failed to get login flow: %v", err)
		return nil, fmt.Errorf("failed to initialize login flow: %w", err)
	}
	defer flowResp.Body.Close()

	if flowResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(flowResp.Body)
		s.log.Errorf("Kratos flow init failed: %s", string(body))
		return nil, fmt.Errorf("kratos flow init failed with status %d", flowResp.StatusCode)
	}

	var flow struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(flowResp.Body).Decode(&flow); err != nil {
		s.log.Errorf("Failed to decode flow: %v", err)
		return nil, fmt.Errorf("failed to decode flow response: %w", err)
	}

	s.log.Infof("Login flow ID: %s", flow.ID)

	loginData := map[string]interface{}{
		"method":     "password",
		"identifier": req.Email,
		"password":   req.Password,
	}

	loginJSON, err := json.Marshal(loginData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal login data: %w", err)
	}

	loginURL := fmt.Sprintf("%s/self-service/login?flow=%s", s.kratosPublicURL, flow.ID)
	loginResp, err := http.Post(loginURL, "application/json", bytes.NewBuffer(loginJSON))
	if err != nil {
		s.log.Errorf("Failed to submit login: %v", err)
		return nil, fmt.Errorf("failed to submit login: %w", err)
	}
	defer loginResp.Body.Close()

	if loginResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(loginResp.Body)
		s.log.Errorf("Kratos login failed: %s", string(body))
		return nil, fmt.Errorf("login failed: invalid credentials or error")
	}

	var kratosLogin struct {
		SessionToken string `json:"session_token"`
		Session      struct {
			Identity struct {
				Traits map[string]interface{} `json:"traits"`
			} `json:"identity"`
		} `json:"session"`
	}

	if err := json.NewDecoder(loginResp.Body).Decode(&kratosLogin); err != nil {
		s.log.Errorf("Failed to decode login response: %v", err)
		return nil, fmt.Errorf("failed to decode login response: %w", err)
	}

	if kratosLogin.SessionToken == "" {
		s.log.Error("No session token in response")
		return nil, fmt.Errorf("no session token received from Kratos")
	}

	traits := kratosLogin.Session.Identity.Traits

	email, _ := traits["email"].(string)
	orgName, _ := traits["organization_name"].(string)
	role, _ := traits["role"].(string)

	var firstName, lastName string
	if nameMap, ok := traits["name"].(map[string]interface{}); ok {
		firstName, _ = nameMap["first"].(string)
		lastName, _ = nameMap["last"].(string)
	}

	s.log.Infof("Login successful for user: %s (%s %s) - Org: %s, Role: %s", email, firstName, lastName, orgName, role)

	return &v1.LoginUserResponse{
		SessionToken: kratosLogin.SessionToken,
		User: &v1.UserInfo{
			Email:            email,
			FirstName:        firstName,
			LastName:         lastName,
			OrganizationName: orgName,
			Role:             role,
		},
	}, nil
}

// ---------------------------------------------------------
// 3. GetJWTToken
// ---------------------------------------------------------
func (s *TenantsService) GetJWTToken(ctx context.Context, req *v1.GetJWTTokenRequest) (*v1.GetJWTTokenResponse, error) {
	if tr, ok := transport.FromServerContext(ctx); ok {
		authHeader := tr.RequestHeader().Get("Authorization")

		token := authHeader
		if len(authHeader) > 7 && strings.EqualFold(authHeader[0:7], "Bearer ") {
			token = authHeader[7:]
		}

		if token == "" {
			return nil, errors.New("authorization header (JWT) missing from Oathkeeper")
		}

		return &v1.GetJWTTokenResponse{
			Data: &v1.TokenData{
				JwtToken: token,
			},
		}, nil
	}

	return nil, errors.New("failed to extract transport info")
}

// ---------------------------------------------------------
// 4. CreateSubUser - ONLY MASTERS CAN CREATE
// ---------------------------------------------------------
func (s *TenantsService) CreateSubUser(ctx context.Context, req *v1.CreateSubUserRequest) (*v1.CreateSubUserResponse, error) {

	details, err := s.getDetailsFromJWT(ctx)
	if err != nil {
		s.log.Errorf("Failed to extract details from JWT: %v", err)
		return nil, err
	}

	if details.Role != "master" {
		s.log.Warnf("Unauthorized create sub-user attempt by role: %s", details.Role)
		return nil, errors.New("forbidden: only master users can create sub-users")
	}

	s.log.WithContext(ctx).Infof("CreateSubUser: %s under Org: %s by master user", req.Email, details.OrganizationName)

	data, err := s.uc.CreateSubUser(ctx, req.Email, req.Password, req.FirstName, req.LastName, details.OrganizationName)
	if err != nil {
		return nil, err
	}

	return &v1.CreateSubUserResponse{
		Data: &v1.UserData{
			IdentityId:       data.IdentityID,
			OrganizationName: data.OrganizationName,
		},
	}, nil
}

// ---------------------------------------------------------
// 5. ListSubUsers
// ---------------------------------------------------------
func (s *TenantsService) ListSubUsers(ctx context.Context, req *v1.ListSubUsersRequest) (*v1.ListSubUsersResponse, error) {

	details, err := s.getDetailsFromJWT(ctx)
	if err != nil {
		s.log.Errorf("Failed to extract details from JWT: %v", err)
		return nil, err
	}

	users, err := s.uc.ListSubUsers(ctx, details.OrganizationName)
	if err != nil {
		return nil, err
	}

	var protoUsers []*v1.UserSummary
	for _, u := range users {
		protoUsers = append(protoUsers, &v1.UserSummary{
			IdentityId:       u.IdentityID,
			Email:            u.Email,
			FirstName:        u.FirstName,
			LastName:         u.LastName,
			OrganizationName: u.OrganizationName,
			Role:             u.Role,
		})
	}

	return &v1.ListSubUsersResponse{
		Users: protoUsers,
	}, nil
}

// ---------------------------------------------------------
// 6. DeleteSubUser - ONLY MASTERS CAN DELETE
// ---------------------------------------------------------
func (s *TenantsService) DeleteSubUser(ctx context.Context, req *v1.DeleteUserRequest) (*v1.DeleteUserResponse, error) {

	details, err := s.getDetailsFromJWT(ctx)
	if err != nil {
		s.log.Errorf("Failed to extract details from JWT: %v", err)
		return nil, errors.New("unauthorized: unable to verify user role")
	}

	if details.Role != "master" {
		s.log.Warnf("Unauthorized delete attempt by non-master user with role: %s", details.Role)
		return nil, errors.New("forbidden: only master users can delete other users")
	}

	s.log.Infof("Master user deleting sub-user: %s", req.IdentityId)

	err = s.uc.DeleteSubUser(ctx, req.IdentityId)
	if err != nil {
		return nil, err
	}

	return &v1.DeleteUserResponse{
		Status:  "success",
		Message: "User deleted successfully",
	}, nil
}

// ---------------------------------------------------------
// 7. DeleteMasterUser - ONLY MASTERS CAN DELETE
// ---------------------------------------------------------
func (s *TenantsService) DeleteMasterUser(ctx context.Context, req *v1.DeleteUserRequest) (*v1.DeleteUserResponse, error) {

	details, err := s.getDetailsFromJWT(ctx)
	if err != nil {
		s.log.Errorf("Failed to extract details from JWT: %v", err)
		return nil, errors.New("unauthorized: unable to verify user role")
	}

	if details.Role != "master" {
		s.log.Warnf("Unauthorized delete master user attempt by role: %s", details.Role)
		return nil, errors.New("forbidden: only master users can delete other users")
	}

	s.log.Infof("Master user deleting master user: %s", req.IdentityId)

	err = s.uc.DeleteMasterUser(ctx, req.IdentityId)
	if err != nil {
		return nil, err
	}

	return &v1.DeleteUserResponse{
		Status:  "success",
		Message: "Master user deleted successfully",
	}, nil
}

// ---------------------------------------------------------
// 8. ChangePassword - User changes their own password
// ---------------------------------------------------------
func (s *TenantsService) ChangePassword(ctx context.Context, req *v1.ChangePasswordRequest) (*v1.ChangePasswordResponse, error) {

	var sessionToken string
	if tr, ok := transport.FromServerContext(ctx); ok {
		authHeader := tr.RequestHeader().Get("Authorization")
		if len(authHeader) > 7 && strings.EqualFold(authHeader[0:7], "Bearer ") {
			sessionToken = authHeader[7:]
		} else {
			sessionToken = authHeader
		}
	}

	if sessionToken == "" {
		return nil, errors.New("session token missing")
	}

	s.log.Infof("ChangePassword: initiating password change")

	flowURL := fmt.Sprintf("%s/self-service/settings/api", s.kratosPublicURL)
	flowReq, err := http.NewRequest("GET", flowURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create flow request: %w", err)
	}
	flowReq.Header.Set("Authorization", "Bearer "+sessionToken)

	flowResp, err := http.DefaultClient.Do(flowReq)
	if err != nil {
		s.log.Errorf("Failed to initialize settings flow: %v", err)
		return nil, fmt.Errorf("failed to initialize settings flow: %w", err)
	}
	defer flowResp.Body.Close()

	if flowResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(flowResp.Body)
		s.log.Errorf("Settings flow init failed: %s", string(body))
		return nil, fmt.Errorf("settings flow init failed")
	}

	var flow struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(flowResp.Body).Decode(&flow); err != nil {
		return nil, fmt.Errorf("failed to decode flow: %w", err)
	}

	s.log.Infof("Settings flow ID: %s", flow.ID)

	updateURL := fmt.Sprintf("%s/self-service/settings?flow=%s", s.kratosPublicURL, flow.ID)
	updateData := map[string]interface{}{
		"method":   "password",
		"password": req.NewPassword,
	}

	updateJSON, err := json.Marshal(updateData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal update data: %w", err)
	}

	updateReq, err := http.NewRequest("POST", updateURL, bytes.NewBuffer(updateJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create update request: %w", err)
	}
	updateReq.Header.Set("Authorization", "Bearer "+sessionToken)
	updateReq.Header.Set("Content-Type", "application/json")

	updateResp, err := http.DefaultClient.Do(updateReq)
	if err != nil {
		s.log.Errorf("Password update failed: %v", err)
		return nil, fmt.Errorf("password update failed: %w", err)
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(updateResp.Body)
		s.log.Errorf("Password update failed: %s", string(body))
		return nil, fmt.Errorf("password update failed")
	}

	s.log.Info("Password changed successfully")

	return &v1.ChangePasswordResponse{
		Status:  "success",
		Message: "Password updated successfully",
	}, nil
}

// ---------------------------------------------------------
//  9. UpdateUserProfile - Authorization Logic
//     Master Users:
//     - Can update ANYONE (including self)
//     - Can change: first_name, last_name, role
//     - CANNOT change: email (email is read-only)
//     Normal Users:
//     - Can ONLY update SELF
//     - Can change: first_name, last_name
//     - CANNOT change: role, email
//
// ---------------------------------------------------------
func (s *TenantsService) UpdateUserProfile(ctx context.Context, req *v1.UpdateUserProfileRequest) (*v1.UpdateUserProfileResponse, error) {
	details, err := s.getDetailsFromJWT(ctx)
	if err != nil {
		s.log.Errorf("Failed to extract details from JWT: %v", err)
		return nil, errors.New("unauthorized: unable to verify user role")
	}

	isSelf := details.Subject == req.IdentityId

	isChangingRole := req.Role != ""

	if details.Role == "master" {
		if isSelf {
			s.log.Infof("Master user %s updating their own profile", details.Subject)
		} else {
			s.log.Infof("Master user %s updating profile for %s", details.Subject, req.IdentityId)
		}

		if isChangingRole && req.Role != "master" && req.Role != "sub" {
			return nil, errors.New("invalid role: must be 'master' or 'sub'")
		}

	} else {
		if !isSelf {
			s.log.Warnf("Non-master user %s attempted to update another user's profile", details.Subject)
			return nil, errors.New("forbidden: you can only update your own profile")
		}

		if isChangingRole {
			s.log.Warnf("Non-master user %s attempted to change role", details.Subject)
			return nil, errors.New("forbidden: you cannot change your role")
		}

		s.log.Infof("User %s updating their own profile", details.Subject)
	}

	if req.Email != "" {
		s.log.Warnf("Email change attempted but will be ignored (email is read-only)")
	}

	user, err := s.uc.UpdateUserProfile(ctx, req.IdentityId, req.FirstName, req.LastName, req.Role)
	if err != nil {
		s.log.Errorf("Failed to update user profile: %v", err)
		return nil, err
	}

	return &v1.UpdateUserProfileResponse{
		Status:  "success",
		Message: "User profile updated successfully",
		User: &v1.UserInfo{
			Email:            user.Email,
			FirstName:        user.FirstName,
			LastName:         user.LastName,
			OrganizationName: user.OrganizationName,
			Role:             user.Role,
		},
	}, nil
}

// ---------------------------------------------------------
// 10. ForgotPassword - Initiate OTP Recovery Flow
// ---------------------------------------------------------
func (s *TenantsService) ForgotPassword(ctx context.Context, req *v1.ForgotPasswordRequest) (*v1.ForgotPasswordResponse, error) {
	s.log.Infof("ForgotPassword: initiating OTP recovery for email: %s", req.Email)

	flowURL := fmt.Sprintf("%s/self-service/recovery/api", s.kratosPublicURL)
	flowReq, err := http.NewRequest("GET", flowURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create recovery flow request: %w", err)
	}

	flowResp, err := http.DefaultClient.Do(flowReq)
	if err != nil {
		s.log.Errorf("Failed to initialize recovery flow: %v", err)
		return nil, fmt.Errorf("failed to initialize recovery flow: %w", err)
	}
	defer flowResp.Body.Close()

	if flowResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("recovery flow init failed with status: %d", flowResp.StatusCode)
	}

	var flow struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(flowResp.Body).Decode(&flow); err != nil {
		return nil, fmt.Errorf("failed to decode recovery flow: %w", err)
	}

	s.log.Infof("API Recovery flow created: %s", flow.ID)

	recoveryURL := fmt.Sprintf("%s/self-service/recovery?flow=%s", s.kratosPublicURL, flow.ID)
	recoveryData := map[string]interface{}{
		"method": "code",
		"email":  req.Email,
	}

	recoveryJSON, _ := json.Marshal(recoveryData)
	recoveryReq, _ := http.NewRequest("POST", recoveryURL, bytes.NewBuffer(recoveryJSON))
	recoveryReq.Header.Set("Content-Type", "application/json")
	recoveryReq.Header.Set("Accept", "application/json")

	recoveryResp, err := http.DefaultClient.Do(recoveryReq)
	if err != nil {
		s.log.Errorf("Recovery email send failed: %v", err)
		return nil, fmt.Errorf("recovery email send failed: %w", err)
	}
	defer recoveryResp.Body.Close()

	if recoveryResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(recoveryResp.Body)
		s.log.Errorf("Failed to send OTP code: %s", string(body))
		return nil, fmt.Errorf("failed to process recovery request")
	}

	s.log.Infof("OTP code sent to: %s", req.Email)

	return &v1.ForgotPasswordResponse{
		Status:  "success",
		Message: "If an account exists, a 6-digit code has been sent to your email.",
		FlowId:  flow.ID,
	}, nil
}

// ---------------------------------------------------------
// 11. ResetPassword - Validate Code & Update Password
// ---------------------------------------------------------
func (s *TenantsService) ResetPassword(ctx context.Context, req *v1.ResetPasswordRequest) (*v1.ResetPasswordResponse, error) {
	if req.Code == "" || req.NewPassword == "" || req.Email == "" {
		return nil, fmt.Errorf("email, OTP code, and new password are required")
	}

	recoveryURL := fmt.Sprintf("%s/self-service/recovery?flow=%s", s.kratosPublicURL, req.FlowId)
	codeData := map[string]interface{}{"method": "code", "code": req.Code}
	codeJSON, _ := json.Marshal(codeData)
	codeReq, _ := http.NewRequest("POST", recoveryURL, bytes.NewBuffer(codeJSON))
	codeReq.Header.Set("Content-Type", "application/json")
	codeReq.Header.Set("Accept", "application/json")

	codeResp, err := http.DefaultClient.Do(codeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to verify code: %w", err)
	}
	defer codeResp.Body.Close()

	if codeResp.StatusCode != http.StatusOK && codeResp.StatusCode != http.StatusUnprocessableEntity {
		return nil, fmt.Errorf("invalid or expired verification code")
	}

	searchURL := fmt.Sprintf("%s/admin/identities?credentials_identifier=%s", s.kratosAdminURL, url.QueryEscape(req.Email))
	searchReq, _ := http.NewRequest("GET", searchURL, nil)
	searchResp, err := http.DefaultClient.Do(searchReq)
	if err != nil || searchResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to lookup user identity")
	}
	defer searchResp.Body.Close()

	var identities []struct {
		ID     string                 `json:"id"`
		Traits map[string]interface{} `json:"traits"`
	}
	if err := json.NewDecoder(searchResp.Body).Decode(&identities); err != nil || len(identities) == 0 {
		return nil, fmt.Errorf("user not found in system")
	}

	identity := identities[0]

	updateURL := fmt.Sprintf("%s/admin/identities/%s", s.kratosAdminURL, identity.ID)
	updateData := map[string]interface{}{
		"schema_id": "default",
		"state":     "active",
		"traits":    identity.Traits,
		"credentials": map[string]interface{}{
			"password": map[string]interface{}{
				"config": map[string]string{
					"password": req.NewPassword,
				},
			},
		},
	}

	updateJSON, _ := json.Marshal(updateData)
	updateReq, _ := http.NewRequest("PUT", updateURL, bytes.NewBuffer(updateJSON))
	updateReq.Header.Set("Content-Type", "application/json")

	updateResp, err := http.DefaultClient.Do(updateReq)
	if err != nil || updateResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to save new password")
	}
	defer updateResp.Body.Close()

	return &v1.ResetPasswordResponse{
		Status:  "success",
		Message: "Password updated successfully!",
	}, nil
}

// ---------------------------------------------------------
// Helper: Extract All Details from JWT (Single Source of Truth)
// ---------------------------------------------------------
func (s *TenantsService) getDetailsFromJWT(ctx context.Context) (*JWTDetails, error) {

	var tokenString string
	if tr, ok := transport.FromServerContext(ctx); ok {
		authHeader := tr.RequestHeader().Get("Authorization")
		if len(authHeader) > 7 && strings.EqualFold(authHeader[0:7], "Bearer ") {
			tokenString = authHeader[7:]
		} else {
			tokenString = authHeader
		}
	}

	tokenString = strings.TrimSpace(tokenString)
	tokenString = strings.Trim(tokenString, "\"")

	if tokenString == "" {
		return nil, errors.New("jwt token missing from headers")
	}

	token, _, err := new(jwtv5.Parser).ParseUnverified(tokenString, jwtv5.MapClaims{})
	if err != nil {
		s.log.Errorf("Token Parsing Failed: %v", err)
		return nil, errors.New("failed to parse jwt token")
	}

	details := &JWTDetails{}

	if claims, ok := token.Claims.(jwtv5.MapClaims); ok {

		if org, found := claims["organization_name"]; found {
			details.OrganizationName, _ = org.(string)
		}
		if role, found := claims["role"]; found {
			details.Role, _ = role.(string)
		}
		if email, found := claims["email"]; found {
			details.Email, _ = email.(string)
		}
		if sub, found := claims["sub"]; found {
			details.Subject, _ = sub.(string)
		}

		if traits, found := claims["traits"].(map[string]interface{}); found {
			if details.OrganizationName == "" {
				if org, ok := traits["organization_name"].(string); ok {
					details.OrganizationName = org
				}
			}
			if details.Role == "" {
				if role, ok := traits["role"].(string); ok {
					details.Role = role
				}
			}
			if details.Email == "" {
				if email, ok := traits["email"].(string); ok {
					details.Email = email
				}
			}
		}
	}

	if details.OrganizationName == "" {
		return nil, errors.New("organization_name claim not found in token")
	}
	if details.Role == "" {
		return nil, errors.New("role claim not found in token")
	}

	return details, nil
}
