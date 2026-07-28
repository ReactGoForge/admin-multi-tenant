# 前端组件说明

本目录按组件职责分为基础组件、组合组件和业务组件。新增组件前应先判断现有组件是否能够满足需求，避免重复封装 Ant Design 能力。

## 目录分层

| 目录 | 职责 | 当前组件 |
| --- | --- | --- |
| `base` | 对外表现为单一控件或基础布局，不负责完整业务流程 | `ConfirmDelete`、`IconPicker`、`PageContainer`、`StatusSwitch`、具名实体选择器 |
| `composite` | 组合多个基础控件，提供表单、列表、抽屉或图片库等完整通用能力 | `EmptyPage`、`FormDrawer`、`ImagePicker`、`SchemaForm`、`SearchTable` |
| `domain` | 包含认证、日志、RBAC 等业务规则，只在对应业务范围内复用 | `Permission`、路由守卫、日志列表、员工/角色/菜单/部门管理组件 |

基础和组合组件的 Props 均导出具名类型。业务组件只对职责和关键工作空间参数做源码说明，不作为通用组件 API 使用。

## 基础组件

### ConfirmDelete

导入：`@/components/base/confirm-delete`

统一删除操作的二次确认文案。

| 属性 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `children` | `ReactElement` | 是 | - | 触发确认框的单个元素 |
| `onConfirm` | `() => void` | 是 | - | 用户确认删除时触发 |
| `title` | `string` | 否 | `确认删除？` | 确认框标题 |
| `description` | `string` | 否 | 删除后无法恢复的统一提示 | 风险说明 |
| `disabled` | `boolean` | 否 | `false` | 禁用确认交互 |

### IconPicker

导入：`@/components/base/icon-picker`

从项目注册的 Ant Design 图标名称中搜索和选择图标。

| 属性 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `value` | `string` | 否 | - | 当前图标名称 |
| `onChange` | `(value?: string) => void` | 否 | - | 选择或清空时触发 |
| `disabled` | `boolean` | 否 | `false` | 禁止打开和清空 |
| `allowClear` | `boolean` | 否 | `true` | 是否显示清空入口 |

### PageContainer

导入：`@/components/base/page-container`

| 属性 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `children` | `ReactNode` | 是 | - | 页面主体内容，组件统一控制外层宽度和间距 |

### StatusSwitch

导入：`@/components/base/status-switch`

| 属性 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `value` | `"enabled" \| "disabled"` | 是 | - | 当前实体状态 |
| `onChange` | `(value: EntityStatus) => void` | 是 | - | 用户切换后返回新状态 |
| `disabled` | `boolean` | 否 | `false` | 禁止状态切换 |

组件通过系统字典读取启用和禁用文案，不接受自定义状态文案。

### 具名实体选择器

统一从 `@/components/base/selects` 导入，每个选择器拥有独立源码文件，共用内部搜索和布局逻辑。

| 组件 | 选项类型 | 提交值 | 默认占位符 | 搜索字段 |
| --- | --- | --- | --- | --- |
| `TenantSelect` | `NamedEntityOption[]` | 租户 ID | 请选择租户 | 名称 |
| `DepartmentSelect` | `NamedEntityOption[]` | 部门 ID | 请选择部门 | 名称 |
| `RoleSelect` | `NamedEntityOption[]` | 角色 ID 或 ID 数组 | 请选择角色 | 名称 |
| `EmployeeSelect` | `EmployeeSelectOption[]` | 员工 ID | 请选择员工 | 姓名、登录账号 |
| `LogOperatorSelect` | `LogOperatorSelectOption[]` | 操作者复合键 | 请选择操作者 | 姓名、账号、操作者类型 |

除 `options` 和 `showSearch` 外，其余属性透传给 Ant Design `Select`。组件固定启用搜索，默认 `allowClear=true`、宽度 `100%`。`TenantSelect`、`DepartmentSelect`、`RoleSelect`、`EmployeeSelect` 额外支持：

