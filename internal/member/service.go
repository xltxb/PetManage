package member

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"github.com/xltxb/PetManage/internal/pet"
	"github.com/xltxb/PetManage/pkg/apperrors"
	cryptopkg "github.com/xltxb/PetManage/pkg/crypto"
)

// Member represents a member record.
type Member struct {
	ID           int64      `json:"id"`
	MerchantID   int64      `json:"merchant_id"`
	CardNo       string     `json:"card_no"`
	Name         string     `json:"name"`
	Phone        string     `json:"phone"`
	Wechat       string     `json:"wechat"`
	Gender       string     `json:"gender"`
	Birthday     *string    `json:"birthday,omitempty"`
	Address      string     `json:"address"`
	Remark       string     `json:"remark"`
	BalanceCents         int64      `json:"balance_cents"`
	PrincipalBalanceCents int64     `json:"principal_balance_cents"`
	BonusBalanceCents    int64     `json:"bonus_balance_cents"`
	Points               int        `json:"points"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}

// ConsumptionRecord is a simplified order record for the member detail page.
type ConsumptionRecord struct {
	OrderID   int64     `json:"order_id"`
	TotalCents int      `json:"total_cents"`
	PaidCents int      `json:"paid_cents"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// MemberDetail includes member info, bound pets, balance, points, and consumption records.
type MemberDetail struct {
	Member             Member              `json:"member"`
	Pets               []pet.Pet           `json:"pets"`
	ConsumptionRecords []ConsumptionRecord `json:"consumption_records"`
	TotalOrders        int                 `json:"total_orders"`
}

// CreateMemberRequest is the request body for creating a member.
type CreateMemberRequest struct {
	Name     string  `json:"name"`
	Phone    string  `json:"phone"`
	Wechat   string  `json:"wechat"`
	Gender   string  `json:"gender"`
	Birthday *string `json:"birthday"`
	Address  string  `json:"address"`
	Remark   string  `json:"remark"`
}

// UpdateMemberRequest is the request body for updating a member (partial).
type UpdateMemberRequest struct {
	Name     *string `json:"name"`
	Phone    *string `json:"phone"`
	Wechat   *string `json:"wechat"`
	Gender   *string `json:"gender"`
	Birthday *string `json:"birthday"`
	Address  *string `json:"address"`
	Remark   *string `json:"remark"`
}

// ListParams holds optional filters and pagination for listing members.
type ListParams struct {
	Status   string
	Keyword  string
	TagID    int64
	Page     int
	PageSize int
}

// ListResult wraps the members list with pagination info.
type ListResult struct {
	Members  []Member `json:"members"`
	Total    int      `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
}

// BatchImportResult is the result of a batch import operation.
type BatchImportResult struct {
	TotalRows   int              `json:"total_rows"`
	SuccessRows int              `json:"success_rows"`
	FailedRows  int              `json:"failed_rows"`
	Errors      []BatchRowError  `json:"errors"`
}

// BatchRowError represents a single row validation/import error.
type BatchRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// Service provides member management operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new member Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

const memberColumns = `id, merchant_id, card_no, name, phone, wechat, gender, birthday, address, remark, balance_cents, COALESCE(principal_balance_cents,0), COALESCE(bonus_balance_cents,0), points, status, created_at, updated_at`

// phoneHash returns a SHA-256 hex hash for deterministic phone lookup.
func phoneHash(phone string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(phone)))
	return hex.EncodeToString(h[:])
}

// encryptPhone encrypts a phone number and returns its hash.
func encryptPhone(phone string) (encrypted string, phash string, err error) {
	phone = strings.TrimSpace(phone)
	encrypted, err = cryptopkg.Encrypt(phone)
	if err != nil {
		return "", "", err
	}
	return encrypted, phoneHash(phone), nil
}

// decryptPhone attempts to decrypt a phone value. Falls back to plaintext.
func decryptPhone(raw string) string {
	return cryptopkg.TryDecrypt(raw)
}

func scanMemberRow(row *sql.Row) (*Member, error) {
	m := &Member{}
	err := row.Scan(
		&m.ID, &m.MerchantID, &m.CardNo, &m.Name, &m.Phone, &m.Wechat,
		&m.Gender, &m.Birthday, &m.Address, &m.Remark, &m.BalanceCents,
		&m.PrincipalBalanceCents, &m.BonusBalanceCents,
		&m.Points, &m.Status, &m.CreatedAt, &m.UpdatedAt,
	)
	if err == nil {
		m.Phone = decryptPhone(m.Phone)
	}
	return m, err
}

func scanMemberRows(rows *sql.Rows) (*Member, error) {
	m := &Member{}
	err := rows.Scan(
		&m.ID, &m.MerchantID, &m.CardNo, &m.Name, &m.Phone, &m.Wechat,
		&m.Gender, &m.Birthday, &m.Address, &m.Remark, &m.BalanceCents,
		&m.PrincipalBalanceCents, &m.BonusBalanceCents,
		&m.Points, &m.Status, &m.CreatedAt, &m.UpdatedAt,
	)
	if err == nil {
		m.Phone = decryptPhone(m.Phone)
	}
	return m, err
}

// generateCardNo generates a member card number: M + YYYYMMDD + 4-digit serial.
func (s *Service) generateCardNo(ctx context.Context, merchantID int64) (string, error) {
	today := time.Now().Format("20060102")
	prefix := "M" + today

	var maxNo sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT MAX(card_no) FROM members
		 WHERE card_no LIKE $1`,
		prefix+"%",
	).Scan(&maxNo)
	if err != nil {
		return "", err
	}

	serial := 1
	if maxNo.Valid && len(maxNo.String) == len(prefix)+4 {
		if n, err := strconv.Atoi(maxNo.String[len(prefix):]); err == nil {
			serial = n + 1
		}
	}
	if serial > 9999 {
		serial = 1
	}

	return fmt.Sprintf("%s%04d", prefix, serial), nil
}

