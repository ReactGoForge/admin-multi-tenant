package user

import (
	"context"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var (
	errTenantNotFound     = errors.New("tenant not found")
	errTenantDisabled     = errors.New("tenant disabled")
	errUserNotFound       = errors.New("user not found")
	errUserDisabled       = errors.New("user disabled")
	errTenantUserDisabled = errors.New("tenant user disabled")
	errIdentityConflict   = errors.New("wechat identity conflict")
)

// User 描述小程序平台用户的可公开业务字段。
type User struct {
	ID              uint64
	WechatOpenID    string
	WechatUnionID   *string
	Phone           *string
	Nickname        *string
	AvatarObjectKey *string
	AvatarURL       *string
	Status          uint8
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Tenant 描述小程序登录和响应所需的租户字段。
type Tenant struct {
	ID     uint64
	Name   string
	Status uint8
}

// Session 描述平台用户在指定租户下的实时登录状态。
type Session struct {
	User             User
	Tenant           Tenant
	TenantUserStatus uint8
	JoinedAt         time.Time
}

// PlatformUserQuery 描述平台用户列表的分页和筛选条件。
type PlatformUserQuery struct {
	Page     int
	PageSize int
	Nickname string
	Phone    string
	TenantID *uint64
	Status   *uint8
}

// TenantUserQuery 描述租户用户列表的分页和筛选条件。
type TenantUserQuery struct {
	Page     int
	PageSize int
	Nickname string
	Phone    string
	Status   *uint8
}

// PlatformUser 描述平台用户列表的一行数据。
type PlatformUser struct {
	User
	TenantCount int64
}

// TenantUser 描述租户用户列表的一行数据。
type TenantUser struct {
	User
	TenantStatus uint8
	JoinedAt     time.Time
}

// TenantOption 描述平台用户筛选所需的最小租户字段。
type TenantOption struct {
	ID     uint64
	Name   string
	Status uint8
}

// PlatformUserTenant 描述平台用户关联的一条租户归属。
type PlatformUserTenant struct {
	TenantID     uint64
	TenantName   string
	TenantStatus uint8
	UserStatus   uint8
	JoinedAt     time.Time
}

// GormStore 使用 GORM 访问小程序用户和租户归属表。
type GormStore struct {
	db *gorm.DB
}

// miniappGormTransactionStore 使用事务内 GORM 连接访问小程序登录数据。
type miniappGormTransactionStore struct {
	db *gorm.DB
}

// GetMiniappAppID 读取数据库中的全平台唯一微信小程序 AppID。
func (store *GormStore) GetMiniappAppID(ctx context.Context) (string, error) {
	var row struct {
		AppID string `gorm:"column:app_id"`
	}
	err := store.db.WithContext(ctx).Table("wechat_miniapp_settings").Select("app_id").Where("id = ?", 1).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return row.AppID, err
}

// SaveMiniappAppID 新增或更新全平台唯一微信小程序 AppID。
func (store *GormStore) SaveMiniappAppID(ctx context.Context, appID string) error {
	return store.db.WithContext(ctx).Exec("INSERT INTO wechat_miniapp_settings (id, app_id) VALUES (1, ?) ON DUPLICATE KEY UPDATE app_id = VALUES(app_id)", appID).Error
}

// FindTenantOption 按 ID 读取小程序码生成所需的租户最小信息。
func (store *GormStore) FindTenantOption(ctx context.Context, tenantID uint64) (*TenantOption, error) {
	var option TenantOption
	err := store.db.WithContext(ctx).Table("tenants").Select("id", "name", "status").Where("id = ?", tenantID).Take(&option).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &option, err
}

// NewStore 创建小程序用户数据对象。
func NewStore(db *gorm.DB) *GormStore {
	return &GormStore{db: db}
}

// userRow 映射微信用户创建或更新所需的平台用户字段。
type userRow struct {
	ID              uint64    `gorm:"column:id"`
	WechatOpenID    string    `gorm:"column:wechat_openid"`
	WechatUnionID   *string   `gorm:"column:wechat_unionid"`
	Phone           *string   `gorm:"column:phone"`
	Nickname        *string   `gorm:"column:nickname"`
	AvatarObjectKey *string   `gorm:"column:avatar_url"`
	Status          uint8     `gorm:"column:status"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

// userListRow 显式映射用户列表查询字段，避免匿名嵌入结构无法被 GORM 扫描。
type userListRow struct {
	ID               uint64    `gorm:"column:id"`
	WechatOpenID     string    `gorm:"column:wechat_openid"`
	WechatUnionID    *string   `gorm:"column:wechat_unionid"`
	Phone            *string   `gorm:"column:phone"`
	Nickname         *string   `gorm:"column:nickname"`
	AvatarObjectKey  *string   `gorm:"column:avatar_url"`
	Status           uint8     `gorm:"column:status"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
	TenantCount      int64     `gorm:"column:tenant_count"`
	TenantStatus     uint8     `gorm:"column:tenant_status"`
	JoinedAt         time.Time `gorm:"column:joined_at"`
	TenantName       string    `gorm:"column:tenant_name"`
	TenantUserStatus uint8     `gorm:"column:tenant_user_status"`
}

// tenantRow 映射小程序登录时校验的租户字段。
type tenantRow struct {
	ID     uint64 `gorm:"column:id"`
	Name   string `gorm:"column:name"`
	Status uint8  `gorm:"column:status"`
}

// tenantUserRow 映射平台用户与租户归属关系字段。
type tenantUserRow struct {
	TenantID uint64    `gorm:"column:tenant_id"`
	UserID   uint64    `gorm:"column:user_id"`
	Status   uint8     `gorm:"column:status"`
	JoinedAt time.Time `gorm:"column:joined_at;autoCreateTime"`
}

// TableName 指定平台用户模型对应的真实表名。
func (userRow) TableName() string { return "users" }

// TableName 指定租户用户关系模型对应的真实表名。
func (tenantUserRow) TableName() string { return "tenant_users" }

// WithMiniappTransaction 在单个数据库事务内执行小程序登录数据写入。
func (store *GormStore) WithMiniappTransaction(ctx context.Context, fn func(MiniappTransactionStore) error) error {
	// Go 学习提示：GORM Transaction 回调返回 nil 时自动提交；返回任意错误时自动回滚。
	// Service 决定事务边界，Store 只负责把事务内数据能力交给回调。
	return store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&miniappGormTransactionStore{db: tx})
	})
}

