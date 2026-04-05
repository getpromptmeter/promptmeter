package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/promptmeter/promptmeter/server/internal/domain"
)

// PostgresStore implements APIKeyStore and PriceStore using PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new PostgreSQL store with a connection pool.
func NewPostgresStore(ctx context.Context, connStr string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return &PostgresStore{pool: pool}, nil
}

// Close closes the connection pool.
func (s *PostgresStore) Close() {
	s.pool.Close()
}

// GetAPIKeyByHash looks up a non-revoked API key by its SHA-256 hash.
func (s *PostgresStore) GetAPIKeyByHash(ctx context.Context, keyHash string) (*domain.APIKey, error) {
	query := `
		SELECT ak.id, ak.org_id, ak.key_prefix, ak.key_hash, ak.name, ak.scopes,
		       ak.last_used_at, ak.created_at, ak.revoked_at, o.tier
		FROM api_keys ak
		JOIN organizations o ON o.id = ak.org_id
		WHERE ak.key_hash = $1
	`
	row := s.pool.QueryRow(ctx, query, keyHash)

	var key domain.APIKey
	err := row.Scan(
		&key.ID, &key.OrgID, &key.KeyPrefix, &key.KeyHash, &key.Name,
		&key.Scopes, &key.LastUsedAt, &key.CreatedAt, &key.RevokedAt, &key.Tier,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres: get api key: %w", err)
	}
	return &key, nil
}

// GetAllPrices returns all model prices from the model_prices table.
func (s *PostgresStore) GetAllPrices(ctx context.Context) ([]domain.ModelPrice, error) {
	query := `
		SELECT id, provider, model_name, input_price_per_million,
		       output_price_per_million, effective_from, created_at
		FROM model_prices
		ORDER BY provider, model_name, effective_from DESC
	`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("postgres: get prices: %w", err)
	}
	defer rows.Close()

	var prices []domain.ModelPrice
	for rows.Next() {
		var p domain.ModelPrice
		if err := rows.Scan(
			&p.ID, &p.Provider, &p.ModelName, &p.InputPricePerMillion,
			&p.OutputPricePerMillion, &p.EffectiveFrom, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres: scan price: %w", err)
		}
		prices = append(prices, p)
	}
	return prices, rows.Err()
}

// Pool returns the underlying connection pool for use in tests or custom queries.
func (s *PostgresStore) Pool() *pgxpool.Pool {
	return s.pool
}

// --- UserStore implementation ---

// CreateOrGetUser finds a user by email or creates a new one.
func (s *PostgresStore) CreateOrGetUser(ctx context.Context, email, name, avatarURL string) (*domain.User, bool, error) {
	// Try to find existing user
	var user domain.User
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, name, avatar_url, created_at FROM users WHERE email = $1`,
		email,
	).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.CreatedAt)

	if err == nil {
		return &user, false, nil
	}
	if err != pgx.ErrNoRows {
		return nil, false, fmt.Errorf("postgres: get user by email: %w", err)
	}

	// Create new user
	err = s.pool.QueryRow(ctx,
		`INSERT INTO users (email, name, avatar_url) VALUES ($1, $2, $3)
		 RETURNING id, email, name, avatar_url, created_at`,
		email, name, avatarURL,
	).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.CreatedAt)
	if err != nil {
		return nil, false, fmt.Errorf("postgres: create user: %w", err)
	}
	return &user, true, nil
}

// GetUserByID returns a user by ID.
func (s *PostgresStore) GetUserByID(ctx context.Context, userID string) (*domain.User, error) {
	var user domain.User
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, name, avatar_url, created_at FROM users WHERE id = $1`,
		userID,
	).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres: get user by id: %w", err)
	}
	return &user, nil
}

// --- RefreshTokenStore implementation ---

