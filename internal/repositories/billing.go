package repositories

import (
	"context"
	"database/sql"
	"errors"
)

var (
	// ErrConflict indicates a request collides with existing data ownership.
	ErrConflict = errors.New("conflict")
)

type BillingRepository struct {
	db *sql.DB
}

func NewBillingRepository(db *sql.DB) *BillingRepository {
	return &BillingRepository{db: db}
}

// ActivateProFromReceipt stores a purchase token and upgrades the user to Pro.
// If the token already belongs to the same user, the operation is idempotent.
func (r *BillingRepository) ActivateProFromReceipt(
	ctx context.Context,
	userID string,
	productID string,
	platform string,
	purchaseToken string,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var existingUserID string
	err = tx.QueryRowContext(
		ctx,
		`SELECT user_id::text
		 FROM billing_receipts
		 WHERE purchase_token = $1
		 LIMIT 1`,
		purchaseToken,
	).Scan(&existingUserID)
	switch {
	case err == nil:
		if existingUserID != userID {
			err = ErrConflict
			return err
		}
	case errors.Is(err, sql.ErrNoRows):
		if _, execErr := tx.ExecContext(
			ctx,
			`INSERT INTO billing_receipts (user_id, product_id, platform, purchase_token)
			 VALUES ($1::uuid, $2, $3, $4)`,
			userID,
			productID,
			platform,
			purchaseToken,
		); execErr != nil {
			err = execErr
			return err
		}
	default:
		return err
	}

	if _, execErr := tx.ExecContext(
		ctx,
		`UPDATE users SET is_pro = true WHERE id = $1::uuid`,
		userID,
	); execErr != nil {
		err = execErr
		return err
	}

	err = tx.Commit()
	return err
}