// FindTenantForLogin 读取小程序登录目标租户的最小字段。
func (store *miniappGormTransactionStore) FindTenantForLogin(ctx context.Context, tenantID uint64) (Tenant, error) {
	var tenant tenantRow
	err := store.db.WithContext(ctx).Table("tenants").Select("id", "name", "status").Where("id = ?", tenantID).Take(&tenant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Tenant{}, errTenantNotFound
	}
	if err != nil {
		return Tenant{}, err
	}
	return Tenant{ID: tenant.ID, Name: tenant.Name, Status: tenant.Status}, nil
}

// EnsureWechatUser 创建或复用 OpenID 对应的平台用户，并按需绑定微信返回的可信手机号。
func (store *miniappGormTransactionStore) EnsureWechatUser(_ context.Context, identity WechatIdentity, phone *string) (User, error) {
	user, err := ensureUser(store.db, identity, phone)
	if err != nil {
		return User{}, err
	}
	return newUser(*user), nil
}

// EnsureTenantMembership 创建或复用用户在当前租户内的归属关系。
func (store *miniappGormTransactionStore) EnsureTenantMembership(_ context.Context, tenantID, userID uint64) (TenantMembership, error) {
	membership, err := ensureTenantUser(store.db, tenantID, userID)
	if err != nil {
		return TenantMembership{}, err
	}
	return TenantMembership{Status: membership.Status, JoinedAt: membership.JoinedAt}, nil
}