| 属性 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `options` | 对应业务选项数组 | 是 | - | 选择器数据源 |
| `showStatus` | `boolean` | 否 | `true` | 是否在禁用实体名称后展示“已禁用” |

单选示例：

```tsx
<TenantSelect
  options={tenants}
  value={tenantId}
  onChange={setTenantId}
/>
```

多选示例：

```tsx
<RoleSelect
  mode="multiple"
  options={roles}
  value={roleIds}
  onChange={setRoleIds}
/>
```

## 组合组件

### EmptyPage

导入：`@/components/composite/empty-page`

| 属性 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `title` | `string` | 是 | - | 功能占位页标题；组件同时展示当前路由 |

### FormDrawer

导入：`@/components/composite/form-drawer`

| 属性 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `title` | `ReactNode` | 是 | - | 抽屉标题 |
| `open` | `boolean` | 是 | - | 是否显示抽屉 |
| `children` | `ReactNode` | 是 | - | 表单或只读内容 |
| `onClose` | `() => void` | 是 | - | 关闭或取消时触发 |
| `onSubmit` | `() => void` | 否 | - | 点击保存时触发 |
| `loading` | `boolean` | 否 | `false` | 保存按钮加载状态 |
| `readonly` | `boolean` | 否 | `false` | 只读时仅显示关闭按钮 |
| `width` | `number` | 否 | `560` | 抽屉宽度 |

### ImageLibrary 与 ImagePicker

导入：`@/components/composite/image-picker`

`ImageLibrary` 是独立图片库页面和图片选择弹窗共用的图库主体，统一负责来源、分类、搜索、分页、上传、预览、编辑、删除和批量管理。组件内部根据工作空间和当前权限控制具体能力。

管理模式直接用于图片库页面：

```tsx
<ImageLibrary mode="manage" workspace="tenant" />
```

选择模式由 `ImagePicker` 内部使用，业务页面继续使用 `ImagePicker`，其选择值为 `ImageValue` 摘要，不直接向表单返回完整图片实体。

共享配置：

| 属性 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `workspace` | `"platform" \| "tenant"` | 是 | - | 决定请求范围和权限前缀 |
| `selectionOwner` | `"platform" \| "tenant"` | 是 | - | 限制最终可选图片的数据来源 |
| `disabled` | `boolean` | 否 | `false` | 禁止打开、选择和清空 |
| `children` | `(context) => ReactNode` | 否 | 默认按钮 | 自定义触发器 |

单选模式：

| 属性 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `multiple` | `false` | 否 | `false` | 单选模式标识 |
| `value` | `ImageValue \| null` | 否 | `null` | 当前图片摘要 |
| `onChange` | `(value: ImageValue \| null) => void` | 否 | - | 确认或清空时触发 |

多选模式：

| 属性 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `multiple` | `true` | 是 | - | 开启多选 |
| `maxCount` | `number` | 否 | 不限制 | 最大选择数量，负数按 0 处理 |
| `value` | `ImageValue[]` | 否 | `[]` | 当前图片摘要列表 |
| `onChange` | `(value: ImageValue[]) => void` | 否 | - | 确认或清空时触发 |

自定义触发器上下文包含 `value`、`openPicker()`、`clear()` 和 `disabled`。

```tsx
<ImagePicker
  workspace="tenant"
  selectionOwner="tenant"
  value={icon}
  onChange={setIcon}
>
  {({ value, openPicker, clear, disabled }) => (
    <BrandImageField
      disabled={disabled}
      image={value}
      onClear={clear}
      onSelect={openPicker}
    />
  )}
</ImagePicker>
```

### SchemaForm

导入：`@/components/composite/schema-form`

`SchemaForm<TValues>` 根据配置渲染 Ant Design 表单。复杂字段通过 `render` 或 `renderItem` 接入，不为业务控件新增通用字段类型。

组件配置：

