package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/database"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	superAdminRoleID      uint64 = 1001
	transactionTimeout           = 10 * time.Second
	minPasswordCharacters        = 6
	maxPasswordCharacters        = 18
)

// roleRow 映射平台超级管理员角色查询所需的数据库字段。
type roleRow struct {
	ID uint64 `gorm:"column:id;primaryKey"`
}

// TableName 指定 roleRow 对应现有的 roles 表。
func (roleRow) TableName() string {
	return "roles"
}

// employeeRow 映射平台所有者员工创建和存在性检查所需的数据库字段。
type employeeRow struct {
	ID           uint64  `gorm:"column:id;primaryKey;autoIncrement"`
	Scope        string  `gorm:"column:scope"`
	TenantID     *uint64 `gorm:"column:tenant_id"`
	DepartmentID *uint64 `gorm:"column:department_id"`
	Name         string  `gorm:"column:name"`
	LoginAccount string  `gorm:"column:login_account"`
	PasswordHash string  `gorm:"column:password_hash"`
	Phone        *string `gorm:"column:phone"`
	Status       uint8   `gorm:"column:status"`
}

// TableName 指定 employeeRow 对应现有的 employees 表。
func (employeeRow) TableName() string {
	return "employees"
}

// employeeRoleRow 映射员工与角色关联写入所需的数据库字段。
type employeeRoleRow struct {
	Scope      string  `gorm:"column:scope"`
	TenantID   *uint64 `gorm:"column:tenant_id"`
	EmployeeID uint64  `gorm:"column:employee_id"`
	RoleID     uint64  `gorm:"column:role_id"`
}

// TableName 指定 employeeRoleRow 对应现有的 employee_roles 表。
func (employeeRoleRow) TableName() string {
	return "employee_roles"
}

// main 执行平台所有者初始化命令，并把失败原因输出到标准错误。
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "初始化平台所有者失败: %v\n", err)
		os.Exit(1)
	}
}

// run 校验命令参数和密码，连接数据库并完成一次性平台所有者创建。
func run(arguments []string) error {
	flags := flag.NewFlagSet("bootstrap-owner", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	accountFlag := flags.String("account", "", "平台所有者登录账号")
	nameFlag := flags.String("name", "", "平台所有者员工姓名")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("存在无法识别的额外参数")
	}

	account, name, err := validateIdentity(*accountFlag, *nameFlag)
	if err != nil {
		return err
	}

	config, err := database.LoadConfig()
	if err != nil {
		// Go 学习提示：%w 会把原始错误包在带业务语义的新错误中。上层既能看到这里的说明，
		// 也可以继续用 errors.Is/errors.As 判断底层错误类型。
		return fmt.Errorf("读取数据库配置失败: %w", err)
	}
	db, sqlDB, err := database.Open(context.Background(), config)
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	defer func() {
		_ = sqlDB.Close()
	}()

	stdinFD := int(os.Stdin.Fd())
	if !term.IsTerminal(stdinFD) {
		return fmt.Errorf("必须在交互式终端中运行，以安全输入密码")
	}

	password, err := readPassword(stdinFD, "请输入平台所有者密码: ")
	if err != nil {
		return err
	}
	// 安全边界：密码使用 []byte 而不是 string，使用结束后可主动清零底层内存；
	// string 不可变，无法可靠地覆盖其中的敏感内容。
	defer clear(password)
	confirmation, err := readPassword(stdinFD, "请再次输入平台所有者密码: ")
	if err != nil {
		return err
	}
	defer clear(confirmation)
	if err := validatePassword(password, confirmation); err != nil {
		return err
	}

	passwordHash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("生成密码哈希失败: %w", err)
	}

	transactionContext, cancel := context.WithTimeout(context.Background(), transactionTimeout)
	defer cancel()
	employeeID, err := createPlatformOwner(transactionContext, db, account, name, string(passwordHash))
	if err != nil {
		return err
	}

	fmt.Printf("平台所有者初始化成功：员工ID=%d，账号=%s\n", employeeID, account)
	return nil
}