// Create creates a new member for a merchant.
func (s *Service) Create(ctx context.Context, merchantID int64, req CreateMemberRequest) (*Member, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperrors.NewValidationError("member name is required")
	}
	if strings.TrimSpace(req.Phone) == "" {
		return nil, apperrors.NewValidationError("member phone is required")
	}
	if req.Gender != "" && req.Gender != "M" && req.Gender != "F" && req.Gender != "O" {
		return nil, apperrors.NewValidationError("gender must be M, F, or O")
	}

	cardNo, err := s.generateCardNo(ctx, merchantID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to generate card number", err)
	}

	phone := strings.TrimSpace(req.Phone)
	encPhone, phash, err := encryptPhone(phone)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to encrypt phone", err)
	}

	m, err := scanMemberRow(s.db.QueryRowContext(ctx,
		`INSERT INTO members (merchant_id, card_no, name, phone, phone_hash, wechat, gender, birthday, address, remark)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING `+memberColumns,
		merchantID, cardNo, strings.TrimSpace(req.Name), encPhone, phash,
		req.Wechat, req.Gender, req.Birthday, req.Address, req.Remark,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to create member", err)
	}
	return m, nil
}

// GetByID returns a single member by ID, scoped to a merchant.
func (s *Service) GetByID(ctx context.Context, memberID, merchantID int64) (*Member, error) {
	m, err := scanMemberRow(s.db.QueryRowContext(ctx,
		`SELECT `+memberColumns+` FROM members
		 WHERE id = $1 AND merchant_id = $2 AND deleted_at IS NULL`,
		memberID, merchantID,
	))
	if err == sql.ErrNoRows {
		return nil, apperrors.NewNotFoundError("member not found")
	}
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get member", err)
	}
	return m, nil
}