// ensureUser 创建或复用 OpenID 对应的平台用户，并保守补充 UnionID 与可信手机号。
func ensureUser(tx *gorm.DB, identity WechatIdentity, phone *string) (*userRow, error) {
	var row userRow
	err := tx.Where("wechat_openid = ?", identity.OpenID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := ensurePhoneAvailable(tx, 0, phone); err != nil {
			return nil, err
		}
		row = userRow{WechatOpenID: identity.OpenID, WechatUnionID: identity.UnionID, Phone: phone, Status: 1}
		if createErr := tx.Create(&row).Error; createErr != nil {
			// 业务约束：两个并发的首次登录可能同时查询不到用户。唯一索引会让其中一次创建失败，
			// 失败的一方重新查询即可复用已创建的用户，不应把它误报为系统错误。
			if !isDuplicateError(createErr) {
				return nil, createErr
			}
			if retryErr := tx.Where("wechat_openid = ?", identity.OpenID).Take(&row).Error; retryErr != nil {
				return nil, errIdentityConflict
			}
		}
	} else if err != nil {
		return nil, err
	}

	if identity.UnionID != nil {
		// 安全边界：OpenID 已绑定的 UnionID 不允许被另一个微信身份覆盖。
		// 指针为 nil 表示数据库当前没有该值，并不等同于空字符串。
		if row.WechatUnionID != nil {
			if *row.WechatUnionID != *identity.UnionID {
				return nil, errIdentityConflict
			}
		} else {
			result := tx.Model(&userRow{}).Where("id = ? AND wechat_unionid IS NULL", row.ID).Update("wechat_unionid", *identity.UnionID)
			if updateErr := result.Error; updateErr != nil {
				if isDuplicateError(updateErr) {
					return nil, errIdentityConflict
				}
				return nil, updateErr
			}
			if result.RowsAffected == 0 {
				if err := tx.Where("id = ?", row.ID).Take(&row).Error; err != nil {
					return nil, err
				}
				if row.WechatUnionID == nil || *row.WechatUnionID != *identity.UnionID {
					return nil, errIdentityConflict
				}
			} else {
				row.WechatUnionID = identity.UnionID
			}
		}
	}

	if phone == nil {
		return &row, nil
	}
	if row.Phone != nil {
		if *row.Phone != *phone {
			return nil, errIdentityConflict
		}
		return &row, nil
	}
	if err := ensurePhoneAvailable(tx, row.ID, phone); err != nil {
		return nil, err
	}
	result := tx.Model(&userRow{}).Where("id = ? AND phone IS NULL", row.ID).Update("phone", *phone)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		if err := tx.Where("id = ?", row.ID).Take(&row).Error; err != nil {
			return nil, err
		}
		if row.Phone == nil || *row.Phone != *phone {
			return nil, errIdentityConflict
		}
		return &row, nil
	}
	row.Phone = phone
	return &row, nil
}

