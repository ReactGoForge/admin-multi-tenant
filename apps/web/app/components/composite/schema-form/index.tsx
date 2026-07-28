import type { FormInstance, FormItemProps } from 'antd'
import type { CSSProperties, ReactNode } from 'react'
import {
  Button,
  Col,
  DatePicker,
  Form,
  Input,
  InputNumber,
  Row,
  Select,
  Switch,
} from 'antd'

/** SchemaForm 字段路径，支持顶层键、字符串路径和嵌套数组路径。 */
export type FieldName<TValues extends object>
  = | Extract<keyof TValues, string>
    | string
    | number
    | Array<string | number>

/** 表单字段支持的内置控件类型，不满足时使用 render 自定义渲染。 */
export type FormFieldType
  = | 'input'
    | 'password'
    | 'select'
    | 'datePicker'
    | 'rangePicker'
    | 'textarea'
    | 'number'
    | 'switch'

/** SchemaForm 选项型控件使用的选项。 */
export interface FieldOption {
  /** 展示给用户看的选项文案，可以传字符串或自定义节点。 */
  label: ReactNode
  /** 实际提交到表单值里的选项值，需要和后端或页面逻辑约定一致。 */
  value: string | number | boolean
  /** 是否禁用当前选项。 */
  disabled?: boolean
}

/** SchemaForm 内置字段配置。 */
export interface FormFieldConfig<TValues extends object> {
  /** 字段名，对应表单提交 values 里的 key；数组形式用于嵌套字段。 */
  name: FieldName<TValues>
  /** 表单项左侧或上方显示的标题文案。 */
  label: ReactNode
  /** 控件类型；不传时默认按 input 渲染。 */
  type?: FormFieldType
  /** 控件占位文案；不传时根据字段类型自动生成“请输入/请选择”。 */
  placeholder?: string
  /** select 等选项型控件的数据源。 */
  options?: FieldOption[]
  /** antd Form.Item 校验规则，控制必填、格式、长度等校验行为。 */
  rules?: FormItemProps<TValues>['rules']
  /** 当前字段在 24 栅格中的宽度；不传时按表单 columns 自动计算。 */
  colSpan?: number
  /** 是否隐藏该字段；隐藏后不渲染对应 Form.Item。 */
  hidden?: boolean
  /** 是否禁用该字段；会传递给内置控件。 */
  disabled?: boolean
  /** 透传给实际 antd 控件的属性，比如 maxLength、mode、showTime。 */
  componentProps?: Record<string, unknown>
  /** 透传给 Form.Item 的属性，比如 extra、tooltip、dependencies。 */
  formItemProps?: Omit<
    FormItemProps<TValues>,
    'children' | 'label' | 'name' | 'rules' | 'valuePropName'
  >
  /** 自定义字段渲染函数；传入后会覆盖内置 type 渲染逻辑。 */
  render?: (
    field: FormFieldConfig<TValues>,
    form: FormInstance<TValues>,
  ) => ReactNode
}

/** SchemaForm 自定义复杂内容配置。 */
export interface FormCustomContentConfig<TValues extends object> {
  /** 自定义内容项的稳定标识，用于列表渲染。 */
  key: string
  /** 当前内容项在 24 栅格中的宽度；不传时按表单 columns 自动计算。 */
  colSpan?: number
  /** 是否隐藏当前内容项。 */
  hidden?: boolean
  /** 直接渲染复杂表单区域，例如权限树 Tabs 或组合字段。 */
  renderItem: (form: FormInstance<TValues>) => ReactNode
}

/** SchemaForm 支持的内置字段或自定义内容配置。 */
export type FormContentConfig<TValues extends object>
  = | FormFieldConfig<TValues>
    | FormCustomContentConfig<TValues>

/** SchemaForm 配置。 */
export interface SchemaFormProps<TValues extends object> {
  /** 表单字段配置列表，决定渲染哪些 Form.Item 和控件。 */
  fields: Array<FormContentConfig<TValues>>
  /** 表单初始值，常用于编辑页回填或默认开关状态。 */
  initialValues?: Partial<TValues>
  /** 外部传入的 antd Form 实例；需要父组件控制表单时传入。 */
  form?: FormInstance<TValues>
  /** antd 表单布局方式，默认 vertical。 */
  layout?: 'horizontal' | 'vertical' | 'inline'
  /** 透传给表单根节点的样式类，用于页面级布局微调。 */
  className?: string
  /** 每行展示几列字段，组件会按 24 栅格自动计算字段宽度，默认 3。 */
  columns?: number
  /** 提交按钮文案，默认“提交”。 */
  submitText?: string
  /** 重置按钮文案，默认“重置”。 */
  resetText?: string
  /** 是否显示内置提交和重置按钮；弹窗页自定义 footer 时可关闭。 */
  showActions?: boolean
  /** 提交按钮 loading 状态，用于异步保存时防止重复提交。 */
  loading?: boolean
  /** 表单校验通过后的提交回调，参数是完整表单值。 */
  onFinish?: (values: TValues) => void | Promise<void>
  /** 点击重置按钮并清空表单后触发，常用于同步外部查询条件。 */
  onReset?: () => void
}