// validateIdentity 清理并校验平台所有者的登录账号和员工姓名。
func validateIdentity(account, name string) (string, string, error) {
	account = strings.TrimSpace(account)
	name = strings.TrimSpace(name)
	if account == "" {
		return "", "", fmt.Errorf("登录账号不能为空")
	}
	if utf8.RuneCountInString(account) > 40 {
		return "", "", fmt.Errorf("登录账号不能超过 40 个字符")
	}
	if name == "" {
		return "", "", fmt.Errorf("员工姓名不能为空")
	}
	if utf8.RuneCountInString(name) > 30 {
		return "", "", fmt.Errorf("员工姓名不能超过 30 个字符")
	}

	return account, name, nil
}

// validatePassword 按字符校验密码长度，并确认两次输入完全一致。
func validatePassword(password, confirmation []byte) error {
	passwordLength := utf8.RuneCount(password)
	if passwordLength < minPasswordCharacters {
		return fmt.Errorf("密码不能少于 %d 个字符", minPasswordCharacters)
	}
	if passwordLength > maxPasswordCharacters {
		return fmt.Errorf("密码不能超过 %d 个字符", maxPasswordCharacters)
	}
	if !bytes.Equal(password, confirmation) {
		return fmt.Errorf("两次输入的密码不一致")
	}

	return nil
}

// readPassword 从交互式终端无回显地读取密码。
func readPassword(fileDescriptor int, prompt string) ([]byte, error) {
	fmt.Fprint(os.Stderr, prompt)
	password, err := term.ReadPassword(fileDescriptor)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("读取密码失败: %w", err)
	}

	return password, nil
}

// createPlatformOwner 在单个事务中创建平台员工，并关联唯一的平台超级管理员角色。
func createPlatformOwner(ctx context.Context, db *gorm.DB, account, name, passwordHash string) (uint64, error) {
	var employeeID uint64
	// Go 学习提示：回调返回 nil 时事务提交，返回错误时事务回滚。
	// 业务约束：角色检查、唯一所有者检查、员工创建和角色关联必须作为一个整体成功。
	err := db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var roles []roleRow
		// 业务约束：FOR UPDATE 锁住内置超级管理员角色，避免并发初始化时越过检查。
		if err := transaction.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("scope = ? AND tenant_id IS NULL AND system_key = ? AND status = ?", "platform", "platform_super_admin", 1).
			Find(&roles).Error; err != nil {
			return fmt.Errorf("查询平台超级管理员角色失败: %w", err)
		}
		if len(roles) != 1 || roles[0].ID != superAdminRoleID {
			return fmt.Errorf("平台超级管理员角色不存在、未启用或数据不唯一")
		}

		var assignmentCount int64
		if err := transaction.
			Table("employee_roles").
			Where("role_id = ?", superAdminRoleID).
			Count(&assignmentCount).Error; err != nil {
			return fmt.Errorf("检查平台所有者状态失败: %w", err)
		}
		if assignmentCount != 0 {
			return fmt.Errorf("平台所有者已经初始化，禁止重复创建")
		}

		var accountCount int64
		if err := transaction.
			Table("employees").
			Where("login_account = ?", account).
			Count(&accountCount).Error; err != nil {
			return fmt.Errorf("检查登录账号失败: %w", err)
		}
		if accountCount != 0 {
			return fmt.Errorf("登录账号已存在")
		}

		employee := employeeRow{
			Scope:        "platform",
			Name:         name,
			LoginAccount: account,
			PasswordHash: passwordHash,
			Status:       1,
		}
		if err := transaction.Create(&employee).Error; err != nil {
			return fmt.Errorf("创建平台所有者员工失败: %w", err)
		}

		assignment := employeeRoleRow{
			Scope:      "platform",
			EmployeeID: employee.ID,
			RoleID:     superAdminRoleID,
		}
		if err := transaction.Create(&assignment).Error; err != nil {
			return fmt.Errorf("关联平台超级管理员角色失败: %w", err)
		}

		employeeID = employee.ID
		return nil
	})
	if err != nil {
		return 0, err
	}

	return employeeID, nil
}