// ensurePhoneAvailable 拒绝把同一个可信手机号绑定到其他 OpenID 用户。
func ensurePhoneAvailable(tx *gorm.DB, userID uint64, phone *string) error {
	if phone == nil {
		return nil
	}
	var existing userRow
	query := tx.Select("id").Where("phone = ?", *phone)
	if userID != 0 {
		query = query.Where("id <> ?", userID)
	}
	err := query.Take(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return errIdentityConflict
}

// ensureTenantUser 创建或复用用户在当前租户的归属关系。
func ensureTenantUser(tx *gorm.DB, tenantID, userID uint64) (*tenantUserRow, error) {
	var row tenantUserRow
	err := tx.Where("tenant_id = ? AND user_id = ?", tenantID, userID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = tenantUserRow{TenantID: tenantID, UserID: userID, Status: 1}
		if createErr := tx.Create(&row).Error; createErr != nil {
			if !isDuplicateError(createErr) {
				return nil, createErr
			}
			if retryErr := tx.Where("tenant_id = ? AND user_id = ?", tenantID, userID).Take(&row).Error; retryErr != nil {
				return nil, retryErr
			}
		}
	} else if err != nil {
		return nil, err
	}
	return &row, nil
}

// FindSession 读取 Token 对应用户、租户和租户关系的最新状态。
func (store *GormStore) FindSession(ctx context.Context, userID, tenantID uint64) (*Session, error) {
	var row userListRow
	// Go 学习提示：WithContext 把请求的取消和超时传给数据库驱动；客户端断开或请求超时后，
	// 尚未完成的查询可以尽快停止。下面的链式调用只是在组装一条 SQL，Take 才真正执行。
	// 业务约束：JOIN 同时读取平台用户、租户和归属关系，确保返回的是同一个租户范围内的会话。
	err := store.db.WithContext(ctx).Table("users AS u").
		Select("u.id, u.wechat_openid, u.wechat_unionid, u.phone, u.nickname, u.avatar_url, u.status, u.created_at, u.updated_at, t.name AS tenant_name, t.status AS tenant_status, tu.status AS tenant_user_status, tu.joined_at").
		Joins("JOIN tenant_users AS tu ON tu.user_id = u.id AND tu.tenant_id = ?", tenantID).
		Joins("JOIN tenants AS t ON t.id = tu.tenant_id").
		Where("u.id = ?", userID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &Session{User: newUserFromListRow(row), Tenant: Tenant{ID: tenantID, Name: row.TenantName, Status: row.TenantStatus}, TenantUserStatus: row.TenantUserStatus, JoinedAt: row.JoinedAt}, nil
}

// ListPlatformUsers 分页查询平台唯一用户及其租户数量。
func (store *GormStore) ListPlatformUsers(ctx context.Context, query PlatformUserQuery) ([]PlatformUser, int64, error) {
	base := store.db.WithContext(ctx).Table("users AS u")
	if query.Nickname != "" {
		base = base.Where("LOCATE(?, COALESCE(u.nickname, '')) > 0", query.Nickname)
	}
	if query.Phone != "" {
		base = base.Where("LOCATE(?, COALESCE(u.phone, '')) > 0", query.Phone)
	}
	if query.Status != nil {
		base = base.Where("u.status = ?", *query.Status)
	}
	if query.TenantID != nil {
		base = base.Where("EXISTS (SELECT 1 FROM tenant_users AS filtered_tu WHERE filtered_tu.user_id = u.id AND filtered_tu.tenant_id = ?)", *query.TenantID)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]userListRow, 0)
	err := base.Select("u.id, u.wechat_openid, u.wechat_unionid, u.phone, u.nickname, u.avatar_url, u.status, u.created_at, u.updated_at, (SELECT COUNT(*) FROM tenant_users AS counted_tu WHERE counted_tu.user_id = u.id) AS tenant_count").
		Order("u.id DESC").Limit(query.PageSize).Offset((query.Page - 1) * query.PageSize).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	items := make([]PlatformUser, 0, len(rows))
	for _, row := range rows {
		items = append(items, PlatformUser{User: newUserFromListRow(row), TenantCount: row.TenantCount})
	}
	return items, total, nil
}

// ListPlatformUserTenants 查询指定平台用户关联的全部租户。
func (store *GormStore) ListPlatformUserTenants(ctx context.Context, userID uint64) ([]PlatformUserTenant, error) {
	if err := store.ensureUserExists(ctx, userID); err != nil {
		return nil, err
	}
	tenants := make([]PlatformUserTenant, 0)
	err := store.db.WithContext(ctx).Table("tenant_users AS tu").
		Select("t.id AS tenant_id, t.name AS tenant_name, t.status AS tenant_status, tu.status AS user_status, tu.joined_at").
		Joins("JOIN tenants AS t ON t.id = tu.tenant_id").
		Where("tu.user_id = ?", userID).
		Order("tu.joined_at DESC, tu.tenant_id DESC").
		Scan(&tenants).Error
	return tenants, err
}

// ListTenantOptions 查询平台用户筛选可使用的全部租户，并按名称和 ID 稳定排序。
func (store *GormStore) ListTenantOptions(ctx context.Context) ([]TenantOption, error) {
	options := make([]TenantOption, 0)
	err := store.db.WithContext(ctx).
		Table("tenants").
		Select("id", "name", "status").
		Order("name ASC, id ASC").
		Scan(&options).Error
	return options, err
}

// SetPlatformUserStatus 更新平台用户的全局启用状态。
func (store *GormStore) SetPlatformUserStatus(ctx context.Context, userID uint64, status uint8) error {
	result := store.db.WithContext(ctx).Table("users").Where("id = ?", userID).Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return store.ensureUserExists(ctx, userID)
	}
	return nil
}

