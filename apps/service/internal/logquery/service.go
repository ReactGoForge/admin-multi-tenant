package logquery

import (
	"context"
	"strconv"
	"strings"

	"github.com/ReactGoForge/admin-multi-tenant/apps/service/internal/logging"
)

// DataStore 定义日志查询 Service 需要的只读数据访问能力。
type DataStore interface {
	ListSystemLogs(context.Context, listQuery) ([]systemLogRow, int64, error)
	ListAuditLogs(context.Context, listQuery, *uint64) ([]auditLogRow, int64, error)
	ListLoginLogs(context.Context, listQuery, *uint64) ([]systemLogRow, int64, error)
	ListTenants(context.Context) ([]tenantOptionRow, error)
	ListLatestOperators(context.Context, string, *uint64) ([]operatorOptionRow, error)
	ListAuditCodeOptions(context.Context, *uint64) ([]auditModuleOptionRow, []auditActionOptionRow, error)
}

// Service 编排日志查询的数据范围、筛选选项和响应转换。
type Service struct {
	store DataStore
}

// NewService 使用日志查询数据能力创建日志查询服务。
func NewService(store DataStore) *Service { return &Service{store: store} }

// ListPlatformSystemLogs 查询平台可见的系统日志列表。
func (service *Service) ListPlatformSystemLogs(ctx context.Context, query listQuery) (listResponse, error) {
	rows, total, err := service.store.ListSystemLogs(ctx, query)
	if err != nil {
		return listResponse{}, err
	}
	items := make([]objectResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, systemResponse(row))
	}
	return listResponse{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

// ListPlatformAuditLogs 查询平台可见的操作审计日志列表。
func (service *Service) ListPlatformAuditLogs(ctx context.Context, query listQuery) (listResponse, error) {
	rows, total, err := service.store.ListAuditLogs(ctx, query, nil)
	if err != nil {
		return listResponse{}, err
	}
	items := make([]objectResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, auditResponse(row))
	}
	return listResponse{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

// ListTenantAuditLogs 查询认证租户范围内的操作审计日志列表。
func (service *Service) ListTenantAuditLogs(ctx context.Context, tenantID uint64, query listQuery) (listResponse, error) {
	rows, total, err := service.store.ListAuditLogs(ctx, query, &tenantID)
	if err != nil {
		return listResponse{}, err
	}
	items := make([]objectResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, auditResponse(row))
	}
	return listResponse{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

// ListPlatformLoginLogs 查询平台可见的后台登录日志列表。
func (service *Service) ListPlatformLoginLogs(ctx context.Context, query listQuery) (listResponse, error) {
	rows, total, err := service.store.ListLoginLogs(ctx, query, nil)
	if err != nil {
		return listResponse{}, err
	}
	items := make([]objectResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, loginResponse(row))
	}
	return listResponse{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

// ListTenantLoginLogs 查询认证租户范围内的后台登录日志列表。
func (service *Service) ListTenantLoginLogs(ctx context.Context, tenantID uint64, query listQuery) (listResponse, error) {
	rows, total, err := service.store.ListLoginLogs(ctx, query, &tenantID)
	if err != nil {
		return listResponse{}, err
	}
	items := make([]objectResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, loginResponse(row))
	}
	return listResponse{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}

// ListPlatformSystemFilterOptions 查询平台系统日志筛选选项。
func (service *Service) ListPlatformSystemFilterOptions(ctx context.Context) (filterOptionsResponse, error) {
	return service.listFilterOptions(ctx, "system", nil, true)
}

// ListPlatformAuditFilterOptions 查询平台操作日志筛选选项。
func (service *Service) ListPlatformAuditFilterOptions(ctx context.Context) (filterOptionsResponse, error) {
	return service.listFilterOptions(ctx, "audit", nil, true)
}

// ListTenantAuditFilterOptions 查询租户操作日志筛选选项。
func (service *Service) ListTenantAuditFilterOptions(ctx context.Context, tenantID uint64) (filterOptionsResponse, error) {
	return service.listFilterOptions(ctx, "audit", &tenantID, false)
}

// ListPlatformLoginFilterOptions 查询平台登录日志筛选选项。
func (service *Service) ListPlatformLoginFilterOptions(ctx context.Context) (filterOptionsResponse, error) {
	response := emptyFilterOptionsResponse()
	tenants, err := service.store.ListTenants(ctx)
	if err != nil {
		return filterOptionsResponse{}, err
	}
	response.Tenants = tenantOptions(tenants)
	return response, nil
}

// ListTenantLoginFilterOptions 返回租户登录日志的空筛选选项集合。
func (service *Service) ListTenantLoginFilterOptions() filterOptionsResponse {
	return emptyFilterOptionsResponse()
}

// listFilterOptions 按日志可见范围查询租户、操作者、模块和动作筛选选项。
func (service *Service) listFilterOptions(ctx context.Context, kind string, tenantID *uint64, includeTenants bool) (filterOptionsResponse, error) {
	response := emptyFilterOptionsResponse()
	if includeTenants {
		tenants, err := service.store.ListTenants(ctx)
		if err != nil {
			return filterOptionsResponse{}, err
		}
		response.Tenants = tenantOptions(tenants)
	}

	operators, err := service.store.ListLatestOperators(ctx, kind, tenantID)
	if err != nil {
		return filterOptionsResponse{}, err
	}
	response.Operators = operatorOptions(operators)
	if kind == "audit" {
		modules, actions, optionsErr := service.store.ListAuditCodeOptions(ctx, tenantID)
		if optionsErr != nil {
			return filterOptionsResponse{}, optionsErr
		}
		response.Modules = moduleOptions(modules)
		response.Actions = actionOptions(actions)
	}
	return response, nil
}

// emptyFilterOptionsResponse 创建兼容前端的空筛选选项响应。
func emptyFilterOptionsResponse() filterOptionsResponse {
	return filterOptionsResponse{Tenants: []filterOption{}, Operators: []operatorOption{}, Modules: []codeOption{}, Actions: []codeOption{}}
}

// tenantOptions 将租户数据库行转换为筛选选项。
func tenantOptions(rows []tenantOptionRow) []filterOption {
	options := make([]filterOption, 0, len(rows))
	for _, row := range rows {
		status := "disabled"
		if row.Status == 1 {
			status = "enabled"
		}
		options = append(options, filterOption{ID: strconv.FormatUint(row.ID, 10), Name: row.Name, Status: status})
	}
	return options
}

// operatorOptions 将历史操作者最新快照转换为筛选选项。
func operatorOptions(rows []operatorOptionRow) []operatorOption {
	options := make([]operatorOption, 0, len(rows))
	for _, row := range rows {
		name := "未知操作者"
		if row.Name != nil && strings.TrimSpace(*row.Name) != "" {
			name = strings.TrimSpace(*row.Name)
		}
		options = append(options, operatorOption{
			Key:       row.ActorType + ":" + strconv.FormatUint(row.ActorID, 10),
			ActorType: row.ActorType,
			ActorID:   strconv.FormatUint(row.ActorID, 10),
			Name:      name,
			Account:   row.Account,
		})
	}
	return options
}

// moduleOptions 将审计模块编码转换为带展示文案的筛选选项。
func moduleOptions(rows []auditModuleOptionRow) []codeOption {
	options := make([]codeOption, 0, len(rows))
	for _, row := range rows {
		options = append(options, codeOption{Value: row.Value, Label: logging.AuditModuleLabel(row.Value)})
	}
	return options
}

// actionOptions 将审计动作编码和历史名称转换为筛选选项。
func actionOptions(rows []auditActionOptionRow) []codeOption {
	options := make([]codeOption, 0, len(rows))
	for _, row := range rows {
		options = append(options, codeOption{Value: row.Value, Label: row.Label})
	}
	return options
}
