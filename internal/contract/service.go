package contract

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xltxb/PetManage/pkg/apperrors"
)

// Service handles merchant contract operations.
type Service struct {
	db          *sql.DB
	uploadDir   string
}

// NewService creates a new contract Service.
func NewService(db *sql.DB, uploadDir string) *Service {
	return &Service{db: db, uploadDir: uploadDir}
}

// ContractRecord represents a merchant contract in the database.
type ContractRecord struct {
	ID              int64   `json:"id"`
	MerchantID      int64   `json:"merchant_id"`
	ContractNumber  string  `json:"contract_number"`
	FileName        string  `json:"file_name"`
	FilePath        string  `json:"file_path"`
	FileSize        int64   `json:"file_size"`
	StartDate       string  `json:"start_date"`
	EndDate         string  `json:"end_date"`
	Status          string  `json:"status"`
	IsCurrent       bool    `json:"is_current"`
	PrevContractID  *int64  `json:"prev_contract_id,omitempty"`
	UploadedBy      int64   `json:"uploaded_by"`
	DaysRemaining   *int    `json:"days_remaining,omitempty"`
	ExpiryReminder  bool    `json:"expiry_reminder"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

// UploadRequest holds the contract upload metadata.
type UploadRequest struct {
	ContractNumber string
	StartDate      string
	EndDate        string
	FileHeader     *multipart.FileHeader
}

// UploadResponse is returned after a successful contract upload.
type UploadResponse struct {
	ID             int64  `json:"id"`
	ContractNumber string `json:"contract_number"`
	FileName       string `json:"file_name"`
	Status         string `json:"status"`
	Message        string `json:"message"`
}

// ContractListResponse is the response for listing contracts.
type ContractListResponse struct {
	Contracts []ContractRecord `json:"contracts"`
	Total     int              `json:"total"`
}

// ReminderResponse holds contracts expiring within 7 days.
type ReminderResponse struct {
	Contracts   []ContractRecord `json:"contracts"`
	ReminderMsg string           `json:"reminder_message"`
	Total       int              `json:"total"`
}

// Upload saves a contract file to disk and creates a database record.
func (s *Service) Upload(ctx context.Context, merchantID int64, req UploadRequest, operatorID int64) (*UploadResponse, error) {
	// Validate required fields.
	if strings.TrimSpace(req.ContractNumber) == "" {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInvalidParams,
			Message: "contract_number is required",
		}
	}
	if req.FileHeader == nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInvalidParams,
			Message: "contract file is required",
		}
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInvalidParams,
			Message: "invalid start_date format, expected YYYY-MM-DD",
		}
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInvalidParams,
			Message: "invalid end_date format, expected YYYY-MM-DD",
		}
	}
	if !endDate.After(startDate) {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInvalidParams,
			Message: "end_date must be after start_date",
		}
	}

	// Check merchant exists and is approved.
	var merchantStatus string
	err = s.db.QueryRowContext(ctx,
		`SELECT status FROM merchants WHERE id = $1 AND deleted_at IS NULL`,
		merchantID,
	).Scan(&merchantStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeNotFound,
			Message: "merchant not found",
		}
	}
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to look up merchant",
			Err:     err,
		}
	}
	if merchantStatus != "approved" {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeConflict,
			Message: "only approved merchants can have contracts",
		}
	}

	// Save file to disk.
	merchantDir := filepath.Join(s.uploadDir, fmt.Sprintf("%d", merchantID))
	if err := os.MkdirAll(merchantDir, 0755); err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to create upload directory",
			Err:     err,
		}
	}

	timestamp := time.Now().Unix()
	safeFilename := fmt.Sprintf("%d_%s", timestamp, filepath.Base(req.FileHeader.Filename))
	destPath := filepath.Join(merchantDir, safeFilename)

	src, err := req.FileHeader.Open()
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to open uploaded file",
			Err:     err,
		}
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to create file on disk",
			Err:     err,
		}
	}
	defer dst.Close()

	written, err := io.Copy(dst, src)
	if err != nil {
		os.Remove(destPath)
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to write file to disk",
			Err:     err,
		}
	}

	// Determine contract status based on dates.
	now := time.Now().Truncate(24 * time.Hour)
	status := "active"
	if endDate.Before(now) {
		status = "expired"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		os.Remove(destPath)
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to begin transaction",
			Err:     err,
		}
	}
	defer tx.Rollback()

	// Set all existing contracts for this merchant to not current.
	_, err = tx.ExecContext(ctx,
		`UPDATE merchant_contracts SET is_current = false, updated_at = NOW()
		 WHERE merchant_id = $1 AND is_current = true`,
		merchantID,
	)
	if err != nil {
		os.Remove(destPath)
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to update existing contracts",
			Err:     err,
		}
	}

	var id int64
	var createdAt time.Time
	err = tx.QueryRowContext(ctx,
		`INSERT INTO merchant_contracts
		 (merchant_id, contract_number, file_name, file_path, file_size, start_date, end_date, status, is_current, uploaded_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, true, $9)
		 RETURNING id, created_at`,
		merchantID, req.ContractNumber, req.FileHeader.Filename, destPath, written,
		req.StartDate, req.EndDate, status, operatorID,
	).Scan(&id, &createdAt)
	if err != nil {
		os.Remove(destPath)
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to create contract record",
			Err:     err,
		}
	}

	// Record operation log.
	detail, _ := json.Marshal(map[string]interface{}{
		"contract_number": req.ContractNumber,
		"start_date":      req.StartDate,
		"end_date":        req.EndDate,
	})
	_, err = tx.ExecContext(ctx,
		`INSERT INTO operation_logs (user_id, action, target_type, target_id, detail)
		 VALUES ($1, 'upload_contract', 'contract', $2, $3)`,
		operatorID, id, detail,
	)
	if err != nil {
		os.Remove(destPath)
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to record operation log",
			Err:     err,
		}
	}

	if err := tx.Commit(); err != nil {
		os.Remove(destPath)
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to commit contract upload",
			Err:     err,
		}
	}

	_ = createdAt

	return &UploadResponse{
		ID:             id,
		ContractNumber: req.ContractNumber,
		FileName:       req.FileHeader.Filename,
		Status:         status,
		Message:        "contract uploaded successfully",
	}, nil
}

// List returns all contracts for a merchant, ordered by created_at DESC.
func (s *Service) List(ctx context.Context, merchantID int64) (*ContractListResponse, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, merchant_id, contract_number, file_name, file_path, file_size,
		        start_date, end_date, status, is_current, prev_contract_id, uploaded_by,
		        created_at, updated_at
		 FROM merchant_contracts
		 WHERE merchant_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at DESC`,
		merchantID,
	)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to list contracts",
			Err:     err,
		}
	}
	defer rows.Close()

	now := time.Now().Truncate(24 * time.Hour)
	var contracts []ContractRecord
	for rows.Next() {
		var c ContractRecord
		var startDate, endDate time.Time
		var createdAt, updatedAt time.Time
		var prevContractID sql.NullInt64

		err := rows.Scan(&c.ID, &c.MerchantID, &c.ContractNumber, &c.FileName, &c.FilePath,
			&c.FileSize, &startDate, &endDate, &c.Status, &c.IsCurrent,
			&prevContractID, &c.UploadedBy, &createdAt, &updatedAt)
		if err != nil {
			return nil, &apperrors.AppError{
				Code:    apperrors.CodeInternalError,
				Message: "failed to scan contract row",
				Err:     err,
			}
		}

		c.StartDate = startDate.Format("2006-01-02")
		c.EndDate = endDate.Format("2006-01-02")
		c.CreatedAt = createdAt.Format(time.RFC3339)
		c.UpdatedAt = updatedAt.Format(time.RFC3339)
		if prevContractID.Valid {
			c.PrevContractID = &prevContractID.Int64
		}

		days := int(endDate.Sub(now).Hours() / 24)
		c.DaysRemaining = &days
		c.ExpiryReminder = c.Status == "active" && days >= 0 && days <= 7

		contracts = append(contracts, c)
	}

	if contracts == nil {
		contracts = []ContractRecord{}
	}

	return &ContractListResponse{
		Contracts: contracts,
		Total:     len(contracts),
	}, nil
}