// GetDetail returns member detail with bound pets, balance, points, and consumption records.
func (s *Service) GetDetail(ctx context.Context, memberID, merchantID int64) (*MemberDetail, error) {
	m, err := s.GetByID(ctx, memberID, merchantID)
	if err != nil {
		return nil, err
	}

	// Query consumption records (recent orders for this member).
	consumptionRecords := make([]ConsumptionRecord, 0)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, total_cents, paid_cents, status, created_at
		 FROM orders
		 WHERE merchant_id = $1 AND member_id = $2
		 ORDER BY created_at DESC LIMIT 50`,
		merchantID, memberID,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get consumption records", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cr ConsumptionRecord
		if err := rows.Scan(&cr.OrderID, &cr.TotalCents, &cr.PaidCents, &cr.Status, &cr.CreatedAt); err != nil {
			return nil, apperrors.NewInternalError("failed to scan consumption record", err)
		}
		consumptionRecords = append(consumptionRecords, cr)
	}
	if consumptionRecords == nil {
		consumptionRecords = make([]ConsumptionRecord, 0)
	}

	// Count total orders.
	var totalOrders int
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM orders
		 WHERE merchant_id = $1 AND member_id = $2`,
		merchantID, memberID,
	).Scan(&totalOrders)

	// Query bound pets.
	petSvc := pet.NewService(s.db)
	pets, _ := petSvc.ListByMember(ctx, merchantID, memberID)

	return &MemberDetail{
		Member:             *m,
		Pets:               pets,
		ConsumptionRecords: consumptionRecords,
		TotalOrders:        totalOrders,
	}, nil
}