// ListTenantUsers 分页查询当前租户的用户归属和平台状态。
func (store *GormStore) ListTenantUsers(ctx context.Context, tenantID uint64, query TenantUserQuery) ([]TenantUser, int64, error) {
	base := store.db.WithContext(ctx).Table("tenant_users AS tu").Joins("JOIN users AS u ON u.id = tu.user_id").Where("tu.tenant_id = ?", tenantID)
	if query.Nickname != "" {
		base = base.Where("LOCATE(?, COALESCE(u.nickname, '')) > 0", query.Nickname)
	}
	if query.Phone != "" {
		base = base.Where("LOCATE(?, COALESCE(u.phone, '')) > 0", query.Phone)
	}
	if query.Status != nil {
		base = base.Where("tu.status = ?", *query.Status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]userListRow, 0)
	err := base.Select("u.id, u.wechat_openid, u.wechat_unionid, u.phone, u.nickname, u.avatar_url, u.status, u.created_at, u.updated_at, tu.status AS tenant_status, tu.joined_at").
		Order("tu.joined_at DESC, tu.user_id DESC").Limit(query.PageSize).Offset((query.Page - 1) * query.PageSize).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	items := make([]TenantUser, 0, len(rows))
	for _, row := range rows {
		items = append(items, TenantUser{User: newUserFromListRow(row), TenantStatus: row.TenantStatus, JoinedAt: row.JoinedAt})
	}
	return items, total, nil
}

// SetTenantUserStatus 更新用户在指定租户内的启用状态。
func (store *GormStore) SetTenantUserStatus(ctx context.Context, tenantID, userID uint64, status uint8) error {
	result := store.db.WithContext(ctx).Table("tenant_users").Where("tenant_id = ? AND user_id = ?", tenantID, userID).Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := store.db.WithContext(ctx).Table("tenant_users").Where("tenant_id = ? AND user_id = ?", tenantID, userID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errUserNotFound
		}
	}
	return nil
}

// ensureUserExists 将状态幂等更新与用户不存在区分开。
func (store *GormStore) ensureUserExists(ctx context.Context, userID uint64) error {
	var count int64
	if err := store.db.WithContext(ctx).Table("users").Where("id = ?", userID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errUserNotFound
	}
	return nil
}

// newUser 将数据库行转换为不包含表结构细节的业务用户。
func newUser(row userRow) User {
	return User{ID: row.ID, WechatOpenID: row.WechatOpenID, WechatUnionID: row.WechatUnionID, Phone: row.Phone, Nickname: row.Nickname, AvatarObjectKey: row.AvatarObjectKey, Status: row.Status, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

// newUserFromListRow 将显式列表扫描行转换为业务用户。
func newUserFromListRow(row userListRow) User {
	return User{ID: row.ID, WechatOpenID: row.WechatOpenID, WechatUnionID: row.WechatUnionID, Phone: row.Phone, Nickname: row.Nickname, AvatarObjectKey: row.AvatarObjectKey, Status: row.Status, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

// isDuplicateError 判断 MySQL 唯一约束冲突，供并发首次登录安全重试。
func isDuplicateError(err error) bool {
	var mysqlError *mysql.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