| 属性 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `fields` | `FormContentConfig<TValues>[]` | 是 | - | 字段和自定义内容配置 |
| `initialValues` | `Partial<TValues>` | 否 | - | 表单初始值 |
| `form` | `FormInstance<TValues>` | 否 | 内部实例 | 外部控制的表单实例 |
| `layout` | `horizontal \| vertical \| inline` | 否 | `vertical` | 表单布局 |
| `className` | `string` | 否 | - | 表单根节点类名 |
| `columns` | `number` | 否 | `3` | 每行字段列数 |
| `submitText` | `string` | 否 | `提交` | 提交按钮文案 |
| `resetText` | `string` | 否 | `重置` | 重置按钮文案 |
| `showActions` | `boolean` | 否 | `true` | 是否显示内置操作区 |
| `loading` | `boolean` | 否 | `false` | 提交按钮加载状态 |
| `onFinish` | `(values) => void \| Promise<void>` | 否 | - | 校验通过后触发 |
| `onReset` | `() => void` | 否 | - | 重置字段后触发 |

内置字段配置：

| 属性 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `name` | `FieldName<TValues>` | 是 | - | 提交值路径 |
| `label` | `ReactNode` | 是 | - | 字段标题 |
| `type` | `FormFieldType` | 否 | `input` | 内置控件类型 |
| `placeholder` | `string` | 否 | 自动生成 | 控件占位文案 |
| `options` | `FieldOption[]` | 否 | - | 选项型控件数据源 |
| `rules` | `FormItemProps["rules"]` | 否 | - | Ant Design 校验规则 |
| `colSpan` | `number` | 否 | 按 columns 计算 | 24 栅格宽度 |
| `hidden` | `boolean` | 否 | `false` | 隐藏并停止渲染字段 |
| `disabled` | `boolean` | 否 | `false` | 禁用内置控件 |
| `componentProps` | `Record<string, unknown>` | 否 | - | 透传给实际控件 |
| `formItemProps` | `FormItemProps` | 否 | - | 透传给 `Form.Item` |
| `render` | `(field, form) => ReactNode` | 否 | - | 覆盖内置控件渲染 |

```tsx
<SchemaForm<UserFormValues>
  fields={[
    { name: "name", label: "姓名", rules: [{ required: true }] },
    { name: "status", label: "状态", type: "select", options: statuses },
  ]}
  onFinish={saveUser}
/>
```

### SearchTable

导入：`@/components/composite/search-table`

`SearchTable<TRecord, TQuery>` 组合查询表单、扩展头部、操作区和 Ant Design `Table`。除 `children` 外，其余 `TableProps<TRecord>` 均直接透传。

项目扩展配置：

| 属性 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `search` | `SearchTableSearchConfig<TQuery>` | 否 | 不显示 | 搜索区配置 |
| `tableHeader` | `ReactNode` | 否 | - | 表格上方扩展内容，如 Tabs |
| `actions` | `ReactNode` | 否 | - | 列表右上角操作区 |

`search` 配置：

| 属性 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `fields` | `FormFieldConfig<TQuery>[]` | 是 | - | 搜索字段；空数组不展示搜索区 |
| `form` | `FormInstance<TQuery>` | 否 | 内部实例 | 页面控制的查询表单 |
| `columns` | `number` | 否 | `4` | 每行搜索字段列数 |
| `onSearch` | `(values: TQuery) => void` | 是 | - | 查询提交回调 |
| `onReset` | `() => void` | 是 | - | 查询重置回调 |

```tsx
<SearchTable<Employee, EmployeeQuery>
  rowKey="id"
  columns={columns}
  dataSource={employees}
  loading={loading}
  search={{
    fields: searchFields,
    onSearch: setQuery,
    onReset: resetQuery,
  }}
  actions={<Button type="primary">新增员工</Button>}
/>
```

## 业务组件

- `domain/auth`：登录保护、工作空间路由校验和操作权限展示。
- `domain/logs`：平台系统日志、平台操作日志和租户操作日志的共用查询展示。
- `domain/rbac`：员工、角色、菜单、部门管理及权限树、业务表格配置。

业务组件可以复用基础和组合组件，但基础、组合组件不得反向依赖业务管理组件。
