package data

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"user-management/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type tenantsRepo struct {
	data *Data
	log  *log.Helper
}

func NewTenantsRepo(data *Data, logger log.Logger) biz.TenantsRepo {
	return &tenantsRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *tenantsRepo) CreateKratosIdentity(ctx context.Context, email, password, firstName, lastName, orgName, orgID, role string) (*biz.User, error) {
	url := fmt.Sprintf("%s/admin/identities", r.data.KratosAdminURL())

	payload := map[string]interface{}{
		"schema_id": "default",
		"state":     "active",
		"traits": map[string]interface{}{
			"email": email,
			"name": map[string]interface{}{
				"first": firstName,
				"last":  lastName,
			},
			"role":              role,
			"organization_name": orgName,
			"tenant_id":         orgID,
		},
		"credentials": map[string]interface{}{
			"password": map[string]interface{}{
				"config": map[string]string{
					"password": password,
				},
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.data.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kratos error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &biz.User{
		IdentityID:       result.ID,
		Email:            email,
		FirstName:        firstName,
		LastName:         lastName,
		OrganizationName: orgName,
		OrganizationID:   orgID,
		Role:             role,
	}, nil
}

func (r *tenantsRepo) ListIdentitiesByOrg(ctx context.Context, orgName, orgID string) ([]*biz.User, error) {
	url := fmt.Sprintf("%s/admin/identities?per_page=500", r.data.KratosAdminURL())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := r.data.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list identities, status: %d", resp.StatusCode)
	}

	var identities []struct {
		ID     string                 `json:"id"`
		Traits map[string]interface{} `json:"traits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&identities); err != nil {
		return nil, err
	}

	var users []*biz.User
	for _, id := range identities {
		tOrgName, _ := id.Traits["organization_name"].(string)
		tOrgID, _ := id.Traits["tenant_id"].(string)

		if tOrgName == orgName && tOrgID == orgID {
			email, _ := id.Traits["email"].(string)
			role, _ := id.Traits["role"].(string)

			var firstName, lastName string
			if nameMap, ok := id.Traits["name"].(map[string]interface{}); ok {
				firstName, _ = nameMap["first"].(string)
				lastName, _ = nameMap["last"].(string)
			}

			users = append(users, &biz.User{
				IdentityID:       id.ID,
				Email:            email,
				FirstName:        firstName,
				LastName:         lastName,
				OrganizationName: tOrgName,
				OrganizationID:   tOrgID,
				Role:             role,
			})
		}
	}

	return users, nil
}

func (r *tenantsRepo) DeleteIdentity(ctx context.Context, id string) error {
	url := fmt.Sprintf("%s/admin/identities/%s", r.data.KratosAdminURL(), id)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := r.data.HTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete identity, status: %d", resp.StatusCode)
	}

	return nil
}

func (r *tenantsRepo) DeletePendingOidcUserByEmail(ctx context.Context, email string) error {
	db := r.data.DB()
	if db == nil {
		return fmt.Errorf("database connection is not configured")
	}

	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil
	}

	const q = `
DELETE FROM login_assist
WHERE email_id = $1
`

	_, err := db.ExecContext(ctx, q, email)
	return err
}

func (r *tenantsRepo) UpdateIdentity(ctx context.Context, id, firstName, lastName, role string) (*biz.User, error) {
	getURL := fmt.Sprintf("%s/admin/identities/%s", r.data.KratosAdminURL(), id)

	getReq, err := http.NewRequestWithContext(ctx, "GET", getURL, nil)
	if err != nil {
		return nil, err
	}

	getResp, err := r.data.HTTPClient().Do(getReq)
	if err != nil {
		return nil, err
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get identity, status: %d", getResp.StatusCode)
	}

	var currentIdentity struct {
		Traits map[string]interface{} `json:"traits"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&currentIdentity); err != nil {
		return nil, err
	}

	orgName, _ := currentIdentity.Traits["organization_name"].(string)
	orgID, _ := currentIdentity.Traits["tenant_id"].(string)
	email, _ := currentIdentity.Traits["email"].(string)
	currentRole, _ := currentIdentity.Traits["role"].(string)

	if firstName == "" {
		if nameMap, ok := currentIdentity.Traits["name"].(map[string]interface{}); ok {
			firstName, _ = nameMap["first"].(string)
		}
	}
	if lastName == "" {
		if nameMap, ok := currentIdentity.Traits["name"].(map[string]interface{}); ok {
			lastName, _ = nameMap["last"].(string)
		}
	}
	if role == "" {
		role = currentRole
	}

	updateURL := fmt.Sprintf("%s/admin/identities/%s", r.data.KratosAdminURL(), id)

	payload := map[string]interface{}{
		"schema_id": "default",
		"state":     "active",
		"traits": map[string]interface{}{
			"email": email,
			"name": map[string]interface{}{
				"first": firstName,
				"last":  lastName,
			},
			"role":              role,
			"organization_name": orgName,
			"tenant_id":         orgID,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", updateURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.data.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kratos update error %d: %s", resp.StatusCode, string(body))
	}

	return &biz.User{
		IdentityID:       id,
		Email:            email,
		FirstName:        firstName,
		LastName:         lastName,
		OrganizationName: orgName,
		OrganizationID:   orgID,
		Role:             role,
	}, nil
}

func (r *tenantsRepo) UpsertPendingOidcUser(ctx context.Context, email, role, orgName, orgID string) error {
	db := r.data.DB()
	if db == nil {
		return fmt.Errorf("database connection is not configured")
	}

	const q = `
INSERT INTO login_assist (email_id, details, status)
VALUES (
		$1::text,
  jsonb_build_object(
			'role', $2::text,
			'organization_name', $3::text,
			'tenant_id', $4::text
  ),
  'pending'
)
ON CONFLICT (email_id)
DO UPDATE SET
  details = EXCLUDED.details,
  status = 'pending'
`

	_, err := db.ExecContext(ctx, q, email, role, orgName, orgID)
	return err
}

func (r *tenantsRepo) CheckAndCompletePendingUser(ctx context.Context, email string) (*biz.PendingLoginUser, bool, error) {
	db := r.data.DB()
	if db == nil {
		return nil, false, fmt.Errorf("database connection is not configured")
	}

	const q = `
WITH matched AS (
  SELECT email_id, details
  FROM login_assist
  WHERE status = 'pending' AND email_id = $1
  FOR UPDATE
), updated AS (
  UPDATE login_assist l
  SET status = 'completed'
  FROM matched m
  WHERE l.email_id = m.email_id
  RETURNING m.email_id AS email_id, m.details AS details
)
SELECT email_id, details
FROM updated
`

	var emailID string
	var detailsRaw []byte
	err := db.QueryRowContext(ctx, q, email).Scan(&emailID, &detailsRaw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	var details struct {
		Role             string `json:"role"`
		OrganizationName string `json:"organization_name"`
		TenantID         string `json:"tenant_id"`
	}
	if err := json.Unmarshal(detailsRaw, &details); err != nil {
		return nil, false, err
	}

	return &biz.PendingLoginUser{
		Email:            emailID,
		Role:             details.Role,
		OrganizationName: details.OrganizationName,
		OrganizationID:   details.TenantID,
	}, true, nil
}