// List returns members for a merchant with optional filters and pagination.
func (s *Service) List(ctx context.Context, merchantID int64, params ListParams) (*ListResult, error) {
	page := params.Page
	if page < 1 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	args := []interface{}{merchantID}
	argIdx := 2

	where := "WHERE merchant_id = $1 AND deleted_at IS NULL"
	if params.Status != "" {
		where += " AND status = $" + strconv.Itoa(argIdx)
		args = append(args, params.Status)
		argIdx++
	}
	if params.Keyword != "" {
		phash := phoneHash(params.Keyword)
		where += " AND (name ILIKE $" + strconv.Itoa(argIdx) + " OR phone_hash = $" + strconv.Itoa(argIdx+1) + ")"
		args = append(args, "%"+params.Keyword+"%", phash)
		argIdx += 2
	}
		if params.TagID > 0 {
			where += " AND id IN (SELECT member_id FROM member_tag_relations WHERE tag_id = $" + strconv.Itoa(argIdx) + ")"
			args = append(args, params.TagID)
			argIdx++
		}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM members `+where,
		args...,
	).Scan(&total)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to count members", err)
	}

	offset := (page - 1) * pageSize
	queryArgs := append(args, pageSize, offset)

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+memberColumns+` FROM members `+where+
			` ORDER BY created_at DESC LIMIT $`+strconv.Itoa(argIdx)+` OFFSET $`+strconv.Itoa(argIdx+1),
		queryArgs...,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list members", err)
	}
	defer rows.Close()

	members := make([]Member, 0)
	for rows.Next() {
		m, err := scanMemberRows(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan member", err)
		}
		members = append(members, *m)
	}

	if members == nil {
		members = make([]Member, 0)
	}

	return &ListResult{
		Members:  members,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// SearchByPhone searches members by phone number using hash-based exact match.
func (s *Service) SearchByPhone(ctx context.Context, merchantID int64, phone string) ([]Member, error) {
	if strings.TrimSpace(phone) == "" {
		return make([]Member, 0), nil
	}

	phash := phoneHash(strings.TrimSpace(phone))
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+memberColumns+` FROM members
		 WHERE merchant_id = $1 AND phone_hash = $2 AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT 20`,
		merchantID, phash,
	)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to search members", err)
	}
	defer rows.Close()

	members := make([]Member, 0)
	for rows.Next() {
		m, err := scanMemberRows(rows)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to scan member", err)
		}
		members = append(members, *m)
	}

	if members == nil {
		members = make([]Member, 0)
	}

	return members, nil
}

// Update updates member fields. Only non-nil fields in the request are applied.
func (s *Service) Update(ctx context.Context, memberID, merchantID int64, req UpdateMemberRequest) (*Member, error) {
	existing, err := s.GetByID(ctx, memberID, merchantID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			return nil, apperrors.NewValidationError("member name is required")
		}
		existing.Name = strings.TrimSpace(*req.Name)
	}
	encPhone := existing.Phone // already decrypted raw phone
	var phash *string
	if req.Phone != nil {
		if strings.TrimSpace(*req.Phone) == "" {
			return nil, apperrors.NewValidationError("member phone is required")
		}
		phone := strings.TrimSpace(*req.Phone)
		var enc string
		var h string
		enc, h, err := encryptPhone(phone)
		if err != nil {
			return nil, apperrors.NewInternalError("failed to encrypt phone", err)
		}
		encPhone = enc
		phash = &h
		existing.Phone = phone
	}
	if req.Wechat != nil {
		existing.Wechat = *req.Wechat
	}
	if req.Gender != nil {
		if *req.Gender != "" && *req.Gender != "M" && *req.Gender != "F" && *req.Gender != "O" {
			return nil, apperrors.NewValidationError("gender must be M, F, or O")
		}
		existing.Gender = *req.Gender
	}
	if req.Birthday != nil {
		existing.Birthday = req.Birthday
	}
	if req.Address != nil {
		existing.Address = *req.Address
	}
	if req.Remark != nil {
		existing.Remark = *req.Remark
	}

	if phash != nil {
		m, err := scanMemberRow(s.db.QueryRowContext(ctx,
			`UPDATE members SET name=$1, phone=$2, phone_hash=$3, wechat=$4, gender=$5, birthday=$6, address=$7, remark=$8, updated_at=NOW()
			 WHERE id=$9 AND merchant_id=$10 AND deleted_at IS NULL
			 RETURNING `+memberColumns,
			existing.Name, encPhone, *phash, existing.Wechat, existing.Gender,
			existing.Birthday, existing.Address, existing.Remark,
			memberID, merchantID,
		))
		if err != nil {
			return nil, apperrors.NewInternalError("failed to update member", err)
		}
		return m, nil
	}

	m, err := scanMemberRow(s.db.QueryRowContext(ctx,
		`UPDATE members SET name=$1, wechat=$2, gender=$3, birthday=$4, address=$5, remark=$6, updated_at=NOW()
		 WHERE id=$7 AND merchant_id=$8 AND deleted_at IS NULL
		 RETURNING `+memberColumns,
		existing.Name, existing.Wechat, existing.Gender,
		existing.Birthday, existing.Address, existing.Remark,
		memberID, merchantID,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to update member", err)
	}
	return m, nil
}

// ToggleStatus toggles a member between active and inactive.
func (s *Service) ToggleStatus(ctx context.Context, memberID, merchantID int64) (*Member, error) {
	existing, err := s.GetByID(ctx, memberID, merchantID)
	if err != nil {
		return nil, err
	}

	newStatus := "inactive"
	if existing.Status == "inactive" {
		newStatus = "active"
	}

	m, err := scanMemberRow(s.db.QueryRowContext(ctx,
		`UPDATE members SET status=$1, updated_at=NOW()
		 WHERE id=$2 AND merchant_id=$3 AND deleted_at IS NULL
		 RETURNING `+memberColumns,
		newStatus, memberID, merchantID,
	))
	if err != nil {
		return nil, apperrors.NewInternalError("failed to toggle member status", err)
	}
	return m, nil
}

// batchColumns defines the expected Excel column order.
var batchColumns = []string{"姓名", "手机号", "微信", "性别", "生日", "地址", "备注"}

// BatchImport parses an Excel file and imports members row by row.
func (s *Service) BatchImport(ctx context.Context, merchantID int64, reader io.Reader) (*BatchImportResult, error) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, apperrors.NewValidationError("failed to parse Excel file: " + err.Error())
	}
	defer f.Close()

	// Get the first sheet.
	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, apperrors.NewValidationError("failed to read sheet rows: " + err.Error())
	}

	result := &BatchImportResult{
		Errors: make([]BatchRowError, 0),
	}

	if len(rows) == 0 {
		return nil, apperrors.NewValidationError("Excel file is empty")
	}

	// Skip header row.
	dataRows := rows[1:]
	result.TotalRows = len(dataRows)

	for i, row := range dataRows {
		rowNum := i + 2 // 1-indexed + header

		// Ensure row has enough columns.
		req, errMsg := parseBatchRow(row)
		if errMsg != "" {
			result.FailedRows++
			result.Errors = append(result.Errors, BatchRowError{Row: rowNum, Message: errMsg})
			continue
		}

		_, err := s.Create(ctx, merchantID, req)
		if err != nil {
			result.FailedRows++
			msg := "import failed"
			if appErr, ok := err.(*apperrors.AppError); ok {
				msg = appErr.Message
			}
			result.Errors = append(result.Errors, BatchRowError{Row: rowNum, Message: msg})
			continue
		}

		result.SuccessRows++
	}

	if result.Errors == nil {
		result.Errors = make([]BatchRowError, 0)
	}

	return result, nil
}

// parseBatchRow maps an Excel row to a CreateMemberRequest.
func parseBatchRow(row []string) (CreateMemberRequest, string) {
	get := func(idx int) string {
		if idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	name := get(0)
	phone := get(1)
	wechat := get(2)
	gender := get(3)
	birthday := get(4)
	address := get(5)
	remark := get(6)

	// Silently skip empty rows.
	if name == "" && phone == "" {
		return CreateMemberRequest{}, "empty row"
	}

	if name == "" {
		return CreateMemberRequest{}, "姓名 is required"
	}
	if phone == "" {
		return CreateMemberRequest{}, "手机号 is required"
	}
	if gender != "" && gender != "M" && gender != "F" && gender != "O" {
		return CreateMemberRequest{}, "性别 must be M, F, or O, got: " + gender
	}

	req := CreateMemberRequest{
		Name:    name,
		Phone:   phone,
		Wechat:  wechat,
		Gender:  gender,
		Address: address,
		Remark:  remark,
	}
	if birthday != "" {
		req.Birthday = &birthday
	}

	return req, ""
}

// BulkCreateJSON accepts JSON array input for batch member creation (used by API).
func (s *Service) BulkCreateJSON(ctx context.Context, merchantID int64, body io.Reader) (*BatchImportResult, error) {
	var reqs []CreateMemberRequest
	if err := json.NewDecoder(body).Decode(&reqs); err != nil {
		return nil, apperrors.NewValidationError("invalid JSON: expected array of members")
	}
	if len(reqs) == 0 {
		return nil, apperrors.NewValidationError("at least one member is required")
	}

	result := &BatchImportResult{
		TotalRows: len(reqs),
		Errors:    make([]BatchRowError, 0),
	}

	for i, req := range reqs {
		rowNum := i + 1
		_, err := s.Create(ctx, merchantID, req)
		if err != nil {
			result.FailedRows++
			msg := "import failed"
			if appErr, ok := err.(*apperrors.AppError); ok {
				msg = appErr.Message
			}
			result.Errors = append(result.Errors, BatchRowError{Row: rowNum, Message: msg})
			continue
		}
		result.SuccessRows++
	}

	if result.Errors == nil {
		result.Errors = make([]BatchRowError, 0)
	}

	return result, nil
}
