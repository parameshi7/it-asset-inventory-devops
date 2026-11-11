package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	mysql "github.com/go-sql-driver/mysql"
)

type Handler struct{ db *sql.DB }

func New(db *sql.DB) *Handler { return &Handler{db: db} }

type User struct {
	ID         int64     `json:"id"`
	EmployeeID string    `json:"employee_id"`
	Name       string    `json:"name" binding:"required"`
	Email      string    `json:"email" binding:"required"`
	Department string    `json:"department"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
type Asset struct {
	ID               int64     `json:"id"`
	AssetTag         string    `json:"asset_tag" binding:"required"`
	AssetName        string    `json:"asset_name" binding:"required"`
	Category         string    `json:"category" binding:"required"`
	Manufacturer     string    `json:"manufacturer"`
	Model            string    `json:"model"`
	SerialNumber     *string   `json:"serial_number"`
	Status           string    `json:"status"`
	AssignedUserID   *int64    `json:"assigned_user_id"`
	AssignedUserName *string   `json:"assigned_user_name"`
	Location         string    `json:"location"`
	PurchaseDate     *string   `json:"purchase_date"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (h *Handler) Health(c *gin.Context) {
	ctx, cancel := contextWithTimeout(c, 2*time.Second)
	defer cancel()
	if err := h.db.PingContext(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "database": "unreachable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "database": "connected"})
}

func (h *Handler) CreateUser(c *gin.Context) {
	var in User
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "invalid JSON request")
		return
	}
	if strings.TrimSpace(in.Name) == "" || validateEmail(in.Email) != nil {
		bad(c, "name and a valid email address are required")
		return
	}
	in.EmployeeID, in.Name, in.Email, in.Department = strings.TrimSpace(in.EmployeeID), strings.TrimSpace(in.Name), strings.TrimSpace(in.Email), strings.TrimSpace(in.Department)
	if in.EmployeeID == "" {
		in.EmployeeID = "EMP-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	if in.Department == "" {
		in.Department = "Unassigned"
	}
	res, err := h.db.ExecContext(c.Request.Context(), `INSERT INTO users(employee_id,name,email,department) VALUES(?,?,?,?)`, in.EmployeeID, in.Name, in.Email, in.Department)
	if duplicate(c, err, "employee ID or email already exists") {
		return
	}
	if err != nil {
		serverError(c, "could not create user")
		return
	}
	id, _ := res.LastInsertId()
	out, err := h.fetchUser(c, id)
	if err != nil {
		serverError(c, "could not retrieve created user")
		return
	}
	c.JSON(http.StatusCreated, out)
}
func (h *Handler) ListUsers(c *gin.Context) {
	rows, err := h.db.QueryContext(c.Request.Context(), `SELECT id,employee_id,name,email,department,created_at,updated_at FROM users ORDER BY id DESC LIMIT 500`)
	if err != nil {
		serverError(c, "could not list users")
		return
	}
	defer rows.Close()
	items := make([]User, 0)
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.EmployeeID, &u.Name, &u.Email, &u.Department, &u.CreatedAt, &u.UpdatedAt); err != nil {
			serverError(c, "could not read users")
			return
		}
		items = append(items, u)
	}
	if rows.Err() != nil {
		serverError(c, "could not read users")
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) GetUser(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	u, err := h.fetchUser(c, id)
	if errors.Is(err, sql.ErrNoRows) {
		notFound(c, "user not found")
		return
	}
	if err != nil {
		serverError(c, "could not retrieve user")
		return
	}
	c.JSON(http.StatusOK, u)
}
func (h *Handler) UpdateUser(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var in User
	if c.ShouldBindJSON(&in) != nil {
		bad(c, "invalid JSON request")
		return
	}
	if strings.TrimSpace(in.Name) == "" || validateEmail(in.Email) != nil {
		bad(c, "name and a valid email address are required")
		return
	}
	current, err := h.fetchUser(c, id)
	if errors.Is(err, sql.ErrNoRows) {
		notFound(c, "user not found")
		return
	}
	if err != nil {
		serverError(c, "could not retrieve user")
		return
	}
	employeeID := strings.TrimSpace(in.EmployeeID)
	if employeeID == "" {
		employeeID = current.EmployeeID
	}
	department := strings.TrimSpace(in.Department)
	if department == "" {
		department = current.Department
	}
	_, err = h.db.ExecContext(c.Request.Context(), `UPDATE users SET employee_id=?,name=?,email=?,department=? WHERE id=?`, employeeID, strings.TrimSpace(in.Name), strings.TrimSpace(in.Email), department, id)
	if duplicate(c, err, "employee ID or email already exists") {
		return
	}
	if err != nil {
		serverError(c, "could not update user")
		return
	}
	out, err := h.fetchUser(c, id)
	if errors.Is(err, sql.ErrNoRows) {
		notFound(c, "user not found")
		return
	}
	if err != nil {
		serverError(c, "could not retrieve user")
		return
	}
	c.JSON(http.StatusOK, out)
}
func (h *Handler) DeleteUser(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var count int
	if err := h.db.QueryRowContext(c.Request.Context(), `SELECT COUNT(*) FROM assets WHERE assigned_user_id=?`, id).Scan(&count); err != nil {
		serverError(c, "could not check assigned assets")
		return
	}
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "user has assigned assets; unassign them before deleting"})
		return
	}
	res, err := h.db.ExecContext(c.Request.Context(), `DELETE FROM users WHERE id=?`, id)
	if err != nil {
		serverError(c, "could not delete user")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		notFound(c, "user not found")
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *Handler) fetchUser(c *gin.Context, id int64) (User, error) {
	var u User
	err := h.db.QueryRowContext(c.Request.Context(), `SELECT id,employee_id,name,email,department,created_at,updated_at FROM users WHERE id=?`, id).Scan(&u.ID, &u.EmployeeID, &u.Name, &u.Email, &u.Department, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (h *Handler) CreateAsset(c *gin.Context) {
	var in Asset
	if c.ShouldBindJSON(&in) != nil {
		bad(c, "asset_tag, asset_name and category are required")
		return
	}
	if in.PurchaseDate != nil && *in.PurchaseDate != "" {
		if _, err := time.Parse("2006-01-02", *in.PurchaseDate); err != nil {
			bad(c, "purchase_date must use YYYY-MM-DD")
			return
		}
	} else {
		in.PurchaseDate = nil
	}
	in.Status = "available"
	var serial any
	if in.SerialNumber != nil && strings.TrimSpace(*in.SerialNumber) != "" {
		serial = strings.TrimSpace(*in.SerialNumber)
	}
	res, err := h.db.ExecContext(c.Request.Context(), `INSERT INTO assets(asset_tag,asset_name,category,manufacturer,model,serial_number,status,location,purchase_date) VALUES(?,?,?,?,?,?,'available',?,?)`, strings.TrimSpace(in.AssetTag), strings.TrimSpace(in.AssetName), strings.TrimSpace(in.Category), strings.TrimSpace(in.Manufacturer), strings.TrimSpace(in.Model), serial, strings.TrimSpace(in.Location), in.PurchaseDate)
	if duplicate(c, err, "asset tag or serial number already exists") {
		return
	}
	if err != nil {
		serverError(c, "could not create asset")
		return
	}
	id, _ := res.LastInsertId()
	out, err := h.fetchAsset(c, id)
	if err != nil {
		serverError(c, "could not retrieve created asset")
		return
	}
	c.JSON(http.StatusCreated, out)
}
func (h *Handler) ListAssets(c *gin.Context) {
	query := `SELECT a.id,a.asset_tag,a.asset_name,a.category,a.manufacturer,a.model,a.serial_number,a.status,a.assigned_user_id,u.name,a.location,a.purchase_date,a.created_at,a.updated_at FROM assets a LEFT JOIN users u ON u.id=a.assigned_user_id WHERE 1=1`
	args := []any{}
	if s := strings.TrimSpace(c.Query("status")); s != "" {
		query += ` AND a.status=?`
		args = append(args, s)
	}
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		like := "%" + q + "%"
		query += ` AND (a.asset_tag LIKE ? OR a.asset_name LIKE ? OR a.serial_number LIKE ? OR a.category LIKE ? OR u.name LIKE ?)`
		args = append(args, like, like, like, like, like)
	}
	query += ` ORDER BY a.id DESC LIMIT 500`
	rows, err := h.db.QueryContext(c.Request.Context(), query, args...)
	if err != nil {
		serverError(c, "could not list assets")
		return
	}
	defer rows.Close()
	items := make([]Asset, 0)
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			serverError(c, "could not read assets")
			return
		}
		items = append(items, a)
	}
	if rows.Err() != nil {
		serverError(c, "could not read assets")
		return
	}
	c.JSON(http.StatusOK, items)
}
func (h *Handler) GetAsset(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	a, err := h.fetchAsset(c, id)
	if errors.Is(err, sql.ErrNoRows) {
		notFound(c, "asset not found")
		return
	}
	if err != nil {
		serverError(c, "could not retrieve asset")
		return
	}
	c.JSON(http.StatusOK, a)
}
func (h *Handler) UpdateAsset(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var in Asset
	if c.ShouldBindJSON(&in) != nil {
		bad(c, "asset_tag, asset_name and category are required")
		return
	}
	if in.Status == "assigned" {
		bad(c, "use the assign endpoint to assign an asset")
		return
	}
	if in.Status == "" {
		in.Status = "available"
	}
	if in.Status != "available" && in.Status != "maintenance" && in.Status != "retired" {
		bad(c, "status must be available, maintenance or retired")
		return
	}
	if in.PurchaseDate != nil && *in.PurchaseDate != "" {
		if _, err := time.Parse("2006-01-02", *in.PurchaseDate); err != nil {
			bad(c, "purchase_date must use YYYY-MM-DD")
			return
		}
	} else {
		in.PurchaseDate = nil
	}
	var serial any
	if in.SerialNumber != nil && strings.TrimSpace(*in.SerialNumber) != "" {
		serial = strings.TrimSpace(*in.SerialNumber)
	}
	res, err := h.db.ExecContext(c.Request.Context(), `UPDATE assets SET asset_tag=?,asset_name=?,category=?,manufacturer=?,model=?,serial_number=?,status=?,location=?,purchase_date=? WHERE id=? AND assigned_user_id IS NULL`, strings.TrimSpace(in.AssetTag), strings.TrimSpace(in.AssetName), strings.TrimSpace(in.Category), strings.TrimSpace(in.Manufacturer), strings.TrimSpace(in.Model), serial, in.Status, strings.TrimSpace(in.Location), in.PurchaseDate, id)
	if duplicate(c, err, "asset tag or serial number already exists") {
		return
	}
	if err != nil {
		serverError(c, "could not update asset")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var exists, assigned int
		if e := h.db.QueryRowContext(c.Request.Context(), `SELECT COUNT(*),COUNT(assigned_user_id) FROM assets WHERE id=?`, id).Scan(&exists, &assigned); e != nil {
			serverError(c, "could not check asset")
			return
		}
		if exists == 0 {
			notFound(c, "asset not found")
			return
		}
		if assigned > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "unassign the asset before editing its details"})
			return
		}
	}
	out, err := h.fetchAsset(c, id)
	if err != nil {
		serverError(c, "could not retrieve updated asset")
		return
	}
	c.JSON(http.StatusOK, out)
}
func (h *Handler) DeleteAsset(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	res, err := h.db.ExecContext(c.Request.Context(), `DELETE FROM assets WHERE id=? AND assigned_user_id IS NULL`, id)
	if err != nil {
		serverError(c, "could not delete asset")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var exists, assigned int
		_ = h.db.QueryRowContext(c.Request.Context(), `SELECT COUNT(*),COUNT(assigned_user_id) FROM assets WHERE id=?`, id).Scan(&exists, &assigned)
		if exists == 0 {
			notFound(c, "asset not found")
		} else {
			c.JSON(http.StatusConflict, gin.H{"error": "unassign the asset before deleting it"})
		}
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *Handler) AssignAsset(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var in struct {
		UserID int64 `json:"user_id" binding:"required"`
	}
	if c.ShouldBindJSON(&in) != nil || in.UserID < 1 {
		bad(c, "a valid user_id is required")
		return
	}
	tx, err := h.db.BeginTx(c, nil)
	if err != nil {
		serverError(c, "could not start assignment")
		return
	}
	defer tx.Rollback()
	var userExists int
	if err = tx.QueryRowContext(c.Request.Context(), `SELECT COUNT(*) FROM users WHERE id=?`, in.UserID).Scan(&userExists); err != nil {
		serverError(c, "could not verify user")
		return
	}
	if userExists == 0 {
		notFound(c, "user not found")
		return
	}
	res, err := tx.ExecContext(c.Request.Context(), `UPDATE assets SET assigned_user_id=?,status='assigned' WHERE id=? AND assigned_user_id IS NULL AND status='available'`, in.UserID, id)
	if err != nil {
		serverError(c, "could not assign asset")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var exists int
		_ = tx.QueryRowContext(c.Request.Context(), `SELECT COUNT(*) FROM assets WHERE id=?`, id).Scan(&exists)
		if exists == 0 {
			notFound(c, "asset not found")
		} else {
			c.JSON(http.StatusConflict, gin.H{"error": "asset is not available for assignment"})
		}
		return
	}
	if err = tx.Commit(); err != nil {
		serverError(c, "could not complete assignment")
		return
	}
	out, err := h.fetchAsset(c, id)
	if err != nil {
		serverError(c, "could not retrieve assigned asset")
		return
	}
	c.JSON(http.StatusOK, out)
}
func (h *Handler) UnassignAsset(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	res, err := h.db.ExecContext(c.Request.Context(), `UPDATE assets SET assigned_user_id=NULL,status='available' WHERE id=? AND assigned_user_id IS NOT NULL`, id)
	if err != nil {
		serverError(c, "could not unassign asset")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var exists int
		_ = h.db.QueryRowContext(c.Request.Context(), `SELECT COUNT(*) FROM assets WHERE id=?`, id).Scan(&exists)
		if exists == 0 {
			notFound(c, "asset not found")
		} else {
			c.JSON(http.StatusConflict, gin.H{"error": "asset is not currently assigned"})
		}
		return
	}
	out, err := h.fetchAsset(c, id)
	if err != nil {
		serverError(c, "could not retrieve unassigned asset")
		return
	}
	c.JSON(http.StatusOK, out)
}
func (h *Handler) fetchAsset(c *gin.Context, id int64) (Asset, error) {
	row := h.db.QueryRowContext(c.Request.Context(), `SELECT a.id,a.asset_tag,a.asset_name,a.category,a.manufacturer,a.model,a.serial_number,a.status,a.assigned_user_id,u.name,a.location,a.purchase_date,a.created_at,a.updated_at FROM assets a LEFT JOIN users u ON u.id=a.assigned_user_id WHERE a.id=?`, id)
	return scanAsset(row)
}

type scanner interface{ Scan(...any) error }

func scanAsset(row scanner) (Asset, error) {
	var a Asset
	var serial, assignedName sql.NullString
	var assignedID sql.NullInt64
	var purchase sql.NullTime
	err := row.Scan(&a.ID, &a.AssetTag, &a.AssetName, &a.Category, &a.Manufacturer, &a.Model, &serial, &a.Status, &assignedID, &assignedName, &a.Location, &purchase, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return a, err
	}
	if serial.Valid {
		a.SerialNumber = &serial.String
	}
	if assignedID.Valid {
		a.AssignedUserID = &assignedID.Int64
	}
	if assignedName.Valid {
		a.AssignedUserName = &assignedName.String
	}
	if purchase.Valid {
		s := purchase.Time.Format("2006-01-02")
		a.PurchaseDate = &s
	}
	return a, nil
}
func (h *Handler) DashboardStats(c *gin.Context) {
	var s struct {
		TotalUsers        int64 `json:"total_users"`
		TotalAssets       int64 `json:"total_assets"`
		AssignedAssets    int64 `json:"assigned_assets"`
		AvailableAssets   int64 `json:"available_assets"`
		MaintenanceAssets int64 `json:"maintenance_assets"`
		RetiredAssets     int64 `json:"retired_assets"`
	}
	err := h.db.QueryRowContext(c.Request.Context(), `SELECT (SELECT COUNT(*) FROM users),(SELECT COUNT(*) FROM assets),(SELECT COUNT(*) FROM assets WHERE status='assigned'),(SELECT COUNT(*) FROM assets WHERE status='available'),(SELECT COUNT(*) FROM assets WHERE status='maintenance'),(SELECT COUNT(*) FROM assets WHERE status='retired')`).Scan(&s.TotalUsers, &s.TotalAssets, &s.AssignedAssets, &s.AvailableAssets, &s.MaintenanceAssets, &s.RetiredAssets)
	if err != nil {
		serverError(c, "could not load dashboard statistics")
		return
	}
	c.JSON(http.StatusOK, s)
}

func contextWithTimeout(c *gin.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), d)
}
func pathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		bad(c, "invalid ID")
		return 0, false
	}
	return id, true
}
func validateEmail(value string) error {
	address, err := mail.ParseAddress(strings.TrimSpace(value))
	if err != nil || address.Address != strings.TrimSpace(value) {
		return errors.New("invalid email")
	}
	return nil
}
func bad(c *gin.Context, msg string)      { c.JSON(http.StatusBadRequest, gin.H{"error": msg}) }
func notFound(c *gin.Context, msg string) { c.JSON(http.StatusNotFound, gin.H{"error": msg}) }
func serverError(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
}
func duplicate(c *gin.Context, err error, msg string) bool {
	if err == nil {
		return false
	}
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		c.JSON(http.StatusConflict, gin.H{"error": msg})
		return true
	}
	return false
}

func departmentOrDefault(v string) string {
	if strings.TrimSpace(v) == "" {
		return "Unassigned"
	}
	return strings.TrimSpace(v)
}