// GetCurrent returns the current active contract for a merchant.
func (s *Service) GetCurrent(ctx context.Context, merchantID int64) (*ContractRecord, error) {
	var c ContractRecord
	var startDate, endDate time.Time
	var createdAt, updatedAt time.Time
	var prevContractID sql.NullInt64

	err := s.db.QueryRowContext(ctx,
		`SELECT id, merchant_id, contract_number, file_name, file_path, file_size,
		        start_date, end_date, status, is_current, prev_contract_id, uploaded_by,
		        created_at, updated_at
		 FROM merchant_contracts
		 WHERE merchant_id = $1 AND is_current = true AND deleted_at IS NULL
		 LIMIT 1`,
		merchantID,
	).Scan(&c.ID, &c.MerchantID, &c.ContractNumber, &c.FileName, &c.FilePath,
		&c.FileSize, &startDate, &endDate, &c.Status, &c.IsCurrent,
		&prevContractID, &c.UploadedBy, &createdAt, &updatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeNotFound,
			Message: "no active contract found for this merchant",
		}
	}
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to retrieve contract",
			Err:     err,
		}
	}

	c.StartDate = startDate.Format("2006-01-02")
	c.EndDate = endDate.Format("2006-01-02")
	c.CreatedAt = createdAt.Format(time.RFC3339)
	c.UpdatedAt = updatedAt.Format(time.RFC3339)
	if prevContractID.Valid {
		c.PrevContractID = &prevContractID.Int64
	}

	now := time.Now().Truncate(24 * time.Hour)
	days := int(endDate.Sub(now).Hours() / 24)
	c.DaysRemaining = &days
	c.ExpiryReminder = c.Status == "active" && days >= 0 && days <= 7

	return &c, nil
}