// CreateRefreshToken stores a new refresh token hash.
func (s *PostgresStore) CreateRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) (*domain.RefreshToken, error) {
	var rt domain.RefreshToken
	err := s.pool.QueryRow(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3)
		 RETURNING id, user_id, token_hash, expires_at, revoked_at, created_at`,
		userID, tokenHash, expiresAt,
	).Scan(&rt.ID, &rt.UserID, &rt.TokenHash, &rt.ExpiresAt, &rt.RevokedAt, &rt.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("postgres: create refresh token: %w", err)
	}
	return &rt, nil
}

// GetRefreshTokenByHash looks up a refresh token by its SHA-256 hash.
func (s *PostgresStore) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	var rt domain.RefreshToken
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
		 FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash,
	).Scan(&rt.ID, &rt.UserID, &rt.TokenHash, &rt.ExpiresAt, &rt.RevokedAt, &rt.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres: get refresh token: %w", err)
	}
	return &rt, nil
}

// RevokeRefreshToken marks a token as revoked.
func (s *PostgresStore) RevokeRefreshToken(ctx context.Context, tokenID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1`,
		tokenID,
	)
	if err != nil {
		return fmt.Errorf("postgres: revoke refresh token: %w", err)
	}
	return nil
}

// RevokeAllUserTokens revokes all refresh tokens for a user.
func (s *PostgresStore) RevokeAllUserTokens(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("postgres: revoke all user tokens: %w", err)
	}
	return nil
}

// --- OrgStore implementation ---

// GetOrg returns the organization for the given ID.
func (s *PostgresStore) GetOrg(ctx context.Context, orgID string) (*domain.Organization, error) {
	var org domain.Organization
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, slug, tier, timezone, pii_enabled, slack_webhook_url,
		        stripe_customer_id, stripe_subscription_id, created_at, updated_at
		 FROM organizations WHERE id = $1`,
		orgID,
	).Scan(&org.ID, &org.Name, &org.Slug, &org.Tier,
		&org.Timezone, &org.PIIEnabled, &org.SlackWebhookURL,
		&org.StripeCustomerID, &org.StripeSubscriptionID,
		&org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres: get org: %w", err)
	}
	return &org, nil
}

// UpdateOrg updates mutable organization fields.
func (s *PostgresStore) UpdateOrg(ctx context.Context, org *domain.Organization) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE organizations
		 SET name = $1, timezone = $2, pii_enabled = $3, slack_webhook_url = $4, updated_at = now()
		 WHERE id = $5`,
		org.Name, org.Timezone, org.PIIEnabled, org.SlackWebhookURL, org.ID,
	)
	if err != nil {
		return fmt.Errorf("postgres: update org: %w", err)
	}
	return nil
}

// GetOrgByUserID returns the organization for a user.
func (s *PostgresStore) GetOrgByUserID(ctx context.Context, userID string) (*domain.Organization, error) {
	var org domain.Organization
	err := s.pool.QueryRow(ctx,
		`SELECT o.id, o.name, o.slug, o.tier, o.timezone, o.pii_enabled, o.slack_webhook_url,
		        o.stripe_customer_id, o.stripe_subscription_id, o.created_at, o.updated_at
		 FROM organizations o
		 JOIN org_members om ON om.org_id = o.id
		 WHERE om.user_id = $1
		 LIMIT 1`,
		userID,
	).Scan(&org.ID, &org.Name, &org.Slug, &org.Tier,
		&org.Timezone, &org.PIIEnabled, &org.SlackWebhookURL,
		&org.StripeCustomerID, &org.StripeSubscriptionID,
		&org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres: get org by user: %w", err)
	}
	return &org, nil
}

// CreateOrg creates a new organization.
func (s *PostgresStore) CreateOrg(ctx context.Context, name, slug string) (*domain.Organization, error) {
	var org domain.Organization
	err := s.pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug, tier) VALUES ($1, $2, 'free')
		 RETURNING id, name, slug, tier, timezone, pii_enabled, slack_webhook_url,
		           stripe_customer_id, stripe_subscription_id, created_at, updated_at`,
		name, slug,
	).Scan(&org.ID, &org.Name, &org.Slug, &org.Tier,
		&org.Timezone, &org.PIIEnabled, &org.SlackWebhookURL,
		&org.StripeCustomerID, &org.StripeSubscriptionID,
		&org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("postgres: create org: %w", err)
	}
	return &org, nil
}

// AddOrgMember adds a user to an organization with the given role.
func (s *PostgresStore) AddOrgMember(ctx context.Context, orgID, userID, role string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role) VALUES ($1, $2, $3)
		 ON CONFLICT (org_id, user_id) DO NOTHING`,
		orgID, userID, role,
	)
	if err != nil {
		return fmt.Errorf("postgres: add org member: %w", err)
	}
	return nil
}

