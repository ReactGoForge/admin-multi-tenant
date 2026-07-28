import type { FormInstance, TableProps } from 'antd'
import type { ReactNode } from 'react'
import type { FormFieldConfig } from '@/components/composite/schema-form'

import { Card, Divider, Table } from 'antd'
import styles from './index.module.scss'
import { SearchForm } from './search-form'

/** SearchTable 可选搜索区配置。 */
export interface SearchTableSearchConfig<TQuery extends object> {
  /** 搜索字段配置；为空时不展示搜索区。 */
  fields: Array<FormFieldConfig<TQuery>>
  /** 可选的外部表单实例，用于页面控制查询表单。 */
  form?: FormInstance<TQuery>
  /** 搜索区每行列数，默认 4。 */
  columns?: number
  /** 查询表单提交回调。 */
  onSearch: (values: TQuery) => void
  /** 查询表单重置回调。 */
  onReset: () => void
}

/**
 * SearchTable 配置。
 * 除 children 外，其余 Ant Design Table 属性全部透传给内部 Table。
 */
export type SearchTableProps<
  TRecord,
  TQuery extends object = Record<string, never>,
> = Omit<TableProps<TRecord>, 'children'> & {
  /** 可选搜索区配置；不传时不展示搜索区。 */
  search?: SearchTableSearchConfig<TQuery>
  /** 表格上方扩展区域，可放 Tabs、提示或其他筛选控件。 */
  tableHeader?: ReactNode
  /** 列表右上角操作区。 */
  actions?: ReactNode
}

/** 统一组合搜索区、表格头部、操作区和 Ant Design Table。 */
export function SearchTable<
  TRecord,
  TQuery extends object = Record<string, never>,
>({
  search,
  tableHeader,
  actions,
  ...tableProps
}: SearchTableProps<TRecord, TQuery>) {
  const showSearch = Boolean(search?.fields.length)

  return (
    <Card className={styles.tableCard} variant="borderless" size="medium">
      {showSearch && search
        ? (
            <>
              <SearchForm<TQuery>
                columns={search.columns}
                fields={search.fields}
                form={search.form}
                onReset={search.onReset}
                onSearch={search.onSearch}
              />
              <Divider size="small" />
            </>
          )
        : null}
      {tableHeader}
      {actions
        ? (
            <div className="mb-4 flex flex-wrap items-center justify-end gap-2">
              {actions}
            </div>
          )
        : null}
      <Table<TRecord> {...tableProps} />
    </Card>
  )
}
