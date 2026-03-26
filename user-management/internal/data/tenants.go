package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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

func (r *tenantsRepo) CreateKratosIdentity(ctx context.Context, email, password, firstName, lastName, orgName, role string) (*biz.User, error) {
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
		Role:             role,
	}, nil
}

func (r *tenantsRepo) ListIdentitiesByOrg(ctx context.Context, orgName string) ([]*biz.User, error) {
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

	var identities []struct {
		ID     string                 `json:"id"`
		Traits map[string]interface{} `json:"traits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&identities); err != nil {
		return nil, err
	}

	var users []*biz.User
	for _, id := range identities {
		if tOrg, ok := id.Traits["organization_name"].(string); ok && tOrg == orgName {
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
				OrganizationName: orgName,
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

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete identity, status: %d", resp.StatusCode)
	}

	return nil
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

	var currentIdentity struct {
		Traits map[string]interface{} `json:"traits"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&currentIdentity); err != nil {
		return nil, err
	}

	orgName, _ := currentIdentity.Traits["organization_name"].(string)
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
		Role:             role,
	}, nil
}
