package authn

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/homesignal-io/homesignal-home-assistant-app/backend/internal/domain/ports"
)

type PostgresAuthRepository struct {
	DB *sql.DB
}

func (r PostgresAuthRepository) GetUserByCognitoSub(ctx context.Context, cognitoSub string) (ports.UserSubject, error) {
	if r.DB == nil {
		return ports.UserSubject{}, fmt.Errorf("auth database is required")
	}
	cognitoSub = strings.TrimSpace(cognitoSub)
	if cognitoSub == "" {
		return ports.UserSubject{}, fmt.Errorf("cognito subject is required")
	}

	var user ports.UserSubject
	err := r.DB.QueryRowContext(ctx, `
SELECT user_id, cognito_sub, email, status
FROM users
WHERE cognito_sub = $1
  AND deleted_at IS NULL
`, cognitoSub).Scan(&user.ID, &user.CognitoSub, &user.Email, &user.Status)
	if err != nil {
		return ports.UserSubject{}, fmt.Errorf("get user by cognito subject: %w", err)
	}
	return user, nil
}

func (r PostgresAuthRepository) ListPermissionKeys(ctx context.Context, userID ports.UserID, accountID ports.AccountID, siteID ports.SiteID) ([]string, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("auth database is required")
	}
	if strings.TrimSpace(string(userID)) == "" {
		return nil, fmt.Errorf("user id is required")
	}

	rows, err := r.DB.QueryContext(ctx, `
WITH requested_scope AS (
  SELECT NULLIF($2, '') AS account_id, NULLIF($3, '') AS site_id
),
membership_permissions AS (
  SELECT rp.action
  FROM account_memberships am
  JOIN roles r ON r.role_id = am.role_id
  JOIN role_permissions rp ON rp.role_id = r.role_id
  JOIN requested_scope scope ON TRUE
  WHERE am.user_id = $1
    AND am.status = 'active'
    AND r.archived_at IS NULL
    AND (scope.account_id IS NULL OR am.account_id = scope.account_id)
),
grant_permissions AS (
  SELECT rp.action
  FROM access_grants ag
  JOIN resources res ON res.resource_id = ag.resource_id
  JOIN roles r ON r.role_id = ag.role_id
  JOIN role_permissions rp ON rp.role_id = r.role_id
  JOIN requested_scope scope ON TRUE
  WHERE ag.subject_type = 'user'
    AND ag.subject_id = $1
    AND ag.status = 'active'
    AND r.archived_at IS NULL
    AND res.status = 'active'
    AND (scope.account_id IS NULL OR res.account_id = scope.account_id)
    AND (
      scope.site_id IS NULL
      OR res.site_id = scope.site_id
      OR (res.resource_type = 'account' AND res.account_id = scope.account_id)
    )
)
SELECT DISTINCT action
FROM (
  SELECT action FROM membership_permissions
  UNION ALL
  SELECT action FROM grant_permissions
) permissions
ORDER BY action
`, userID, accountID, siteID)
	if err != nil {
		return nil, fmt.Errorf("query permission keys: %w", err)
	}
	defer rows.Close()

	var permissions []string
	for rows.Next() {
		var permission string
		if err := rows.Scan(&permission); err != nil {
			return nil, fmt.Errorf("scan permission key: %w", err)
		}
		permissions = append(permissions, permission)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate permission keys: %w", err)
	}
	return permissions, nil
}
