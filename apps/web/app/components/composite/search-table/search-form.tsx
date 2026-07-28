import type { FormInstance } from 'antd'

import type { FormFieldConfig } from '@/components/composite/schema-form'
import {

  SchemaForm,
} from '@/components/composite/schema-form'
import styles from './search-form.module.scss'

/** SearchTable 内部搜索表单配置。 */
interface SearchFormProps<TValues extends object> {
  /** 传给 SchemaForm 的搜索字段配置。 */
  fields: Array<FormFieldConfig<TValues>>
  /** 可选的外部受控 Ant Design 表单实例。 */
  form?: FormInstance<TValues>
  /** 表单校验通过并提交查询时触发。 */
  onSearch: (values: TValues) => void
  /** 用户重置搜索条件时触发。 */
  onReset: () => void
  /** 搜索区域每行字段列数，默认 4。 */
  columns?: number
}

/** SearchForm 为 SearchTable 提供固定查询与重置文案的内部表单。 */
export function SearchForm<TValues extends object>({
  fields,
  form,
  onSearch,
  onReset,
  columns = 4,
}: SearchFormProps<TValues>) {
  return (
    <SchemaForm<TValues>
      className={styles.searchForm}
      columns={columns}
      fields={fields}
      form={form}
      layout="horizontal"
      onFinish={onSearch}
      onReset={onReset}
      resetText="重置"
      submitText="查询"
    />
  )
}