// Renew uploads a new contract that replaces the current one.
func (s *Service) Renew(ctx context.Context, merchantID int64, req UploadRequest, operatorID int64) (*UploadResponse, error) {
	if strings.TrimSpace(req.ContractNumber) == "" {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInvalidParams,
			Message: "contract_number is required",
		}
	}
	if req.FileHeader == nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInvalidParams,
			Message: "contract file is required",
		}
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInvalidParams,
			Message: "invalid start_date format, expected YYYY-MM-DD",
		}
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInvalidParams,
			Message: "invalid end_date format, expected YYYY-MM-DD",
		}
	}
	if !endDate.After(startDate) {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInvalidParams,
			Message: "end_date must be after start_date",
		}
	}

	// Find the current contract to set as previous.
	var currentContractID int64
	var currentEndDate time.Time
	err = s.db.QueryRowContext(ctx,
		`SELECT id, end_date FROM merchant_contracts
		 WHERE merchant_id = $1 AND is_current = true AND deleted_at IS NULL
		 LIMIT 1`,
		merchantID,
	).Scan(&currentContractID, &currentEndDate)

	if errors.Is(err, sql.ErrNoRows) {
		// No current contract, treat as new upload.
		return s.Upload(ctx, merchantID, req, operatorID)
	}
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to find current contract",
			Err:     err,
		}
	}

	// Save file to disk.
	merchantDir := filepath.Join(s.uploadDir, fmt.Sprintf("%d", merchantID))
	if err := os.MkdirAll(merchantDir, 0755); err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to create upload directory",
			Err:     err,
		}
	}

	timestamp := time.Now().Unix()
	safeFilename := fmt.Sprintf("%d_%s", timestamp, filepath.Base(req.FileHeader.Filename))
	destPath := filepath.Join(merchantDir, safeFilename)

	src, err := req.FileHeader.Open()
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to open uploaded file",
			Err:     err,
		}
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to create file on disk",
			Err:     err,
		}
	}
	defer dst.Close()

	written, err := io.Copy(dst, src)
	if err != nil {
		os.Remove(destPath)
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to write file to disk",
			Err:     err,
		}
	}

	// Determine status.
	now := time.Now().Truncate(24 * time.Hour)
	status := "active"
	if endDate.Before(now) {
		status = "expired"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		os.Remove(destPath)
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to begin transaction",
			Err:     err,
		}
	}
	defer tx.Rollback()

	// Expire the previous contract.
	_, err = tx.ExecContext(ctx,
		`UPDATE merchant_contracts SET is_current = false, status = 'expired', updated_at = NOW()
		 WHERE id = $1`,
		currentContractID,
	)
	if err != nil {
		os.Remove(destPath)
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to expire previous contract",
			Err:     err,
		}
	}

	var id int64
	var createdAt time.Time
	err = tx.QueryRowContext(ctx,
		`INSERT INTO merchant_contracts
		 (merchant_id, contract_number, file_name, file_path, file_size, start_date, end_date, status, is_current, prev_contract_id, uploaded_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, true, $9, $10)
		 RETURNING id, created_at`,
		merchantID, req.ContractNumber, req.FileHeader.Filename, destPath, written,
		req.StartDate, req.EndDate, status, currentContractID, operatorID,
	).Scan(&id, &createdAt)
	if err != nil {
		os.Remove(destPath)
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to create contract record",
			Err:     err,
		}
	}

	// Record operation log.
	detail, _ := json.Marshal(map[string]interface{}{
		"contract_number":   req.ContractNumber,
		"start_date":        req.StartDate,
		"end_date":          req.EndDate,
		"prev_contract_id":  currentContractID,
		"action":            "renewal",
	})
	_, err = tx.ExecContext(ctx,
		`INSERT INTO operation_logs (user_id, action, target_type, target_id, detail)
		 VALUES ($1, 'renew_contract', 'contract', $2, $3)`,
		operatorID, id, detail,
	)
	if err != nil {
		os.Remove(destPath)
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to record operation log",
			Err:     err,
		}
	}

	if err := tx.Commit(); err != nil {
		os.Remove(destPath)
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to commit contract renewal",
			Err:     err,
		}
	}

	_ = createdAt

	return &UploadResponse{
		ID:             id,
		ContractNumber: req.ContractNumber,
		FileName:       req.FileHeader.Filename,
		Status:         status,
		Message:        "contract renewed successfully",
	}, nil
}