// CountOrgs returns the total number of organizations.
func (s *PostgresStore) CountOrgs(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM organizations`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("postgres: count orgs: %w", err)
	}
	return count, nil
}

// --- ProjectStore implementation ---

// ListProjects returns all active projects for an organization.
func (s *PostgresStore) ListProjects(ctx context.Context, orgID string) ([]domain.Project, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, org_id, name, slug, description, pii_enabled, is_default,
		        created_at, updated_at, archived_at
		 FROM projects
		 WHERE org_id = $1 AND archived_at IS NULL
		 ORDER BY is_default DESC, name ASC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: list projects: %w", err)
	}
	defer rows.Close()

	var projects []domain.Project
	for rows.Next() {
		var p domain.Project
		if err := rows.Scan(
			&p.ID, &p.OrgID, &p.Name, &p.Slug, &p.Description,
			&p.PIIEnabled, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt, &p.ArchivedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres: scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// CreateProject creates a new project.
func (s *PostgresStore) CreateProject(ctx context.Context, project *domain.Project) error {
	err := s.pool.QueryRow(ctx,
		`INSERT INTO projects (org_id, name, slug, description, pii_enabled, is_default)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at, updated_at`,
		project.OrgID, project.Name, project.Slug, project.Description,
		project.PIIEnabled, project.IsDefault,
	).Scan(&project.ID, &project.CreatedAt, &project.UpdatedAt)
	if err != nil {
		return fmt.Errorf("postgres: create project: %w", err)
	}
	return nil
}

// --- APIKeyManager implementation ---

// CreateAPIKey creates a new API key in PostgreSQL.
func (s *PostgresStore) CreateAPIKey(ctx context.Context, orgID, name, keyPrefix, keyHash string, scopes []string, projectID *string) (*domain.APIKey, error) {
	var key domain.APIKey
	err := s.pool.QueryRow(ctx,
		`INSERT INTO api_keys (org_id, name, key_prefix, key_hash, scopes, project_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, org_id, key_prefix, key_hash, name, scopes, project_id,
		           last_used_at, created_at, revoked_at`,
		orgID, name, keyPrefix, keyHash, scopes, projectID,
	).Scan(&key.ID, &key.OrgID, &key.KeyPrefix, &key.KeyHash, &key.Name,
		&key.Scopes, &key.ProjectID, &key.LastUsedAt, &key.CreatedAt, &key.RevokedAt)
	if err != nil {
		return nil, fmt.Errorf("postgres: create api key: %w", err)
	}
	return &key, nil
}

// ListAPIKeys returns all API keys for an organization.
func (s *PostgresStore) ListAPIKeys(ctx context.Context, orgID string) ([]domain.APIKey, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT ak.id, ak.org_id, ak.key_prefix, ak.name, ak.scopes, ak.project_id,
		        ak.last_used_at, ak.created_at, ak.revoked_at,
		        p.name as project_name
		 FROM api_keys ak
		 LEFT JOIN projects p ON p.id = ak.project_id
		 WHERE ak.org_id = $1
		 ORDER BY ak.created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: list api keys: %w", err)
	}
	defer rows.Close()

	var keys []domain.APIKey
	for rows.Next() {
		var key domain.APIKey
		var projectName *string
		if err := rows.Scan(
			&key.ID, &key.OrgID, &key.KeyPrefix, &key.Name, &key.Scopes,
			&key.ProjectID, &key.LastUsedAt, &key.CreatedAt, &key.RevokedAt,
			&projectName,
		); err != nil {
			return nil, fmt.Errorf("postgres: scan api key: %w", err)
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

// RevokeAPIKey sets revoked_at on an API key.
func (s *PostgresStore) RevokeAPIKey(ctx context.Context, orgID, keyID string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE api_keys SET revoked_at = now() WHERE id = $1 AND org_id = $2 AND revoked_at IS NULL`,
		keyID, orgID,
	)
	if err != nil {
		return fmt.Errorf("postgres: revoke api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: api key not found or already revoked")
	}
	return nil
}