/** isCustomContentConfig 判断配置项是否直接渲染复杂表单内容。 */
function isCustomContentConfig<TValues extends object>(
  field: FormContentConfig<TValues>,
): field is FormCustomContentConfig<TValues> {
  return 'renderItem' in field
}

/** 根据字段类型生成默认占位文案，减少页面配置重复。 */
function getDefaultPlaceholder<TValues extends object>(
  field: FormFieldConfig<TValues>,
) {
  if (field.placeholder) {
    return field.placeholder
  }

  if (field.type === 'select' || field.type === 'datePicker') {
    return `请选择${field.label}`
  }

  return `请输入${field.label}`
}

/** 根据字段配置选择对应的 antd 控件，支持自定义 render 覆盖。 */
function renderField<TValues extends object>(
  field: FormFieldConfig<TValues>,
  form: FormInstance<TValues>,
) {
  if (field.render) {
    return field.render(field, form)
  }

  const commonProps = {
    disabled: field.disabled,
    placeholder: getDefaultPlaceholder(field),
    ...field.componentProps,
  }

  switch (field.type) {
    case 'password':
      return <Input.Password {...commonProps} />
    case 'select':
      return (
        <Select
          allowClear
          options={field.options?.map(option => ({
            label: option.label,
            value: option.value,
            disabled: option.disabled,
          }))}
          {...commonProps}
        />
      )
    case 'datePicker':
      return <DatePicker className="w-full" {...commonProps} />
    case 'rangePicker':
      return (
        <DatePicker.RangePicker
          className="w-full"
          disabled={field.disabled}
          placeholder={['开始日期', '结束日期']}
          {...field.componentProps}
        />
      )
    case 'textarea':
      return (
        <Input.TextArea
          autoSize={{ minRows: 3, maxRows: 6 }}
          {...commonProps}
        />
      )
    case 'number':
      return (
        <InputNumber
          {...commonProps}
          style={{
            width: '100%',
            ...(field.componentProps?.style as CSSProperties | undefined),
          }}
        />
      )
    case 'switch':
      return <Switch disabled={field.disabled} {...field.componentProps} />
    default:
      return <Input allowClear {...commonProps} />
  }
}

/** 通过字段配置渲染通用表单，并内置提交、重置操作区。 */
export function SchemaForm<TValues extends object>({
  fields,
  initialValues,
  form: controlledForm,
  layout = 'vertical',
  className,
  columns = 3,
  submitText = '提交',
  resetText = '重置',
  showActions = true,
  loading = false,
  onFinish,
  onReset,
}: SchemaFormProps<TValues>) {
  const [innerForm] = Form.useForm<TValues>()
  const form = controlledForm ?? innerForm
  const colSpan = Math.floor(24 / columns)

  return (
    <Form<TValues>
      className={className}
      form={form}
      initialValues={initialValues}
      layout={layout}
      onFinish={onFinish}
    >
      <Row gutter={16}>
        {fields
          .filter(field => !field.hidden)
          .map(field => (
            <Col
              key={
                isCustomContentConfig(field)
                  ? field.key
                  : Array.isArray(field.name)
                    ? field.name.join('.')
                    : String(field.name)
              }
              lg={field.colSpan ?? colSpan}
              md={12}
              xs={24}
            >
              {isCustomContentConfig(field)
                ? (
                    field.renderItem(form)
                  )
                : (
                    <Form.Item
                      label={field.label}
                      name={field.name as FormItemProps<TValues>['name']}
                      rules={field.rules}
                      valuePropName={field.type === 'switch' ? 'checked' : 'value'}
                      {...field.formItemProps}
                    >
                      {renderField(field, form)}
                    </Form.Item>
                  )}
            </Col>
          ))}
        {showActions
          ? (
              <Col
                className="flex items-end"
                lg={colSpan}
                md={12}
                style={{ marginLeft: 'auto' }}
                xs={24}
              >
                <Form.Item className="w-full">
                  <div className="flex justify-end gap-2">
                    <Button
                      onClick={() => {
                        form.resetFields()
                        onReset?.()
                      }}
                    >
                      {resetText}
                    </Button>
                    <Button htmlType="submit" loading={loading} type="primary">
                      {submitText}
                    </Button>
                  </div>
                </Form.Item>
              </Col>
            )
          : null}
      </Row>
    </Form>
  )
}