// GetReminders returns all contracts expiring within 7 days.
func (s *Service) GetReminders(ctx context.Context) (*ReminderResponse, error) {
	now := time.Now().Truncate(24 * time.Hour)
	sevenDaysLater := now.AddDate(0, 0, 7)

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, merchant_id, contract_number, file_name, file_path, file_size,
		        start_date, end_date, status, is_current, prev_contract_id, uploaded_by,
		        created_at, updated_at
		 FROM merchant_contracts
		 WHERE status = 'active' AND is_current = true AND end_date >= $1 AND end_date <= $2
		   AND deleted_at IS NULL
		 ORDER BY end_date ASC`,
		now.Format("2006-01-02"), sevenDaysLater.Format("2006-01-02"),
	)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to query reminders",
			Err:     err,
		}
	}
	defer rows.Close()

	var contracts []ContractRecord
	for rows.Next() {
		var c ContractRecord
		var startDate, endDate time.Time
		var createdAt, updatedAt time.Time
		var prevContractID sql.NullInt64

		err := rows.Scan(&c.ID, &c.MerchantID, &c.ContractNumber, &c.FileName, &c.FilePath,
			&c.FileSize, &startDate, &endDate, &c.Status, &c.IsCurrent,
			&prevContractID, &c.UploadedBy, &createdAt, &updatedAt)
		if err != nil {
			return nil, &apperrors.AppError{
				Code:    apperrors.CodeInternalError,
				Message: "failed to scan reminder row",
				Err:     err,
			}
		}

		c.StartDate = startDate.Format("2006-01-02")
		c.EndDate = endDate.Format("2006-01-02")
		c.CreatedAt = createdAt.Format(time.RFC3339)
		c.UpdatedAt = updatedAt.Format(time.RFC3339)
		if prevContractID.Valid {
			c.PrevContractID = &prevContractID.Int64
		}

		days := int(endDate.Sub(now).Hours() / 24)
		c.DaysRemaining = &days
		c.ExpiryReminder = true

		contracts = append(contracts, c)
	}

	if contracts == nil {
		contracts = []ContractRecord{}
	}

	msg := fmt.Sprintf("Found %d contract(s) expiring within 7 days", len(contracts))

	return &ReminderResponse{
		Contracts:   contracts,
		ReminderMsg: msg,
		Total:       len(contracts),
	}, nil
}

// GetContractStatusByMerchant returns a map of merchant_id → contract status for use in merchant listing.
func (s *Service) GetContractStatusByMerchant(ctx context.Context, merchantIDs []int64) (map[int64]string, error) {
	if len(merchantIDs) == 0 {
		return map[int64]string{}, nil
	}

	// Build IN clause.
	placeholders := make([]string, len(merchantIDs))
	args := make([]interface{}, len(merchantIDs))
	for i, id := range merchantIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(
		`SELECT merchant_id, status, end_date
		 FROM merchant_contracts
		 WHERE merchant_id IN (%s) AND is_current = true AND deleted_at IS NULL`,
		strings.Join(placeholders, ","),
	)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, &apperrors.AppError{
			Code:    apperrors.CodeInternalError,
			Message: "failed to query contract statuses",
			Err:     err,
		}
	}
	defer rows.Close()

	now := time.Now().Truncate(24 * time.Hour)
	result := make(map[int64]string)
	for rows.Next() {
		var merchantID int64
		var status string
		var endDate time.Time
		if err := rows.Scan(&merchantID, &status, &endDate); err != nil {
			return nil, err
		}
		// Override status if expired.
		if endDate.Before(now) {
			result[merchantID] = "contract_expired"
		} else {
			result[merchantID] = status
		}
	}

	return result, nil
}
